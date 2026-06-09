package scheduler

import (
	"fmt"

	flvtag "github.com/yutopp/go-flv/tag"
	rtmpmsg "github.com/yutopp/go-rtmp/message"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/media"
)

type FrameWriter interface {
	WriteSetDataFrame(timestamp uint32, frame *rtmpmsg.NetStreamSetDataFrame) error
	WriteOnMetaData(timestamp uint32, payload []byte) error
	WriteAudioPayload(timestamp uint32, payload []byte) error
	WriteVideoPayload(timestamp uint32, payload []byte) error
	WriteAudioData(timestamp uint32, audio *flvtag.AudioData) error
	WriteVideoData(timestamp uint32, video *flvtag.VideoData) error
}

func WriteFrame(writer FrameWriter, frame media.Frame) error {
	switch frame.Kind {
	case media.KindSetDataFrame:
		if frame.SetData == nil {
			return nil
		}
		return writer.WriteSetDataFrame(frame.Timestamp, frame.SetData)
	case media.KindOnMetaData:
		return writer.WriteOnMetaData(frame.Timestamp, frame.MetaData)
	case media.KindAudio:
		if len(frame.AudioPayload) > 0 {
			return writer.WriteAudioPayload(frame.Timestamp, frame.AudioPayload)
		}
		if frame.Audio == nil {
			return nil
		}
		return writer.WriteAudioData(frame.Timestamp, frame.Audio)
	case media.KindVideo:
		if len(frame.VideoPayload) > 0 {
			return writer.WriteVideoPayload(frame.Timestamp, frame.VideoPayload)
		}
		if frame.Video == nil {
			return nil
		}
		return writer.WriteVideoData(frame.Timestamp, frame.Video)
	default:
		return fmt.Errorf("unknown frame kind")
	}
}
