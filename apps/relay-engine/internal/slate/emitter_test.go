package slate_test

import (
	"bytes"
	"sync"
	"testing"
	"time"

	flvtag "github.com/yutopp/go-flv/tag"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/media"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/slate"
)

type recordingWriter struct {
	mu           sync.Mutex
	video        int
	audio        int
	audioPayload []byte
}

func (w *recordingWriter) WriteVideoPayload(_ uint32, _ []byte) error {
	w.mu.Lock()
	w.video++
	w.mu.Unlock()
	return nil
}

func (w *recordingWriter) WriteAudioPayload(_ uint32, payload []byte) error {
	w.mu.Lock()
	w.audio++
	w.audioPayload = append([]byte(nil), payload...)
	w.mu.Unlock()
	return nil
}

type advancingTimeline struct {
	video uint32
	audio uint32
}

func (t *advancingTimeline) AdvanceVideo(deltaMs uint32) uint32 {
	if deltaMs == 0 {
		deltaMs = 1
	}
	t.video += deltaMs
	return t.video
}

func (t *advancingTimeline) AdvanceAudio(deltaMs uint32) uint32 {
	if deltaMs == 0 {
		deltaMs = 1
	}
	t.audio += deltaMs
	return t.audio
}

func seedEmitter(t *testing.T, emitter *slate.Emitter) {
	t.Helper()

	emitter.ObserveFrame(media.Frame{
		Kind:         media.KindVideo,
		Timestamp:    1000,
		VideoPayload: []byte{0x17, 0x01, 0x00, 0x00, 0x00, 0x09, 0x08},
	})
	emitter.ObserveFrame(media.Frame{
		Kind:         media.KindAudio,
		Timestamp:    1000,
		AudioPayload: []byte{0xaf, 0x01, 0x01, 0x02, 0x03},
	})
}

func TestEmitterBootstrapAndContinuing(t *testing.T) {
	emitter := slate.NewEmitter()
	seedEmitter(t, emitter)
	emitter.SetDisplay("Getting ready to delay the stream — 30 seconds left", 30)

	if !emitter.Ready() {
		t.Fatal("expected emitter to be ready")
	}

	writer := &recordingWriter{}
	timeline := &advancingTimeline{video: 60_000, audio: 60_000}

	emitter.BeginHold()
	if !emitter.EmitBootstrap(writer, timeline) {
		t.Fatal("expected bootstrap emit to succeed")
	}

	time.Sleep(60 * time.Millisecond)
	emitter.EmitContinuing(writer, timeline)

	writer.mu.Lock()
	defer writer.mu.Unlock()
	if writer.video == 0 || writer.audio == 0 {
		t.Fatalf("expected slate output, video=%d audio=%d", writer.video, writer.audio)
	}
}

func TestBootstrapOmitsSequenceHeadersAndKeepsAACPayloadValid(t *testing.T) {
	emitter := slate.NewEmitter()
	emitter.ObserveFrame(media.Frame{
		Kind: media.KindVideo,
		Video: &flvtag.VideoData{
			FrameType:     flvtag.FrameTypeKeyFrame,
			CodecID:       flvtag.CodecIDAVC,
			AVCPacketType: flvtag.AVCPacketTypeSequenceHeader,
			Data:          bytes.NewBuffer([]byte{0x01, 0x02}),
		},
	})
	seedEmitter(t, emitter)
	emitter.SetDisplay("Switching back to live", 3)

	writer := &recordingWriter{}
	timeline := &advancingTimeline{video: 1000, audio: 1000}
	emitter.BeginHold()

	if !emitter.EmitBootstrap(writer, timeline) {
		t.Fatal("expected bootstrap")
	}

	writer.mu.Lock()
	defer writer.mu.Unlock()
	if writer.video != 1 {
		t.Fatalf("expected one video packet (keyframe only), got %d", writer.video)
	}
	if got, want := writer.audioPayload, []byte{0xaf, 0x01, 0x01, 0x02, 0x03}; !bytes.Equal(got, want) {
		t.Fatalf("expected AAC payload to be forwarded unchanged, got %x", got)
	}
}

func TestResetConcurrentWithEmitDoesNotPanic(t *testing.T) {
	emitter := slate.NewEmitter()
	seedEmitter(t, emitter)
	emitter.SetDisplay("Switching back to live", 2)

	writer := &recordingWriter{}
	timeline := &advancingTimeline{}

	emitter.BeginHold()
	_ = emitter.EmitBootstrap(writer, timeline)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			emitter.EmitContinuing(writer, timeline)
			time.Sleep(time.Millisecond)
		}
	}()

	for i := 0; i < 50; i++ {
		emitter.Reset()
		time.Sleep(2 * time.Millisecond)
	}

	<-done
}
