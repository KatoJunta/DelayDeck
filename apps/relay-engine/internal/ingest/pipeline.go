package ingest

import (
	"bytes"

	"fmt"

	"sync"

	"time"

	rtmpmsg "github.com/yutopp/go-rtmp/message"

	flvtag "github.com/yutopp/go-flv/tag"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/buffer"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/media"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/output"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/scheduler"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/state"
)

type Pipeline struct {
	machine *state.Machine

	controller *scheduler.Controller

	mu sync.Mutex

	cancelled bool

	sessionID uint64
}

func NewPipeline(

	publisher *output.RTMPPublisher,

	machine *state.Machine,

	bufferCapacityBytes int64,

	initialDelaySeconds int,

) *Pipeline {

	ring := buffer.NewRing(bufferCapacityBytes)

	controller := scheduler.NewController(ring, publisher, machine)

	if initialDelaySeconds > 0 {

		machine.SetFixedDelayTarget(initialDelaySeconds)

		controller.SetTargetDelay(initialDelaySeconds)

		controller.SetPolicy(scheduler.OutputDelayed)

	}

	controller.Start()

	return &Pipeline{

		machine: machine,

		controller: controller,
	}

}

func (p *Pipeline) Stop() {

	if p == nil || p.controller == nil {

		return

	}

	p.mu.Lock()

	p.cancelled = true

	p.mu.Unlock()

	p.controller.Stop()

	p.controller.Clear()

}

func (p *Pipeline) isCancelled() bool {

	if p == nil {

		return true

	}

	p.mu.Lock()

	defer p.mu.Unlock()

	return p.cancelled

}

func (p *Pipeline) BeginEnableDelay(targetSeconds int) error {

	if p == nil || p.controller == nil {

		return fmt.Errorf("media pipeline is not active")

	}

	if targetSeconds <= 0 {

		return fmt.Errorf("target delay must be greater than zero")

	}

	p.machine.SetFixedDelayTarget(targetSeconds)

	p.controller.BeginBufferingFill(targetSeconds)

	return nil

}

func (p *Pipeline) RunEnableDelayFill(targetSeconds int, publish func(slate string, countdown int)) {
	if p == nil || p.controller == nil {
		return
	}

	publish = p.wrapSlatePublish(publish)
	p.controller.BeginSlateHold(true)

	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()

	for {
		if p.isCancelled() {
			return
		}

		active := p.controller.ActiveKeyframeDelaySeconds()
		if active >= targetSeconds {
			return
		}

		remaining := targetSeconds - active
		if remaining < 1 {
			remaining = 1
		}
		publish(fmt.Sprintf(state.SlateEnableDelayFmt, remaining), remaining)

		<-tick.C
	}
}

func (p *Pipeline) CompleteEnableDelay() {

	if p == nil || p.controller == nil {

		return

	}

	p.controller.BeginDelayedOutput()

}

func (p *Pipeline) BeginReturnLive() error {

	if p == nil || p.controller == nil {

		return fmt.Errorf("media pipeline is not active")

	}

	p.controller.BeginDrainAtLive()

	return nil

}

func (p *Pipeline) RunReturnLiveDrain(publish func(slate string, countdown int)) {
	if p == nil || p.controller == nil {
		return
	}

	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()

	for {
		if p.isCancelled() {
			return
		}
		if p.controller.RingLen() == 0 {
			publish("", 0)
			return
		}

		active := p.controller.ActiveDelaySeconds()
		if active < 1 {
			active = 1
		}
		publish(fmt.Sprintf(state.SlateDrainingBufferFmt, active), active)

		<-tick.C
	}
}

func (p *Pipeline) RunReturnLiveSlate(publish func(slate string, countdown int)) {

	p.runTimedSlate(state.SlateReturningLive, state.ReturnLiveSlateSeconds, publish)

}

func (p *Pipeline) BeginDumpSlate() error {

	if p == nil || p.controller == nil {

		return fmt.Errorf("media pipeline is not active")

	}

	p.controller.Clear()

	p.controller.BeginSlateHold(false)

	return nil

}

func (p *Pipeline) RunDumpSlate(publish func(slate string, countdown int)) {

	p.runTimedSlate(state.SlateReturningLive, state.DumpSlateSeconds, publish)

}

func (p *Pipeline) ResumePassthrough() {

	if p == nil || p.controller == nil {

		return

	}

	p.machine.SetFixedDelayTarget(0)

	p.controller.BeginPassthrough()

}

func (p *Pipeline) runTimedSlate(message string, seconds int, publish func(slate string, countdown int)) {
	if p == nil || p.controller == nil {
		return
	}

	publish = p.wrapSlatePublish(publish)
	p.controller.BeginSlateHold(false)

	for remaining := seconds; remaining > 0; remaining-- {
		if p.isCancelled() {
			return
		}
		publish(message, remaining)
		time.Sleep(1 * time.Second)
	}
	publish("", 0)
}

func (p *Pipeline) wrapSlatePublish(publish func(slate string, countdown int)) func(slate string, countdown int) {
	return func(message string, countdown int) {
		if p.controller != nil {
			p.controller.SetSlateDisplay(message, countdown)
		}
		if publish != nil {
			publish(message, countdown)
		}
	}
}

func (p *Pipeline) PushSetDataFrame(timestamp uint32, data *rtmpmsg.NetStreamSetDataFrame) error {

	if p == nil {

		return nil

	}

	payload := append([]byte(nil), data.Payload...)

	return p.controller.Push(media.Frame{

		Kind: media.KindSetDataFrame,

		Timestamp: timestamp,

		SetData: &rtmpmsg.NetStreamSetDataFrame{Payload: payload},
	})

}

func (p *Pipeline) PushOnMetaData(payload []byte) error {

	if p == nil {

		return nil

	}

	return p.controller.Push(media.Frame{

		Kind: media.KindOnMetaData,

		MetaData: append([]byte(nil), payload...),
	})

}

func (p *Pipeline) PushAudioPayload(timestamp uint32, payload []byte) error {

	if p == nil {

		return nil

	}

	return p.controller.Push(media.Frame{

		Kind: media.KindAudio,

		Timestamp: timestamp,

		AudioPayload: append([]byte(nil), payload...),
	})

}

func (p *Pipeline) PushVideoPayload(timestamp uint32, payload []byte) error {

	if p == nil {

		return nil

	}

	return p.controller.Push(media.Frame{

		Kind: media.KindVideo,

		Timestamp: timestamp,

		VideoPayload: append([]byte(nil), payload...),
	})

}

func (p *Pipeline) PushAudio(timestamp uint32, audio *flvtag.AudioData) error {

	if p == nil {

		return nil

	}

	buf := new(bytes.Buffer)

	if err := flvtag.EncodeAudioData(buf, audio); err != nil {

		return err

	}

	return p.PushAudioPayload(timestamp, buf.Bytes())

}

func (p *Pipeline) PushVideo(timestamp uint32, video *flvtag.VideoData) error {

	if p == nil {

		return nil

	}

	buf := new(bytes.Buffer)

	if err := flvtag.EncodeVideoData(buf, video); err != nil {

		return err

	}

	return p.PushVideoPayload(timestamp, buf.Bytes())

}

func (p *Pipeline) Enabled() bool {

	return p != nil && p.controller != nil

}

func (p *Pipeline) OutputPolicy() scheduler.OutputPolicy {

	if p == nil || p.controller == nil {

		return scheduler.OutputPassthrough

	}

	return p.controller.Policy()

}

func (p *Pipeline) SessionID() uint64 {

	if p == nil {

		return 0

	}

	p.mu.Lock()

	defer p.mu.Unlock()

	return p.sessionID

}
