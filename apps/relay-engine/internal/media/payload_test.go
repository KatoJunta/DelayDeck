package media_test

import (
	"bytes"
	"testing"

	flvtag "github.com/yutopp/go-flv/tag"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/media"
)

func TestVideoPayloadRoundtripIsLossless(t *testing.T) {
	original := []byte{
		0x27, 0x01, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x04, 0x65, 0x88, 0x84, 0x00,
	}

	var video flvtag.VideoData
	if err := flvtag.DecodeVideoData(bytes.NewReader(original), &video); err != nil {
		t.Fatalf("decode: %v", err)
	}

	body := new(bytes.Buffer)
	if _, err := body.ReadFrom(video.Data); err != nil {
		t.Fatalf("read body: %v", err)
	}
	video.Data = body

	encoded := new(bytes.Buffer)
	if err := flvtag.EncodeVideoData(encoded, &video); err != nil {
		t.Fatalf("encode: %v", err)
	}

	if !bytes.Equal(original, encoded.Bytes()) {
		t.Fatalf("encode/decode changed payload:\norig=% x\nenc =% x", original, encoded.Bytes())
	}
}

func TestIsVideoKeyframePayload(t *testing.T) {
	keyframe := []byte{0x17, 0x01, 0x00, 0x00, 0x00, 0x00}
	if !media.IsVideoKeyframePayload(keyframe) {
		t.Fatal("expected keyframe payload")
	}

	seqHeader := []byte{0x17, 0x00, 0x00, 0x00, 0x00}
	if media.IsVideoKeyframePayload(seqHeader) {
		t.Fatal("sequence header must not count as keyframe")
	}
}
