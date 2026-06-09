package scheduler_test

import (
	"sync"
	"testing"
	"time"

	flvtag "github.com/yutopp/go-flv/tag"
	rtmpmsg "github.com/yutopp/go-rtmp/message"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/buffer"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/media"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/scheduler"
)

type recordingWriter struct {
	mu      sync.Mutex
	frames  []media.Kind
	metadata []byte
}

func (w *recordingWriter) WriteSetDataFrame(_ uint32, _ *rtmpmsg.NetStreamSetDataFrame) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.frames = append(w.frames, media.KindSetDataFrame)
	return nil
}

func (w *recordingWriter) WriteOnMetaData(_ uint32, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.frames = append(w.frames, media.KindOnMetaData)
	w.metadata = append([]byte(nil), payload...)
	return nil
}

func (w *recordingWriter) WriteAudioData(_ uint32, _ *flvtag.AudioData) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.frames = append(w.frames, media.KindAudio)
	return nil
}

func (w *recordingWriter) WriteVideoData(_ uint32, _ *flvtag.VideoData) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.frames = append(w.frames, media.KindVideo)
	return nil
}

type metricsRecorder struct {
	mu                 sync.Mutex
	usedBytes          int64
	usagePercent       float64
	activeDelaySeconds int
}

func (m *metricsRecorder) UpdateBufferMetrics(usedBytes int64, usagePercent float64, activeDelaySeconds int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.usedBytes = usedBytes
	m.usagePercent = usagePercent
	m.activeDelaySeconds = activeDelaySeconds
}

func TestFixedDelayReleasesFrameAfterDelay(t *testing.T) {
	ring := buffer.NewRing(4096)
	writer := &recordingWriter{}
	metrics := &metricsRecorder{}

	sched := scheduler.NewFixed(ring, writer, metrics, 1)
	sched.Start()
	defer sched.Stop()

	enqueuedAt := time.Now().UTC().Add(-2 * time.Second)
	if err := sched.Push(media.Frame{
		Kind:       media.KindOnMetaData,
		MetaData:   []byte("delayed"),
		EnqueuedAt: enqueuedAt,
	}); err != nil {
		t.Fatalf("push: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		writer.mu.Lock()
		count := len(writer.frames)
		writer.mu.Unlock()
		if count == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	writer.mu.Lock()
	defer writer.mu.Unlock()
	if len(writer.frames) != 1 {
		t.Fatalf("expected 1 released frame, got %d", len(writer.frames))
	}
	if string(writer.metadata) != "delayed" {
		t.Fatalf("metadata = %q", writer.metadata)
	}
}

func TestFixedDelayHoldsFrameUntilDelayElapsed(t *testing.T) {
	ring := buffer.NewRing(4096)
	writer := &recordingWriter{}
	metrics := &metricsRecorder{}

	sched := scheduler.NewFixed(ring, writer, metrics, 30)
	sched.Start()
	defer sched.Stop()

	if err := sched.Push(media.Frame{
		Kind:     media.KindOnMetaData,
		MetaData: []byte("hold"),
	}); err != nil {
		t.Fatalf("push: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	writer.mu.Lock()
	count := len(writer.frames)
	writer.mu.Unlock()
	if count != 0 {
		t.Fatalf("expected no released frames yet, got %d", count)
	}

	metrics.mu.Lock()
	active := metrics.activeDelaySeconds
	metrics.mu.Unlock()
	if active < 0 {
		t.Fatalf("active delay = %d", active)
	}
}
