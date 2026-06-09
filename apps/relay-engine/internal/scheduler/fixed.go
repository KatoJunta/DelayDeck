package scheduler

import (
	"sync"
	"time"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/buffer"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/media"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/state"
)

const drainTickInterval = 5 * time.Millisecond

type MetricsSink interface {
	UpdateBufferMetrics(usedBytes int64, usagePercent float64, activeDelaySeconds int)
}

type Fixed struct {
	mu                 sync.Mutex
	ring               *buffer.Ring
	writer             FrameWriter
	metrics            MetricsSink
	delay              time.Duration
	targetDelaySeconds int
	stopCh             chan struct{}
	doneCh             chan struct{}
	started            bool
}

func NewFixed(
	ring *buffer.Ring,
	writer FrameWriter,
	metrics MetricsSink,
	targetDelaySeconds int,
) *Fixed {
	return &Fixed{
		ring:               ring,
		writer:             writer,
		metrics:            metrics,
		delay:              time.Duration(targetDelaySeconds) * time.Second,
		targetDelaySeconds: targetDelaySeconds,
		stopCh:             make(chan struct{}),
		doneCh:             make(chan struct{}),
	}
}

func (s *Fixed) Start() {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.mu.Unlock()

	go s.run()
}

func (s *Fixed) Stop() {
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

func (s *Fixed) Push(frame media.Frame) error {
	if frame.EnqueuedAt.IsZero() {
		frame.EnqueuedAt = time.Now().UTC()
	}
	if err := s.ring.Push(frame); err != nil {
		return err
	}
	s.publishMetrics()
	return nil
}

func (s *Fixed) Clear() {
	s.ring.Clear()
	s.publishMetrics()
}

func (s *Fixed) run() {
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

func (s *Fixed) drainReady(now time.Time) {
	for {
		frame, ok := s.ring.PeekOldest()
		if !ok {
			break
		}
		if now.Sub(frame.EnqueuedAt) < s.delay {
			break
		}

		frame, ok = s.ring.PopOldest()
		if !ok {
			break
		}
		if err := WriteFrame(s.writer, frame); err != nil {
			if sink, ok := s.metrics.(*state.Machine); ok {
				_ = sink.MarkError("output write failed")
			}
			return
		}
	}
	s.publishMetrics()
}

func (s *Fixed) publishMetrics() {
	if s.metrics == nil {
		return
	}
	now := time.Now().UTC()
	s.metrics.UpdateBufferMetrics(
		s.ring.UsedBytes(),
		s.ring.UsagePercent(),
		s.ring.ActiveDelaySeconds(now, s.targetDelaySeconds),
	)
}
