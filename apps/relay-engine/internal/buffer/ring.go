package buffer

import (
	"errors"
	"sync"
	"time"

	flvtag "github.com/yutopp/go-flv/tag"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/media"
)

var (
	ErrBufferOverflow = errors.New("buffer capacity exceeded")
	ErrFrameTooLarge  = errors.New("frame exceeds buffer capacity")
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

func (r *Ring) DropUntilVideoKeyframe() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	dropped := 0
	for len(r.entries) > 0 {
		frame := r.entries[0]
		if isVideoKeyframe(frame) {
			break
		}
		r.entries = r.entries[1:]
		r.usedBytes -= frame.ByteSize()
		dropped++
	}
	if r.usedBytes < 0 {
		r.usedBytes = 0
	}
	return dropped
}

func (r *Ring) ActiveKeyframeDelaySeconds(now time.Time, targetSeconds int) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if targetSeconds <= 0 {
		return 0
	}

	for _, frame := range r.entries {
		if !isVideoKeyframe(frame) {
			continue
		}
		elapsed := int(now.Sub(frame.EnqueuedAt).Seconds())
		if elapsed < 0 {
			return 0
		}
		if elapsed > targetSeconds {
			return targetSeconds
		}
		return elapsed
	}
	return 0
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

func isVideoKeyframe(frame media.Frame) bool {
	if frame.Kind != media.KindVideo {
		return false
	}
	if len(frame.VideoPayload) > 0 {
		return media.IsVideoKeyframePayload(frame.VideoPayload)
	}
	return frame.Video != nil && frame.Video.FrameType == flvtag.FrameTypeKeyFrame
}

func (r *Ring) UsagePercent() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.capacityBytes <= 0 {
		return 0
	}
	return float64(r.usedBytes) / float64(r.capacityBytes) * 100
}
