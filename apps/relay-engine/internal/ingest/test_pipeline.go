package ingest

import (
	flvtag "github.com/yutopp/go-flv/tag"
	rtmpmsg "github.com/yutopp/go-rtmp/message"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/buffer"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/scheduler"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/state"
)

type noopWriter struct{}

func (noopWriter) WriteSetDataFrame(_ uint32, _ *rtmpmsg.NetStreamSetDataFrame) error {
	return nil
}

func (noopWriter) WriteOnMetaData(_ uint32, _ []byte) error {
	return nil
}

func (noopWriter) WriteAudioPayload(_ uint32, _ []byte) error  { return nil }
func (noopWriter) WriteVideoPayload(_ uint32, _ []byte) error  { return nil }
func (noopWriter) WriteAudioData(_ uint32, _ *flvtag.AudioData) error { return nil }
func (noopWriter) WriteVideoData(_ uint32, _ *flvtag.VideoData) error { return nil }

func NewPipelineForTest(machine *state.Machine, bufferCapacityBytes int64) *Pipeline {
	ring := buffer.NewRing(bufferCapacityBytes)
	controller := scheduler.NewController(ring, noopWriter{}, machine)
	controller.Start()
	return &Pipeline{
		machine:    machine,
		controller: controller,
	}
}
