package output

import (
	"bytes"
	"fmt"
	"sync"

	flvtag "github.com/yutopp/go-flv/tag"
	"github.com/yutopp/go-rtmp"
	rtmpmsg "github.com/yutopp/go-rtmp/message"
)

const (
	audioChunkStreamID = 5
	videoChunkStreamID = 6
	dataChunkStreamID  = 8
)

type RTMPPublisher struct {
	mu     sync.Mutex
	dest   Destination
	client *rtmp.ClientConn
	stream *rtmp.Stream
	closed bool
}

func ConnectPublisher(dest Destination) (*RTMPPublisher, error) {
	publisher := &RTMPPublisher{dest: dest}
	if err := publisher.connect(); err != nil {
		return nil, err
	}
	return publisher, nil
}

func (p *RTMPPublisher) Reconnect() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.closeLocked(); err != nil {
		return err
	}
	p.closed = false
	return p.connectLocked()
}

func (p *RTMPPublisher) connect() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.connectLocked()
}

func (p *RTMPPublisher) connectLocked() error {
	var client *rtmp.ClientConn
	var err error
	switch p.dest.Scheme {
	case "rtmps":
		client, err = rtmp.TLSDial("rtmps", p.dest.HostPort, &rtmp.ConnConfig{}, nil)
	default:
		client, err = rtmp.Dial("rtmp", p.dest.HostPort, &rtmp.ConnConfig{})
	}
	if err != nil {
		return fmt.Errorf("dial output %s: %w", p.dest.HostPort, err)
	}

	connectBody := &rtmpmsg.NetConnectionConnect{
		Command: rtmpmsg.NetConnectionConnectCommand{
			App:            p.dest.App,
			FlashVer:       "FMLE/3.0 (compatible; DelayDeck Relay)",
			TCURL:          p.dest.TCURL,
			Fpad:           false,
			Capabilities:   15,
			AudioCodecs:    3191,
			VideoCodecs:    252,
			VideoFunction:  1,
			ObjectEncoding: rtmpmsg.EncodingTypeAMF0,
		},
	}
	if err := client.Connect(connectBody); err != nil {
		_ = client.Close()
		return fmt.Errorf("connect output: %w", err)
	}

	stream, err := client.CreateStream(nil, 4096)
	if err != nil {
		_ = client.Close()
		return fmt.Errorf("create output stream: %w", err)
	}

	if err := stream.Publish(&rtmpmsg.NetStreamPublish{
		PublishingName: p.dest.StreamKey,
		PublishingType: "live",
	}); err != nil {
		_ = client.Close()
		return fmt.Errorf("publish output stream: %w", err)
	}

	p.client = client
	p.stream = stream
	return nil
}

func (p *RTMPPublisher) WriteSetDataFrame(timestamp uint32, frame *rtmpmsg.NetStreamSetDataFrame) error {
	if len(frame.Payload) == 0 {
		return nil
	}
	return p.writeNamedData(timestamp, "@setDataFrame", frame.Payload)
}

func (p *RTMPPublisher) WriteOnMetaData(timestamp uint32, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	return p.writeNamedData(timestamp, "onMetaData", payload)
}

func (p *RTMPPublisher) WriteAudioPayload(timestamp uint32, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	return p.writeAudioBytes(timestamp, payload)
}

func (p *RTMPPublisher) WriteVideoPayload(timestamp uint32, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	return p.writeVideoBytes(timestamp, payload)
}

func (p *RTMPPublisher) WriteAudioData(timestamp uint32, audio *flvtag.AudioData) error {
	buf := new(bytes.Buffer)
	if err := flvtag.EncodeAudioData(buf, audio); err != nil {
		return fmt.Errorf("encode audio: %w", err)
	}
	return p.writeAudioBytes(timestamp, buf.Bytes())
}

func (p *RTMPPublisher) WriteVideoData(timestamp uint32, video *flvtag.VideoData) error {
	buf := new(bytes.Buffer)
	if err := flvtag.EncodeVideoData(buf, video); err != nil {
		return fmt.Errorf("encode video: %w", err)
	}
	return p.writeVideoBytes(timestamp, buf.Bytes())
}

func (p *RTMPPublisher) writeNamedData(timestamp uint32, name string, payload []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return fmt.Errorf("output publisher is closed")
	}

	return p.stream.Write(dataChunkStreamID, timestamp, &rtmpmsg.DataMessage{
		Name:     name,
		Encoding: rtmpmsg.EncodingTypeAMF0,
		Body:     bytes.NewReader(payload),
	})
}

func (p *RTMPPublisher) writeAudioBytes(timestamp uint32, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return fmt.Errorf("output publisher is closed")
	}

	return p.stream.Write(audioChunkStreamID, timestamp, &rtmpmsg.AudioMessage{
		Payload: bytes.NewReader(data),
	})
}

func (p *RTMPPublisher) writeVideoBytes(timestamp uint32, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return fmt.Errorf("output publisher is closed")
	}

	return p.stream.Write(videoChunkStreamID, timestamp, &rtmpmsg.VideoMessage{
		Payload: bytes.NewReader(data),
	})
}

func (p *RTMPPublisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closeLocked()
}

func (p *RTMPPublisher) closeLocked() error {
	if p.closed {
		return nil
	}
	p.closed = true

	if p.client != nil && p.stream != nil {
		_ = p.client.DeleteStream(&rtmpmsg.NetStreamDeleteStream{
			StreamID: p.stream.StreamID(),
		})
	}
	if p.client != nil {
		return p.client.Close()
	}
	return nil
}
