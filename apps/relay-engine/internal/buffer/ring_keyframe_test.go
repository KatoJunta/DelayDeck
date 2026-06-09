package buffer_test

import (
	"bytes"
	"testing"
	"time"

	flvtag "github.com/yutopp/go-flv/tag"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/buffer"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/media"
)

func TestDropUntilVideoKeyframe(t *testing.T) {
	ring := buffer.NewRing(4096)

	if err := ring.Push(media.Frame{
		Kind:  media.KindAudio,
		Audio: &flvtag.AudioData{Data: bytes.NewBuffer([]byte("a"))},
	}); err != nil {
		t.Fatalf("push audio: %v", err)
	}
	if err := ring.Push(media.Frame{
		Kind:  media.KindVideo,
		Video: &flvtag.VideoData{FrameType: flvtag.FrameTypeInterFrame, Data: bytes.NewBuffer([]byte("p"))},
	}); err != nil {
		t.Fatalf("push p-frame: %v", err)
	}
	if err := ring.Push(media.Frame{
		Kind:  media.KindVideo,
		Video: &flvtag.VideoData{FrameType: flvtag.FrameTypeKeyFrame, Data: bytes.NewBuffer([]byte("k"))},
	}); err != nil {
		t.Fatalf("push keyframe: %v", err)
	}

	dropped := ring.DropUntilVideoKeyframe()
	if dropped != 2 {
		t.Fatalf("dropped = %d, want 2", dropped)
	}
	if ring.Len() != 1 {
		t.Fatalf("len = %d, want 1", ring.Len())
	}

	head, ok := ring.PeekOldest()
	if !ok || head.Video == nil || head.Video.FrameType != flvtag.FrameTypeKeyFrame {
		t.Fatal("expected keyframe at head")
	}
}

func TestActiveKeyframeDelaySecondsUsesFirstKeyframe(t *testing.T) {
	ring := buffer.NewRing(4096)
	now := time.Now().UTC()

	if err := ring.Push(media.Frame{
		Kind:       media.KindAudio,
		EnqueuedAt: now.Add(-5 * time.Second),
		Audio:      &flvtag.AudioData{Data: bytes.NewBuffer([]byte("a"))},
	}); err != nil {
		t.Fatalf("push audio: %v", err)
	}
	if err := ring.Push(media.Frame{
		Kind:         media.KindVideo,
		EnqueuedAt:   now.Add(-2 * time.Second),
		VideoPayload: []byte{0x17, 0x01, 0x00, 0x00, 0x00, 0x09},
	}); err != nil {
		t.Fatalf("push keyframe: %v", err)
	}

	if got := ring.ActiveDelaySeconds(now, 5); got != 5 {
		t.Fatalf("active delay = %d, want 5", got)
	}
	if got := ring.ActiveKeyframeDelaySeconds(now, 5); got != 2 {
		t.Fatalf("keyframe delay = %d, want 2", got)
	}
}
