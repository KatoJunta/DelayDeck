package ingest

import (
	rtmpmsg "github.com/yutopp/go-rtmp/message"

	flvtag "github.com/yutopp/go-flv/tag"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/buffer"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/media"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/output"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/scheduler"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/state"
)

type Pipeline struct {
	scheduler *scheduler.Fixed
}

func NewPipeline(
	publisher *output.RTMPPublisher,
	machine *state.Machine,
	bufferCapacityBytes int64,
	fixedDelaySeconds int,
) *Pipeline {
	ring := buffer.NewRing(bufferCapacityBytes)
	machine.SetFixedDelayTarget(fixedDelaySeconds)

	sched := scheduler.NewFixed(ring, publisher, machine, fixedDelaySeconds)
	sched.Start()

	return &Pipeline{scheduler: sched}
}

func (p *Pipeline) Stop() {
	if p == nil || p.scheduler == nil {
		return
	}
	p.scheduler.Stop()
	p.scheduler.Clear()
}

func (p *Pipeline) PushSetDataFrame(timestamp uint32, data *rtmpmsg.NetStreamSetDataFrame) error {
	if p == nil {
		return nil
	}
	payload := append([]byte(nil), data.Payload...)
	return p.scheduler.Push(media.Frame{
		Kind:      media.KindSetDataFrame,
		Timestamp: timestamp,
		SetData:   &rtmpmsg.NetStreamSetDataFrame{Payload: payload},
	})
}

func (p *Pipeline) PushOnMetaData(payload []byte) error {
	if p == nil {
		return nil
	}
	return p.scheduler.Push(media.Frame{
		Kind:     media.KindOnMetaData,
		MetaData: append([]byte(nil), payload...),
	})
}

func (p *Pipeline) PushAudio(timestamp uint32, audio *flvtag.AudioData) error {
	if p == nil {
		return nil
	}
	cloned, err := media.CloneAudio(audio)
	if err != nil {
		return err
	}
	return p.scheduler.Push(media.Frame{
		Kind:  media.KindAudio,
		Timestamp: timestamp,
		Audio: cloned,
	})
}

func (p *Pipeline) PushVideo(timestamp uint32, video *flvtag.VideoData) error {
	if p == nil {
		return nil
	}
	cloned, err := media.CloneVideo(video)
	if err != nil {
		return err
	}
	return p.scheduler.Push(media.Frame{
		Kind:  media.KindVideo,
		Timestamp: timestamp,
		Video: cloned,
	})
}

func (p *Pipeline) Enabled() bool {
	return p != nil && p.scheduler != nil
}
