package scheduler_test

import (
	"testing"
	"time"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/buffer"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/media"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/scheduler"
)

func TestPassthroughWritesImmediately(t *testing.T) {
	ring := buffer.NewRing(4096)
	writer := &recordingWriter{}

	sched := scheduler.NewController(ring, writer, nil)
	sched.Start()
	defer sched.Stop()

	if err := sched.Push(media.Frame{
		Kind:     media.KindOnMetaData,
		MetaData: []byte("live"),
	}); err != nil {
		t.Fatalf("push: %v", err)
	}

	writer.mu.Lock()
	count := len(writer.frames)
	writer.mu.Unlock()
	if count != 1 {
		t.Fatalf("expected immediate write, got %d frames", count)
	}
	if ring.Len() != 0 {
		t.Fatalf("expected empty ring in passthrough, got %d", ring.Len())
	}
}

func TestBufferingFillPassesThroughWhileBuffering(t *testing.T) {
	ring := buffer.NewRing(4096)
	writer := &recordingWriter{}

	sched := scheduler.NewController(ring, writer, nil)
	sched.BeginBufferingFill(30)
	sched.Start()
	defer sched.Stop()

	if err := sched.Push(media.Frame{
		Kind:     media.KindOnMetaData,
		MetaData: []byte("buffered"),
	}); err != nil {
		t.Fatalf("push: %v", err)
	}

	writer.mu.Lock()
	count := len(writer.frames)
	writer.mu.Unlock()
	if count != 1 {
		t.Fatalf("expected passthrough before slate is ready, got %d", count)
	}
	if ring.Len() != 1 {
		t.Fatalf("expected one buffered frame, got %d", ring.Len())
	}
}

func TestSlatePassesThroughAndBuffersWhenRequested(t *testing.T) {
	ring := buffer.NewRing(4096)
	writer := &recordingWriter{}

	sched := scheduler.NewController(ring, writer, nil)
	sched.SetTargetDelay(30)
	sched.Start()
	defer sched.Stop()

	sched.BeginSlateHold(true)

	if err := sched.Push(media.Frame{
		Kind:         media.KindVideo,
		Timestamp:    1000,
		VideoPayload: []byte{0x17, 0x01, 0x00, 0x00, 0x00, 0x09, 0x08},
	}); err != nil {
		t.Fatalf("push video: %v", err)
	}

	writer.mu.Lock()
	writes := len(writer.frames)
	writer.mu.Unlock()
	if writes != 1 {
		t.Fatalf("expected slate input passthrough, got %d writes", writes)
	}
	if ring.Len() != 1 {
		t.Fatalf("expected slate input buffered, got %d frames", ring.Len())
	}
}

func TestSlatePassesThroughWithoutBufferWhenNotRequested(t *testing.T) {
	ring := buffer.NewRing(4096)
	writer := &recordingWriter{}

	sched := scheduler.NewController(ring, writer, nil)
	sched.SetTargetDelay(30)
	sched.Start()
	defer sched.Stop()

	sched.BeginSlateHold(false)

	if err := sched.Push(media.Frame{
		Kind:         media.KindVideo,
		Timestamp:    1000,
		VideoPayload: []byte{0x17, 0x01, 0x00, 0x00, 0x00, 0x09, 0x08},
	}); err != nil {
		t.Fatalf("push video: %v", err)
	}

	writer.mu.Lock()
	defer writer.mu.Unlock()
	if len(writer.frames) != 1 {
		t.Fatalf("expected slate input passthrough, got %d writes", len(writer.frames))
	}
	if ring.Len() != 0 {
		t.Fatalf("expected no buffering during timed slate, got %d frames", ring.Len())
	}
}

func TestEnableDelayFillWaitsForBufferAge(t *testing.T) {
	ring := buffer.NewRing(4096)
	writer := &recordingWriter{}
	metrics := &metricsRecorder{}

	sched := scheduler.NewController(ring, writer, metrics)
	sched.BeginBufferingFill(2)
	sched.Start()
	defer sched.Stop()

	done := make(chan struct{})
	go func() {
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		for {
			if sched.ActiveDelaySeconds() >= 2 {
				close(done)
				return
			}
			<-tick.C
		}
	}()

	time.Sleep(50 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("buffer reported full too early")
	default:
	}

	enqueuedAt := time.Now().UTC().Add(-3 * time.Second)
	if err := sched.Push(media.Frame{
		Kind:       media.KindOnMetaData,
		MetaData:   []byte("x"),
		EnqueuedAt: enqueuedAt,
	}); err != nil {
		t.Fatalf("push: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for active delay to reach target")
	}
}

func TestActiveKeyframeDelayWaitsForFirstKeyframe(t *testing.T) {
	ring := buffer.NewRing(4096)
	writer := &recordingWriter{}

	sched := scheduler.NewController(ring, writer, nil)
	sched.BeginBufferingFill(5)

	now := time.Now().UTC()
	if err := sched.Push(media.Frame{
		Kind:       media.KindAudio,
		EnqueuedAt: now.Add(-5 * time.Second),
		AudioPayload: []byte{
			0xaf, 0x01, 0x01,
		},
	}); err != nil {
		t.Fatalf("push audio: %v", err)
	}
	if err := sched.Push(media.Frame{
		Kind:         media.KindVideo,
		EnqueuedAt:   now.Add(-2 * time.Second),
		VideoPayload: []byte{0x17, 0x01, 0x00, 0x00, 0x00, 0x09},
	}); err != nil {
		t.Fatalf("push keyframe: %v", err)
	}

	if got := sched.ActiveDelaySeconds(); got != 5 {
		t.Fatalf("active delay = %d, want 5", got)
	}
	if got := sched.ActiveKeyframeDelaySeconds(); got != 2 {
		t.Fatalf("keyframe delay = %d, want 2", got)
	}
}

func TestDrainAtLiveEmptiesRing(t *testing.T) {
	ring := buffer.NewRing(4096)
	writer := &recordingWriter{}

	sched := scheduler.NewController(ring, writer, nil)
	sched.SetTargetDelay(30)
	sched.SetPolicy(scheduler.OutputDelayed)
	sched.Start()
	defer sched.Stop()

	for i := 0; i < 5; i++ {
		if err := sched.Push(media.Frame{
			Kind:     media.KindOnMetaData,
			MetaData: []byte("x"),
		}); err != nil {
			t.Fatalf("push: %v", err)
		}
	}
	if sched.RingLen() != 5 {
		t.Fatalf("expected 5 buffered frames, got %d", sched.RingLen())
	}

	sched.BeginDrainAtLive()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if sched.RingLen() == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("ring still has %d frames after drain", sched.RingLen())
}
