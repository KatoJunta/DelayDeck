package state

import (
	"fmt"
	"time"
)

const (
	mockCountdownTicks = 10
)

const (
	SlateEnableDelayFmt    = "Getting ready to delay the stream — %d seconds left"
	SlateDrainingBufferFmt = "Finishing delayed content — %d seconds left"
	SlateReturningLive     = "Switching back to live"
	ReturnLiveSlateSeconds = 3
	DumpSlateSeconds       = 3
)

func (m *Machine) publishOperatorDisplay(slate string, countdown int) {
	m.mu.Lock()
	m.slateMessage = slate
	m.countdownSeconds = countdown
	m.updatedAt = time.Now().UTC()
	event := ChangeEvent{
		PreviousState: m.state,
		CurrentState:  m.state,
		Trigger:       "operator_display_update",
		Timestamp:     m.updatedAt,
		Status:        m.buildSnapshot(),
	}
	listeners := append([]Listener(nil), m.listeners...)
	m.mu.Unlock()

	for _, listener := range listeners {
		listener(event)
	}
}

func (m *Machine) clearOperatorDisplay() {
	m.mu.Lock()
	m.slateMessage = ""
	m.countdownSeconds = 0
	m.updatedAt = time.Now().UTC()
	m.mu.Unlock()
}

func (m *Machine) snapshotActiveDelaySeconds() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeDelaySeconds
}

func (m *Machine) mockCountdownFill(targetSeconds int) {
	ticks := mockCountdownTicks
	if targetSeconds < ticks {
		ticks = targetSeconds
	}
	if ticks <= 0 {
		ticks = 1
	}

	step := targetSeconds / ticks
	if step < 1 {
		step = 1
	}

	remaining := targetSeconds
	for remaining > 0 {
		m.publishOperatorDisplay(fmt.Sprintf(SlateEnableDelayFmt, remaining), remaining)
		filled := targetSeconds - remaining + step
		if filled > targetSeconds {
			filled = targetSeconds
		}
		m.mockSetPartialBuffer(filled, targetSeconds)
		if remaining <= step {
			remaining = 0
		} else {
			remaining -= step
		}
		m.waitTransitionDelay()
	}

	m.mockFillBuffer(targetSeconds)
	m.publishOperatorDisplay("", 0)
}

func (m *Machine) mockDrainAt1x(totalSeconds int) {
	if totalSeconds <= 0 {
		return
	}

	ticks := mockCountdownTicks
	if totalSeconds < ticks {
		ticks = totalSeconds
	}
	if ticks <= 0 {
		ticks = 1
	}

	step := totalSeconds / ticks
	if step < 1 {
		step = 1
	}

	remaining := totalSeconds
	for remaining > 0 {
		m.publishOperatorDisplay(fmt.Sprintf(SlateDrainingBufferFmt, remaining), remaining)
		m.mockReduceBufferBySeconds(step, totalSeconds)
		if remaining <= step {
			remaining = 0
		} else {
			remaining -= step
		}
		m.waitTransitionDelay()
	}

	m.discardBuffer()
	m.publishOperatorDisplay("", 0)
}

func (m *Machine) mockShowReturnLiveSlate() {
	for remaining := ReturnLiveSlateSeconds; remaining > 0; remaining-- {
		m.publishOperatorDisplay(SlateReturningLive, remaining)
		m.waitTransitionDelay()
	}
	m.publishOperatorDisplay("", 0)
}

func (m *Machine) mockSetPartialBuffer(filledSeconds, targetSeconds int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if targetSeconds <= 0 || m.bufferCapacityBytes <= 0 {
		return
	}

	fillRatio := float64(filledSeconds) / float64(targetSeconds)
	if fillRatio > 1 {
		fillRatio = 1
	}
	m.activeDelaySeconds = filledSeconds
	m.bufferUsagePercent = fillRatio * 100
	m.bufferUsedBytes = int64(float64(m.bufferCapacityBytes) * fillRatio)
	m.updatedAt = time.Now().UTC()
}

func (m *Machine) mockReduceBufferBySeconds(stepSeconds, totalSeconds int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if totalSeconds <= 0 || m.bufferCapacityBytes <= 0 {
		return
	}

	remaining := m.activeDelaySeconds - stepSeconds
	if remaining < 0 {
		remaining = 0
	}
	fillRatio := float64(remaining) / float64(totalSeconds)
	if fillRatio < 0 {
		fillRatio = 0
	}
	m.activeDelaySeconds = remaining
	m.bufferUsagePercent = fillRatio * 100
	m.bufferUsedBytes = int64(float64(m.bufferCapacityBytes) * fillRatio)
	m.updatedAt = time.Now().UTC()
}
