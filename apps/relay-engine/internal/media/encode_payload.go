package media

import (
	"bytes"

	flvtag "github.com/yutopp/go-flv/tag"
)

func EncodeVideoPayload(video *flvtag.VideoData) ([]byte, error) {
	if video == nil {
		return nil, nil
	}
	cloned, err := CloneVideo(video)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	if err := flvtag.EncodeVideoData(buf, cloned); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func EncodeAudioPayload(audio *flvtag.AudioData) ([]byte, error) {
	if audio == nil {
		return nil, nil
	}
	cloned, err := CloneAudio(audio)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	if err := flvtag.EncodeAudioData(buf, cloned); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
