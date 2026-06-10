package scheduler

import (
	"sync"
	"time"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/buffer"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/media"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/slate"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/state"
)

const drainTickInterval = 5 * time.Millisecond

func countdownSecondsLeft(remaining time.Duration) int {
	if remaining <= 0 {
		return 0
	}
	return int((remaining + time.Second - time.Nanosecond) / time.Second)
}

type MetricsSink interface {
	UpdateBufferMetrics(usedBytes int64, usagePercent float64, activeDelaySeconds int)
}

type OutputPolicy int

const (
	OutputPassthrough OutputPolicy = iota
	OutputBufferingFill
	OutputSlate
	OutputDelayed
	OutputDrainAtLive
)

type Controller struct {
	mu                 sync.Mutex
	ring               *buffer.Ring
	writer             FrameWriter
	metrics            MetricsSink
	slate              *slate.Emitter
	timeline           OutputTimeline
	policy             OutputPolicy
	delay              time.Duration
	targetDelaySeconds int
	lastLiveDrainAt    time.Time
	slateBuffersInput  bool
	stopCh             chan struct{}
	doneCh             chan struct{}
	started            bool
}

func NewController(
	ring *buffer.Ring,
	writer FrameWriter,
	metrics MetricsSink,
) *Controller {
	return &Controller{
		ring:    ring,
		writer:  writer,
		metrics: metrics,
		slate:   slate.NewEmitter(),
		policy:  OutputPassthrough,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
}

func NewFixed(
	ring *buffer.Ring,
	writer FrameWriter,
	metrics MetricsSink,
	targetDelaySeconds int,
) *Controller {
	c := NewController(ring, writer, metrics)
	c.SetTargetDelay(targetDelaySeconds)
	c.SetPolicy(OutputDelayed)
	return c
}

func (s *Controller) SetPolicy(policy OutputPolicy) {
	s.mu.Lock()
	s.policy = policy
	s.mu.Unlock()
}

func (s *Controller) Policy() OutputPolicy {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.policy
}

func (s *Controller) SetTargetDelay(seconds int) {
	s.mu.Lock()
	s.targetDelaySeconds = seconds
	s.delay = time.Duration(seconds) * time.Second
	s.mu.Unlock()
}

func (s *Controller) TargetDelaySeconds() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.targetDelaySeconds
}

func (s *Controller) ActiveDelaySeconds() int {
	now := time.Now().UTC()
	return s.ring.ActiveDelaySeconds(now, s.TargetDelaySeconds())
}

func (s *Controller) ActiveDelayDuration() time.Duration {
	now := time.Now().UTC()
	return s.ring.ActiveDelayDuration(now, s.TargetDelaySeconds())
}

func (s *Controller) ActiveKeyframeDelaySeconds() int {
	now := time.Now().UTC()
	return s.ring.ActiveKeyframeDelaySeconds(now, s.TargetDelaySeconds())
}

func (s *Controller) ActiveKeyframeDelayDuration() time.Duration {
	now := time.Now().UTC()
	return s.ring.ActiveKeyframeDelayDuration(now, s.TargetDelaySeconds())
}

func (s *Controller) BufferSpanDuration() time.Duration {
	return s.ring.BufferSpanDuration()
}

func (s *Controller) RingLen() int {
	return s.ring.Len()
}

func (s *Controller) SetSlateDisplay(message string, countdown int) {
	s.slate.SetDisplay(message, countdown)
}

func (s *Controller) BeginBufferingFill(targetSeconds int) {
	s.SetTargetDelay(targetSeconds)
	s.ring.Clear()
	s.SetPolicy(OutputBufferingFill)
	s.publishMetrics()
}

func (s *Controller) BeginSlateHold(bufferInput bool) {
	s.slateBuffersInput = bufferInput
	s.slate.BeginHold()
	s.SetPolicy(OutputSlate)
	s.publishMetrics()
}

func (s *Controller) BeginDelayedOutput() {
	s.slateBuffersInput = false
	s.ring.DropUntilVideoKeyframe()
	s.SetPolicy(OutputDelayed)
	s.publishMetrics()
}

func (s *Controller) BeginDrainAtLive() {
	s.lastLiveDrainAt = time.Time{}
	s.SetPolicy(OutputDrainAtLive)
	s.publishMetrics()
}

func (s *Controller) BeginPassthrough() {
	s.slateBuffersInput = false
	s.SetTargetDelay(0)
	s.SetPolicy(OutputPassthrough)
	s.ring.Clear()
	s.slate.Reset()
	s.publishMetrics()
}

func (s *Controller) Start() {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()

	go s.run()
}

func (s *Controller) Stop() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	s.started = false
	s.mu.Unlock()

	close(s.stopCh)
	<-s.doneCh
}

func (s *Controller) Push(frame media.Frame) error {
	if frame.EnqueuedAt.IsZero() {
		frame.EnqueuedAt = time.Now().UTC()
	}

	s.mu.Lock()
	policy := s.policy
	s.mu.Unlock()

	s.observeForSlate(frame)

	switch policy {
	case OutputPassthrough:
		return s.writeOutDirect(frame)
	case OutputBufferingFill:
		if err := s.ring.Push(frame); err != nil {
			return err
		}
		s.publishMetrics()
		return s.writeOutDirect(frame)
	case OutputSlate:
		s.mu.Lock()
		bufferInput := s.slateBuffersInput
		s.mu.Unlock()
		if bufferInput {
			if err := s.ring.Push(frame); err != nil {
				return err
			}
			s.publishMetrics()
		}
		return s.writeOutDirect(frame)
	case OutputDrainAtLive:
		if s.ring.Len() == 0 {
			return s.writeOutDirect(frame)
		}
		return nil
	default:
		if err := s.ring.Push(frame); err != nil {
			return err
		}
		s.publishMetrics()
		return nil
	}
}

func (s *Controller) Clear() {
	s.ring.Clear()
	s.publishMetrics()
}

func (s *Controller) writeOutDirect(frame media.Frame) error {
	switch frame.Kind {
	case media.KindVideo:
		if len(frame.VideoPayload) > 0 {
			ts := s.timeline.Video(frame.Timestamp)
			return s.writer.WriteVideoPayload(ts, frame.VideoPayload)
		}
		if frame.Video == nil {
			return nil
		}
		frame.Timestamp = s.timeline.Video(frame.Timestamp)
		return s.writer.WriteVideoData(frame.Timestamp, frame.Video)
	case media.KindAudio:
		if len(frame.AudioPayload) > 0 {
			ts := s.timeline.Audio(frame.Timestamp)
			return s.writer.WriteAudioPayload(ts, frame.AudioPayload)
		}
		if frame.Audio == nil {
			return nil
		}
		frame.Timestamp = s.timeline.Audio(frame.Timestamp)
		return s.writer.WriteAudioData(frame.Timestamp, frame.Audio)
	case media.KindSetDataFrame:
		if frame.SetData == nil {
			return nil
		}
		return s.writer.WriteSetDataFrame(frame.Timestamp, frame.SetData)
	case media.KindOnMetaData:
		return s.writer.WriteOnMetaData(frame.Timestamp, frame.MetaData)
	default:
		return nil
	}
}

func (s *Controller) writeOut(frame media.Frame) error {
	switch frame.Kind {
	case media.KindVideo:
		if len(frame.VideoPayload) > 0 {
			ts := s.timeline.Video(frame.Timestamp)
			return s.writer.WriteVideoPayload(ts, frame.VideoPayload)
		}
		if frame.Video == nil {
			return nil
		}
		frame.Timestamp = s.timeline.Video(frame.Timestamp)
		return s.writer.WriteVideoData(frame.Timestamp, frame.Video)
	case media.KindAudio:
		if len(frame.AudioPayload) > 0 {
			ts := s.timeline.Audio(frame.Timestamp)
			return s.writer.WriteAudioPayload(ts, frame.AudioPayload)
		}
		if frame.Audio == nil {
			return nil
		}
		frame.Timestamp = s.timeline.Audio(frame.Timestamp)
		return s.writer.WriteAudioData(frame.Timestamp, frame.Audio)
	case media.KindSetDataFrame:
		if frame.SetData == nil {
			return nil
		}
		return s.writer.WriteSetDataFrame(frame.Timestamp, frame.SetData)
	case media.KindOnMetaData:
		return s.writer.WriteOnMetaData(frame.Timestamp, frame.MetaData)
	default:
		return nil
	}
}

func (s *Controller) observeForSlate(frame media.Frame) {
	s.slate.ObserveFrame(frame)
}

func (s *Controller) run() {
	defer close(s.doneCh)

	ticker := time.NewTicker(drainTickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			s.drainReady(now.UTC())
		}
	}
}

func (s *Controller) drainReady(now time.Time) {
	s.mu.Lock()
	policy := s.policy
	delay := s.delay
	s.mu.Unlock()

	switch policy {
	case OutputDelayed:
		s.drainDelayed(now, delay)
	case OutputDrainAtLive:
		s.drainAtLive(now)
	case OutputPassthrough:
	}

	s.publishMetrics()
}

func (s *Controller) drainDelayed(now time.Time, delay time.Duration) {
	for {
		frame, ok := s.ring.PeekOldest()
		if !ok {
			break
		}
		if now.Sub(frame.EnqueuedAt) < delay {
			break
		}

		frame, ok = s.ring.PopOldest()
		if !ok {
			break
		}
		if err := s.writeOut(frame); err != nil {
			if sink, ok := s.metrics.(*state.Machine); ok {
				_ = sink.MarkError("output write failed")
			}
			return
		}
	}
}

func (s *Controller) drainAtLive(now time.Time) {
	if !s.lastLiveDrainAt.IsZero() && now.Sub(s.lastLiveDrainAt) < 10*time.Millisecond {
		return
	}

	frame, ok := s.ring.PopOldest()
	if !ok {
		return
	}
	s.lastLiveDrainAt = now
	if err := s.writeOut(frame); err != nil {
		if sink, ok := s.metrics.(*state.Machine); ok {
			_ = sink.MarkError("output write failed")
		}
	}
}

func (s *Controller) publishMetrics() {
	if s.metrics == nil {
		return
	}
	now := time.Now().UTC()
	targetSeconds := s.TargetDelaySeconds()
	activeSeconds := s.ring.ActiveDelaySeconds(now, targetSeconds)

	s.mu.Lock()
	policy := s.policy
	s.mu.Unlock()
	if policy == OutputDrainAtLive {
		activeSeconds = countdownSecondsLeft(s.ring.BufferSpanDuration())
	}

	s.metrics.UpdateBufferMetrics(
		s.ring.UsedBytes(),
		s.ring.UsagePercent(),
		activeSeconds,
	)
}
