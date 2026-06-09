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
		Kind:      media.KindOnMetaData,
		MetaData:  []byte("meta"),
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
