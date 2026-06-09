package media

import (
	"bytes"
	"io"

	flvtag "github.com/yutopp/go-flv/tag"
	rtmpmsg "github.com/yutopp/go-rtmp/message"
)

func ClonePayloadFrame(frame Frame) Frame {
	cloned := frame
	if len(frame.AudioPayload) > 0 {
		cloned.AudioPayload = append([]byte(nil), frame.AudioPayload...)
	}
	if len(frame.VideoPayload) > 0 {
		cloned.VideoPayload = append([]byte(nil), frame.VideoPayload...)
	}
	if len(frame.MetaData) > 0 {
		cloned.MetaData = append([]byte(nil), frame.MetaData...)
	}
	if frame.SetData != nil {
		cloned.SetData = &rtmpmsg.NetStreamSetDataFrame{
			Payload: append([]byte(nil), frame.SetData.Payload...),
		}
	}
	cloned.Audio = nil
	cloned.Video = nil
	return cloned
}

func IsVideoKeyframePayload(payload []byte) bool {
	if len(payload) < 2 {
		return false
	}
	frameType := payload[0] >> 4
	if frameType != byte(flvtag.FrameTypeKeyFrame) {
		return false
	}
	if payload[0]&0x0f != byte(flvtag.CodecIDAVC) {
		return false
	}
	return flvtag.AVCPacketType(payload[1]) == flvtag.AVCPacketTypeNALU
}

func DecodeVideoPayload(payload []byte) (*flvtag.VideoData, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	var video flvtag.VideoData
	if err := flvtag.DecodeVideoData(bytes.NewReader(payload), &video); err != nil {
		return nil, err
	}
	body, err := io.ReadAll(video.Data)
	if err != nil {
		return nil, err
	}
	video.Data = bytes.NewBuffer(body)
	return &video, nil
}

func DecodeAudioPayload(payload []byte) (*flvtag.AudioData, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	var audio flvtag.AudioData
	if err := flvtag.DecodeAudioData(bytes.NewReader(payload), &audio); err != nil {
		return nil, err
	}
	body, err := io.ReadAll(audio.Data)
	if err != nil {
		return nil, err
	}
	audio.Data = bytes.NewBuffer(body)
	return &audio, nil
}
