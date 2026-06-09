package slate

import (
	"sync"
	"time"

	flvtag "github.com/yutopp/go-flv/tag"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/media"
)

const (
	audioEmitInterval = 23 * time.Millisecond
	videoEmitInterval = 500 * time.Millisecond
)

type Writer interface {
	WriteVideoPayload(timestamp uint32, payload []byte) error
	WriteAudioPayload(timestamp uint32, payload []byte) error
}

type Timeline interface {
	AdvanceVideo(deltaMs uint32) uint32
	AdvanceAudio(deltaMs uint32) uint32
}

type Emitter struct {
	mu sync.Mutex

	keyframePayload []byte
	audioPayload    []byte

	holdActive    bool
	lastAudioEmit time.Time
	lastVideoEmit time.Time
}

func NewEmitter() *Emitter {
	return &Emitter{}
}

func (e *Emitter) ObserveFrame(frame media.Frame) {
	switch frame.Kind {
	case media.KindVideo:
		if len(frame.VideoPayload) > 0 {
			e.observeVideoPayload(frame.VideoPayload)
			return
		}
		e.observeVideo(frame)
	case media.KindAudio:
		if len(frame.AudioPayload) > 0 {
			e.observeAudioPayload(frame.AudioPayload)
			return
		}
		e.observeAudio(frame)
	}
}

func (e *Emitter) SetDisplay(_ string, _ int) {
}

func (e *Emitter) Ready() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.readyLocked()
}

func (e *Emitter) BeginHold() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.holdActive = true
	e.lastAudioEmit = time.Time{}
	e.lastVideoEmit = time.Time{}
}

func (e *Emitter) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.keyframePayload = nil
	e.audioPayload = nil
	e.holdActive = false
	e.lastAudioEmit = time.Time{}
	e.lastVideoEmit = time.Time{}
}

// EmitBootstrap starts slate on an already-decoding stream.
// Sequence headers are intentionally omitted to avoid mid-stream decoder resets.
func (e *Emitter) EmitBootstrap(w Writer, tl Timeline) bool {
	e.mu.Lock()
	if !e.readyLocked() {
		e.mu.Unlock()
		return false
	}
	keyframe := e.keyframeLocked()
	audio := append([]byte(nil), e.audioPayload...)
	now := time.Now()
	e.lastVideoEmit = now
	e.lastAudioEmit = now
	e.mu.Unlock()

	if len(keyframe) == 0 || len(audio) == 0 {
		return false
	}

	_ = w.WriteVideoPayload(tl.AdvanceVideo(1), keyframe)
	_ = w.WriteAudioPayload(tl.AdvanceAudio(23), audio)
	return true
}

func (e *Emitter) EmitContinuing(w Writer, tl Timeline) {
	e.mu.Lock()
	if !e.readyLocked() || !e.holdActive {
		e.mu.Unlock()
		return
	}

	now := time.Now()
	emitAudio := e.lastAudioEmit.IsZero() || now.Sub(e.lastAudioEmit) >= audioEmitInterval
	emitVideo := e.lastVideoEmit.IsZero() || now.Sub(e.lastVideoEmit) >= videoEmitInterval
	if !emitAudio && !emitVideo {
		e.mu.Unlock()
		return
	}

	keyframe := e.keyframeLocked()
	audio := append([]byte(nil), e.audioPayload...)
	if emitVideo {
		e.lastVideoEmit = now
	}
	if emitAudio {
		e.lastAudioEmit = now
	}
	e.mu.Unlock()

	if emitVideo && len(keyframe) > 0 {
		_ = w.WriteVideoPayload(tl.AdvanceVideo(uint32(videoEmitInterval/time.Millisecond)), keyframe)
	}
	if emitAudio && len(audio) > 0 {
		_ = w.WriteAudioPayload(tl.AdvanceAudio(uint32(audioEmitInterval/time.Millisecond)), audio)
	}
}

func (e *Emitter) observeAudioPayload(payload []byte) {
	if len(payload) < 2 {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if flvtag.SoundFormat(payload[0]>>4) != flvtag.SoundFormatAAC {
		return
	}
	if flvtag.AACPacketType(payload[1]) != flvtag.AACPacketTypeRaw {
		return
	}
	e.audioPayload = append([]byte(nil), payload...)
}

func (e *Emitter) observeVideoPayload(payload []byte) {
	if len(payload) < 2 {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if payload[0]&0x0f != byte(flvtag.CodecIDAVC) {
		return
	}
	if flvtag.AVCPacketType(payload[1]) != flvtag.AVCPacketTypeNALU {
		return
	}
	if payload[0]>>4 != byte(flvtag.FrameTypeKeyFrame) {
		return
	}
	e.keyframePayload = append([]byte(nil), payload...)
}

func (e *Emitter) observeAudio(frame media.Frame) {
	if frame.Audio == nil {
		return
	}
	payload, err := media.EncodeAudioPayload(frame.Audio)
	if err != nil || len(payload) == 0 {
		return
	}
	e.observeAudioPayload(payload)
}

func (e *Emitter) observeVideo(frame media.Frame) {
	if frame.Video == nil {
		return
	}
	payload, err := media.EncodeVideoPayload(frame.Video)
	if err != nil || len(payload) == 0 {
		return
	}
	e.observeVideoPayload(payload)
}

func (e *Emitter) keyframeLocked() []byte {
	if len(e.keyframePayload) == 0 {
		return nil
	}
	return append([]byte(nil), e.keyframePayload...)
}

func (e *Emitter) readyLocked() bool {
	return len(e.keyframeLocked()) > 0 && len(e.audioPayload) > 0
}
