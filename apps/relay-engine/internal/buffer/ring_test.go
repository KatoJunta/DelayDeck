package buffer_test

import (
	"testing"
	"time"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/buffer"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/media"
)

func TestRingPushAndPop(t *testing.T) {
	ring := buffer.NewRing(1024)

	frame := media.Frame{
		Kind:       media.KindOnMetaData,
		MetaData:   []byte("meta"),
		EnqueuedAt: time.Now().UTC(),
	}
	if err := ring.Push(frame); err != nil {
		t.Fatalf("push: %v", err)
	}
	if ring.UsedBytes() != int64(len(frame.MetaData)) {
		t.Fatalf("used bytes = %d", ring.UsedBytes())
	}

	popped, ok := ring.PopOldest()
	if !ok {
		t.Fatal("expected frame")
	}
	if string(popped.MetaData) != "meta" {
		t.Fatalf("metadata = %q", popped.MetaData)
	}
	if ring.Len() != 0 {
		t.Fatal("ring should be empty")
	}
}

func TestRingOverflowRejectsPush(t *testing.T) {
	ring := buffer.NewRing(10)

	err := ring.Push(media.Frame{
		Kind:     media.KindOnMetaData,
		MetaData: []byte("12345678901"),
	})
	if err != buffer.ErrFrameTooLarge {
		t.Fatalf("expected ErrFrameTooLarge, got %v", err)
	}

	if err := ring.Push(media.Frame{Kind: media.KindOnMetaData, MetaData: []byte("12345")}); err != nil {
		t.Fatalf("first push: %v", err)
	}
	err = ring.Push(media.Frame{Kind: media.KindOnMetaData, MetaData: []byte("678901")})
	if err != buffer.ErrBufferOverflow {
		t.Fatalf("expected ErrBufferOverflow, got %v", err)
	}
	if ring.Len() != 1 {
		t.Fatalf("len = %d, want 1", ring.Len())
	}
}

func TestActiveDelaySeconds(t *testing.T) {
	ring := buffer.NewRing(1024)
	enqueuedAt := time.Now().UTC().Add(-15 * time.Second)
	if err := ring.Push(media.Frame{
		Kind:       media.KindOnMetaData,
		MetaData:   []byte("x"),
		EnqueuedAt: enqueuedAt,
	}); err != nil {
		t.Fatalf("push: %v", err)
	}

	active := ring.ActiveDelaySeconds(time.Now().UTC(), 30)
	if active < 14 || active > 16 {
		t.Fatalf("active delay = %d, want about 15", active)
	}
}

func TestActiveDelayDurationKeepsSubsecondPrecision(t *testing.T) {
	ring := buffer.NewRing(1024)
	now := time.Now().UTC()
	if err := ring.Push(media.Frame{
		Kind:       media.KindOnMetaData,
		MetaData:   []byte("x"),
		EnqueuedAt: now.Add(-1500 * time.Millisecond),
	}); err != nil {
		t.Fatalf("push: %v", err)
	}

	active := ring.ActiveDelayDuration(now, 30)
	if active < 1400*time.Millisecond || active > 1600*time.Millisecond {
		t.Fatalf("active delay = %s, want about 1.5s", active)
	}
}

func TestBufferSpanDuration(t *testing.T) {
	ring := buffer.NewRing(1024)
	now := time.Now().UTC()
	if err := ring.Push(media.Frame{
		Kind:       media.KindOnMetaData,
		MetaData:   []byte("a"),
		EnqueuedAt: now.Add(-10 * time.Second),
	}); err != nil {
		t.Fatalf("push oldest: %v", err)
	}
	if err := ring.Push(media.Frame{
		Kind:       media.KindOnMetaData,
		MetaData:   []byte("b"),
		EnqueuedAt: now.Add(-2500 * time.Millisecond),
	}); err != nil {
		t.Fatalf("push newest: %v", err)
	}

	span := ring.BufferSpanDuration()
	if span < 7400*time.Millisecond || span > 7600*time.Millisecond {
		t.Fatalf("buffer span = %s, want about 7.5s", span)
	}
}

func TestActiveKeyframeDelayDurationUsesFirstKeyframe(t *testing.T) {
	ring := buffer.NewRing(1024)
	now := time.Now().UTC()
	if err := ring.Push(media.Frame{
		Kind:         media.KindVideo,
		VideoPayload: []byte{0x27, 0x01, 0x00, 0x00, 0x00},
		EnqueuedAt:   now.Add(-4 * time.Second),
	}); err != nil {
		t.Fatalf("push inter frame: %v", err)
	}
	if err := ring.Push(media.Frame{
		Kind:         media.KindVideo,
		VideoPayload: []byte{0x17, 0x01, 0x00, 0x00, 0x00},
		EnqueuedAt:   now.Add(-1500 * time.Millisecond),
	}); err != nil {
		t.Fatalf("push keyframe: %v", err)
	}

	active := ring.ActiveKeyframeDelayDuration(now, 30)
	if active < 1400*time.Millisecond || active > 1600*time.Millisecond {
		t.Fatalf("keyframe delay = %s, want about 1.5s", active)
	}
}
