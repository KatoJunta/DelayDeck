package buffer

import (
	"errors"
	"sync"
	"time"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/media"
)

var (
	ErrBufferOverflow = errors.New("buffer capacity exceeded")
	ErrFrameTooLarge    = errors.New("frame exceeds buffer capacity")
)

// Ring is a bounded FIFO queue measured in bytes.
// When full, Push returns ErrBufferOverflow instead of evicting frames.
type Ring struct {
	mu            sync.Mutex
	capacityBytes int64
	usedBytes     int64
	entries       []media.Frame
}

func NewRing(capacityBytes int64) *Ring {
	return &Ring{capacityBytes: capacityBytes}
}

func (r *Ring) CapacityBytes() int64 {
	return r.capacityBytes
}

func (r *Ring) UsedBytes() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.usedBytes
}

func (r *Ring) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.entries)
}

func (r *Ring) Push(frame media.Frame) error {
	size := frame.ByteSize()
	if size <= 0 {
		return nil
	}
	if size > r.capacityBytes {
		return ErrFrameTooLarge
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.usedBytes+size > r.capacityBytes {
		return ErrBufferOverflow
	}

	if frame.EnqueuedAt.IsZero() {
		frame.EnqueuedAt = time.Now().UTC()
	}

	r.entries = append(r.entries, frame)
	r.usedBytes += size
	return nil
}

func (r *Ring) PeekOldest() (media.Frame, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.entries) == 0 {
		return media.Frame{}, false
	}
	return r.entries[0], true
}

func (r *Ring) PopOldest() (media.Frame, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.entries) == 0 {
		return media.Frame{}, false
	}

	frame := r.entries[0]
	r.entries = r.entries[1:]
	r.usedBytes -= frame.ByteSize()
	if r.usedBytes < 0 {
		r.usedBytes = 0
	}
	return frame, true
}

func (r *Ring) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = nil
	r.usedBytes = 0
}

func (r *Ring) ActiveDelaySeconds(now time.Time, targetSeconds int) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.entries) == 0 || targetSeconds <= 0 {
		return 0
	}

	elapsed := int(now.Sub(r.entries[0].EnqueuedAt).Seconds())
	if elapsed < 0 {
		return 0
	}
	if elapsed > targetSeconds {
		return targetSeconds
	}
	return elapsed
}

func (r *Ring) UsagePercent() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.capacityBytes <= 0 {
		return 0
	}
	return float64(r.usedBytes) / float64(r.capacityBytes) * 100
}
