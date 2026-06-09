package media

import (
	"bytes"
	"io"
	"time"

	flvtag "github.com/yutopp/go-flv/tag"
	rtmpmsg "github.com/yutopp/go-rtmp/message"
)

type Kind int

const (
	KindAudio Kind = iota
	KindVideo
	KindSetDataFrame
	KindOnMetaData
)

type Frame struct {
	Kind       Kind
	Timestamp  uint32
	EnqueuedAt time.Time
	SetData    *rtmpmsg.NetStreamSetDataFrame
	MetaData   []byte
	Audio      *flvtag.AudioData
	Video      *flvtag.VideoData
}

func (f Frame) ByteSize() int64 {
	switch f.Kind {
	case KindSetDataFrame:
		if f.SetData == nil {
			return 0
		}
		return int64(len(f.SetData.Payload))
	case KindOnMetaData:
		return int64(len(f.MetaData))
	case KindAudio:
		if f.Audio == nil {
			return 0
		}
		return int64(audioPayloadSize(f.Audio))
	case KindVideo:
		if f.Video == nil {
			return 0
		}
		return int64(videoPayloadSize(f.Video))
	default:
		return 0
	}
}

func audioPayloadSize(audio *flvtag.AudioData) int {
	return readerLen(audio.Data) + 2
}

func videoPayloadSize(video *flvtag.VideoData) int {
	return readerLen(video.Data) + 5
}

func readerLen(r io.Reader) int {
	if r == nil {
		return 0
	}
	if buf, ok := r.(*bytes.Buffer); ok {
		return buf.Len()
	}
	return 0
}

func CloneAudio(audio *flvtag.AudioData) (*flvtag.AudioData, error) {
	if audio == nil || audio.Data == nil {
		return nil, nil
	}
	body := new(bytes.Buffer)
	if _, err := io.Copy(body, audio.Data); err != nil {
		return nil, err
	}
	return &flvtag.AudioData{
		SoundFormat:   audio.SoundFormat,
		SoundRate:     audio.SoundRate,
		SoundSize:     audio.SoundSize,
		SoundType:     audio.SoundType,
		AACPacketType: audio.AACPacketType,
		Data:          body,
	}, nil
}

func CloneVideo(video *flvtag.VideoData) (*flvtag.VideoData, error) {
	if video == nil || video.Data == nil {
		return nil, nil
	}
	body := new(bytes.Buffer)
	if _, err := io.Copy(body, video.Data); err != nil {
		return nil, err
	}
	return &flvtag.VideoData{
		FrameType:       video.FrameType,
		CodecID:         video.CodecID,
		AVCPacketType:   video.AVCPacketType,
		CompositionTime: video.CompositionTime,
		Data:            body,
	}, nil
}
