package state

import (
	"sync"
	"time"
)

type ConnectionState string

const (
	ConnectionDisconnected ConnectionState = "disconnected"
	ConnectionConnected    ConnectionState = "connected"
)

type StatusSnapshot struct {
	State              State           `json:"state"`
	TargetDelaySeconds int             `json:"target_delay_seconds"`
	ActiveDelaySeconds int             `json:"active_delay_seconds"`
	BufferUsagePercent float64         `json:"buffer_usage_percent"`
	BufferUsedBytes    int64           `json:"buffer_used_bytes"`
	BufferCapacityBytes int64          `json:"buffer_capacity_bytes"`
	InputConnected     bool            `json:"input_connected"`
	OutputConnected    bool            `json:"output_connected"`
	InputState         ConnectionState `json:"input_state"`
	OutputState        ConnectionState `json:"output_state"`
	TransitionPending  bool            `json:"transition_pending"`
	SlateMessage       string          `json:"slate_message"`
	CountdownSeconds   int             `json:"countdown_seconds"`
	LastError          string          `json:"last_error"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type ChangeEvent struct {
	PreviousState State           `json:"previous_state"`
	CurrentState  State           `json:"current_state"`
	Trigger       string          `json:"trigger"`
	Timestamp     time.Time       `json:"timestamp"`
	Status        StatusSnapshot  `json:"status"`
}

type Listener func(ChangeEvent)

type Machine struct {
	mu sync.RWMutex

	state               State
	targetDelaySeconds  int
	activeDelaySeconds  int
	bufferUsagePercent  float64
	bufferUsedBytes     int64
	bufferCapacityBytes int64
	inputConnected      bool
	outputConnected     bool
	transitionPending   bool
	slateMessage        string
	countdownSeconds    int
	lastError           string
	updatedAt           time.Time

	transitionDelay time.Duration
	listeners       []Listener
}

func NewMachine(bufferCapacityBytes int64, transitionDelay time.Duration) *Machine {
	now := time.Now().UTC()
	return &Machine{
		state:               Stopped,
		bufferCapacityBytes: bufferCapacityBytes,
		transitionDelay:     transitionDelay,
		updatedAt:           now,
	}
}

func (m *Machine) OnChange(listener Listener) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listeners = append(m.listeners, listener)
}

func (m *Machine) Snapshot() StatusSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.buildSnapshot()
}

func (m *Machine) CurrentState() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

func (m *Machine) Start() error {
	return m.apply(Starting, "process_start")
}

func (m *Machine) MarkReady() error {
	return m.apply(Ready, "api_ready")
}

func (m *Machine) ConnectInput() error {
	return m.apply(Ingesting, "input_connected")
}

func (m *Machine) ConnectOutput() error {
	return m.apply(Realtime, "output_connected")
}

func (m *Machine) DisconnectSession() error {
	switch m.CurrentState() {
	case Ready:
		return nil
	case Ingesting, Realtime:
		return m.apply(Ready, "session_stopped")
	default:
		return &TransitionError{From: m.CurrentState(), Trigger: "session_stopped"}
	}
}

func (m *Machine) MockConnectInput() error {
	return m.ConnectInput()
}

func (m *Machine) MockConnectOutput() error {
	return m.ConnectOutput()
}

func (m *Machine) SetFixedDelayTarget(seconds int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.targetDelaySeconds = seconds
	m.updatedAt = time.Now().UTC()
}

func (m *Machine) UpdateBufferMetrics(usedBytes int64, usagePercent float64, activeDelaySeconds int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.bufferUsedBytes = usedBytes
	m.bufferUsagePercent = usagePercent
	m.activeDelaySeconds = activeDelaySeconds
	m.updatedAt = time.Now().UTC()
}

func (m *Machine) MarkError(message string) error {
	m.mu.Lock()
	m.lastError = message
	m.mu.Unlock()
	return m.apply(Error, "fatal_error")
}

func (m *Machine) EnableDelay(targetSeconds int) error {
	if targetSeconds <= 0 {
		return &TransitionError{From: m.CurrentState(), Trigger: "enable_delay_invalid_target"}
	}

	m.mu.Lock()
	if m.transitionPending {
		m.mu.Unlock()
		return ErrTransitionPending
	}
	from := m.state
	if from != Realtime {
		m.mu.Unlock()
		return &TransitionError{From: from, Trigger: "enable_delay"}
	}
	m.targetDelaySeconds = targetSeconds
	m.activeDelaySeconds = 0
	m.bufferUsagePercent = 0
	m.bufferUsedBytes = 0
	m.transitionPending = true
	m.mu.Unlock()

	if err := m.apply(BufferingToDelay, "enable_delay"); err != nil {
		m.clearTransitionPending()
		return err
	}

	go m.runEnableDelaySequence(targetSeconds)
	return nil
}

func (m *Machine) ReturnLive() error {
	m.mu.Lock()
	if m.transitionPending {
		m.mu.Unlock()
		return ErrTransitionPending
	}
	from := m.state
	if from != Delayed {
		m.mu.Unlock()
		return &TransitionError{From: from, Trigger: "return_live"}
	}
	m.transitionPending = true
	m.mu.Unlock()

	if err := m.apply(ReturningToRealtime, "return_live"); err != nil {
		m.clearTransitionPending()
		return err
	}

	go m.runReturnLiveSequence()
	return nil
}

func (m *Machine) DumpBuffer() error {
	m.mu.Lock()
	if m.transitionPending {
		m.mu.Unlock()
		return ErrTransitionPending
	}
	from := m.state
	if from != Delayed {
		m.mu.Unlock()
		return &TransitionError{From: from, Trigger: "dump_buffer"}
	}
	m.transitionPending = true
	m.mu.Unlock()

	if err := m.apply(Dumping, "dump_buffer"); err != nil {
		m.clearTransitionPending()
		return err
	}

	go m.runDumpBufferSequence()
	return nil
}

func (m *Machine) SafeHold() error {
	m.mu.Lock()
	from := m.state
	if m.transitionPending {
		m.mu.Unlock()
		return ErrTransitionPending
	}
	switch from {
	case Realtime, Delayed:
	default:
		m.mu.Unlock()
		return &TransitionError{From: from, Trigger: "safe_hold"}
	}
	m.mu.Unlock()
	return m.apply(SafeHold, "safe_hold")
}

func (m *Machine) ResumeLive() error {
	return m.apply(Realtime, "resume_live")
}

func (m *Machine) ResumeDelayed() error {
	return m.apply(Delayed, "resume_delayed")
}

func (m *Machine) apply(next State, trigger string) error {
	m.mu.Lock()

	from := m.state
	if !canTransition(from, next, trigger) {
		m.mu.Unlock()
		return &TransitionError{From: from, To: next, Trigger: trigger}
	}

	m.state = next
	m.updatedAt = time.Now().UTC()
	m.syncConnectionFlagsLocked()

	event := ChangeEvent{
		PreviousState: from,
		CurrentState:  next,
		Trigger:       trigger,
		Timestamp:     m.updatedAt,
		Status:        m.buildSnapshot(),
	}

	listeners := append([]Listener(nil), m.listeners...)
	m.mu.Unlock()

	for _, listener := range listeners {
		listener(event)
	}
	return nil
}

func canTransition(from, to State, trigger string) bool {
	switch trigger {
	case "process_start":
		return from == Stopped && to == Starting
	case "api_ready":
		return from == Starting && to == Ready
	case "input_connected":
		return from == Ready && to == Ingesting
	case "output_connected":
		return from == Ingesting && to == Realtime
	case "session_stopped":
		return (from == Ingesting || from == Realtime) && to == Ready
	case "enable_delay":
		return from == Realtime && to == BufferingToDelay
	case "buffering_show_safe_slate":
		return from == BufferingToDelay && to == SafeHold
	case "buffer_filled":
		return from == SafeHold && to == Delayed
	case "return_live":
		return from == Delayed && to == ReturningToRealtime
	case "returning_show_safe_slate":
		return from == ReturningToRealtime && to == SafeHold
	case "return_live_complete":
		return from == SafeHold && to == Realtime
	case "dump_buffer":
		return from == Delayed && to == Dumping
	case "dump_complete":
		return from == Dumping && to == SafeHold
	case "dump_resume_live":
		return from == SafeHold && to == Realtime
	case "safe_hold":
		return (from == Realtime || from == Delayed) && to == SafeHold
	case "resume_live":
		return from == SafeHold && to == Realtime
	case "resume_delayed":
		return from == SafeHold && to == Delayed
	case "fatal_error":
		return to == Error
	default:
		return false
	}
}

func (m *Machine) runEnableDelaySequence(targetSeconds int) {
	defer m.clearTransitionPending()
	defer m.clearOperatorDisplay()

	m.waitTransitionDelay()
	if err := m.apply(SafeHold, "buffering_show_safe_slate"); err != nil {
		return
	}

	m.mockCountdownFill(targetSeconds)

	m.waitTransitionDelay()
	_ = m.apply(Delayed, "buffer_filled")
}

func (m *Machine) runReturnLiveSequence() {
	defer m.clearTransitionPending()
	defer m.clearOperatorDisplay()

	drainSeconds := m.snapshotActiveDelaySeconds()
	m.mockDrainAt1x(drainSeconds)

	m.waitTransitionDelay()
	if err := m.apply(SafeHold, "returning_show_safe_slate"); err != nil {
		return
	}

	m.mockShowReturnLiveSlate()

	_ = m.apply(Realtime, "return_live_complete")
}

func (m *Machine) runDumpBufferSequence() {
	defer m.clearTransitionPending()
	defer m.clearOperatorDisplay()

	m.discardBuffer()

	m.waitTransitionDelay()
	if err := m.apply(SafeHold, "dump_complete"); err != nil {
		return
	}

	m.waitTransitionDelay()
	_ = m.apply(Realtime, "dump_resume_live")
}

func (m *Machine) mockFillBuffer(targetSeconds int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.activeDelaySeconds = targetSeconds
	if m.bufferCapacityBytes > 0 {
		fillRatio := float64(targetSeconds) / 120.0
		if fillRatio > 1 {
			fillRatio = 1
		}
		m.bufferUsagePercent = fillRatio * 100
		m.bufferUsedBytes = int64(float64(m.bufferCapacityBytes) * fillRatio)
	}
	m.updatedAt = time.Now().UTC()
}

func (m *Machine) discardBuffer() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.activeDelaySeconds = 0
	m.bufferUsagePercent = 0
	m.bufferUsedBytes = 0
	m.updatedAt = time.Now().UTC()
}

func (m *Machine) clearTransitionPending() {
	m.mu.Lock()
	m.transitionPending = false
	m.updatedAt = time.Now().UTC()
	m.mu.Unlock()
}

func (m *Machine) waitTransitionDelay() {
	if m.transitionDelay <= 0 {
		return
	}
	time.Sleep(m.transitionDelay)
}

func (m *Machine) syncConnectionFlagsLocked() {
	switch m.state {
	case Ingesting:
		m.inputConnected = true
		m.outputConnected = false
	case Realtime, BufferingToDelay, Delayed, ReturningToRealtime, Dumping:
		m.inputConnected = true
		m.outputConnected = true
	case Ready, Starting, Stopped, Error:
		m.inputConnected = false
		m.outputConnected = false
	case SafeHold:
		// keep last known connection flags
	}
}

func (m *Machine) buildSnapshot() StatusSnapshot {
	inputState := ConnectionDisconnected
	if m.inputConnected {
		inputState = ConnectionConnected
	}
	outputState := ConnectionDisconnected
	if m.outputConnected {
		outputState = ConnectionConnected
	}

	return StatusSnapshot{
		State:               m.state,
		TargetDelaySeconds:  m.targetDelaySeconds,
		ActiveDelaySeconds:  m.activeDelaySeconds,
		BufferUsagePercent:  m.bufferUsagePercent,
		BufferUsedBytes:     m.bufferUsedBytes,
		BufferCapacityBytes: m.bufferCapacityBytes,
		InputConnected:      m.inputConnected,
		OutputConnected:     m.outputConnected,
		InputState:          inputState,
		OutputState:         outputState,
		TransitionPending:   m.transitionPending,
		SlateMessage:        m.slateMessage,
		CountdownSeconds:    m.countdownSeconds,
		LastError:           m.lastError,
		UpdatedAt:           m.updatedAt,
	}
}
