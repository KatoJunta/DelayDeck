package ingest

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	flvtag "github.com/yutopp/go-flv/tag"
	"github.com/yutopp/go-rtmp"
	rtmpmsg "github.com/yutopp/go-rtmp/message"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/buffer"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/output"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/rtmpcompat"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/state"
)

type ForwardingOptions struct {
	FixedDelaySeconds   int
	BufferCapacityBytes int64
}

type RTMPServer struct {
	mu            sync.Mutex
	ln            net.Listener
	addr          string
	server        *rtmp.Server
	machine       *state.Machine
	dest          output.Destination
	options       ForwardingOptions
	activeHandler *publishHandler
}

func StartRTMPServer(
	listenAddress string,
	dest output.Destination,
	machine *state.Machine,
	options ForwardingOptions,
) (*RTMPServer, error) {
	ln, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", listenAddress, err)
	}

	srv := &RTMPServer{
		ln:      ln,
		addr:    ln.Addr().String(),
		machine: machine,
		dest:    dest,
		options: options,
	}

	rtmpcompat.SetMetaForwarder(srv)

	srv.server = rtmp.NewServer(&rtmp.ServerConfig{
		OnConnect: func(conn net.Conn) (io.ReadWriteCloser, *rtmp.ConnConfig) {
			handler := &publishHandler{
				server: srv,
			}
			return conn, &rtmp.ConnConfig{
				Handler: handler,
				ControlState: rtmp.StreamControlStateConfig{
					DefaultBandwidthWindowSize: 6 * 1024 * 1024 / 8,
					DefaultBandwidthLimitType:  rtmpmsg.LimitTypeSoft,
				},
			}
		},
	})

	go func() {
		if err := srv.server.Serve(ln); err != nil {
			log.Printf("delaydeck-relay: ingest server stopped: %v", err)
		}
	}()

	return srv, nil
}

func (s *RTMPServer) Address() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

func (s *RTMPServer) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	rtmpcompat.SetMetaForwarder(nil)
	if s.ln == nil {
		return nil
	}
	err := s.ln.Close()
	s.ln = nil
	return err
}

func (s *RTMPServer) ForwardOnMetaData(payload []byte) {
	s.mu.Lock()
	handler := s.activeHandler
	s.mu.Unlock()
	if handler == nil {
		return
	}
	handler.forwardOnMetaData(payload)
}

type publishHandler struct {
	rtmp.DefaultHandler

	mu            sync.Mutex
	server        *RTMPServer
	publisher     *output.RTMPPublisher
	pipeline      *Pipeline
	sessionActive bool
}

func (h *publishHandler) OnPublish(_ *rtmp.StreamContext, _ uint32, cmd *rtmpmsg.NetStreamPublish) error {
	if cmd.PublishingName == "" {
		return fmt.Errorf("publishing name is empty")
	}

	h.endSession()
	if err := h.server.machine.DisconnectSession(); err != nil {
		return err
	}

	publisher, err := output.ConnectPublisher(h.server.dest)
	if err != nil {
		_ = h.server.machine.MarkError("output connection failed")
		return err
	}

	if err := h.server.machine.ConnectInput(); err != nil {
		_ = publisher.Close()
		return err
	}
	if err := h.server.machine.ConnectOutput(); err != nil {
		_ = publisher.Close()
		return err
	}

	h.mu.Lock()
	h.publisher = publisher
	if h.server.options.FixedDelaySeconds > 0 {
		h.pipeline = NewPipeline(
			publisher,
			h.server.machine,
			h.server.options.BufferCapacityBytes,
			h.server.options.FixedDelaySeconds,
		)
	}
	h.sessionActive = true
	h.mu.Unlock()

	h.server.mu.Lock()
	h.server.activeHandler = h
	h.server.mu.Unlock()

	return nil
}

func (h *publishHandler) OnFCUnpublish(_ uint32, _ *rtmpmsg.NetStreamFCUnpublish) error {
	h.endSession()
	return nil
}

func (h *publishHandler) OnDeleteStream(_ uint32, _ *rtmpmsg.NetStreamDeleteStream) error {
	h.endSession()
	return nil
}

func (h *publishHandler) OnSetDataFrame(timestamp uint32, data *rtmpmsg.NetStreamSetDataFrame) error {
	h.mu.Lock()
	pipeline := h.pipeline
	publisher := h.publisher
	h.mu.Unlock()
	if publisher == nil || len(data.Payload) == 0 {
		return nil
	}
	if pipeline != nil && pipeline.Enabled() {
		if err := pipeline.PushSetDataFrame(timestamp, data); err != nil {
			return h.handlePipelineError(err)
		}
		return nil
	}
	if err := publisher.WriteSetDataFrame(timestamp, data); err != nil {
		_ = h.server.machine.MarkError("output write failed")
		return err
	}
	return nil
}

func (h *publishHandler) OnAudio(timestamp uint32, payload io.Reader) error {
	h.mu.Lock()
	pipeline := h.pipeline
	publisher := h.publisher
	h.mu.Unlock()
	if publisher == nil {
		return nil
	}

	var audio flvtag.AudioData
	if err := flvtag.DecodeAudioData(payload, &audio); err != nil {
		return fmt.Errorf("decode audio: %w", err)
	}

	flvBody := new(bytes.Buffer)
	if _, err := io.Copy(flvBody, audio.Data); err != nil {
		return fmt.Errorf("copy audio payload: %w", err)
	}
	audio.Data = flvBody

	if pipeline != nil && pipeline.Enabled() {
		if err := pipeline.PushAudio(timestamp, &audio); err != nil {
			return h.handlePipelineError(err)
		}
		return nil
	}

	if err := publisher.WriteAudioData(timestamp, &audio); err != nil {
		_ = h.server.machine.MarkError("output write failed")
		return err
	}
	return nil
}

func (h *publishHandler) OnVideo(timestamp uint32, payload io.Reader) error {
	h.mu.Lock()
	pipeline := h.pipeline
	publisher := h.publisher
	h.mu.Unlock()
	if publisher == nil {
		return nil
	}

	var video flvtag.VideoData
	if err := flvtag.DecodeVideoData(payload, &video); err != nil {
		return fmt.Errorf("decode video: %w", err)
	}

	flvBody := new(bytes.Buffer)
	if _, err := io.Copy(flvBody, video.Data); err != nil {
		return fmt.Errorf("copy video payload: %w", err)
	}
	video.Data = flvBody

	if pipeline != nil && pipeline.Enabled() {
		if err := pipeline.PushVideo(timestamp, &video); err != nil {
			return h.handlePipelineError(err)
		}
		return nil
	}

	if err := publisher.WriteVideoData(timestamp, &video); err != nil {
		_ = h.server.machine.MarkError("output write failed")
		return err
	}
	return nil
}

func (h *publishHandler) OnClose() {
	h.endSession()
}

func (h *publishHandler) forwardOnMetaData(payload []byte) {
	h.mu.Lock()
	pipeline := h.pipeline
	publisher := h.publisher
	h.mu.Unlock()
	if publisher == nil {
		return
	}
	if pipeline != nil && pipeline.Enabled() {
		if err := pipeline.PushOnMetaData(payload); err != nil {
			_ = h.handlePipelineError(err)
		}
		return
	}
	if err := publisher.WriteOnMetaData(0, payload); err != nil {
		_ = h.server.machine.MarkError("output write failed")
	}
}

func (h *publishHandler) handlePipelineError(err error) error {
	if err == nil {
		return nil
	}
	if err == buffer.ErrBufferOverflow || err == buffer.ErrFrameTooLarge {
		_ = h.server.machine.MarkError("buffer overflow")
		return err
	}
	_ = h.server.machine.MarkError("output write failed")
	return err
}

func (h *publishHandler) endSession() {
	h.mu.Lock()
	publisher := h.publisher
	pipeline := h.pipeline
	active := h.sessionActive
	h.publisher = nil
	h.pipeline = nil
	h.sessionActive = false
	h.mu.Unlock()

	if pipeline != nil {
		pipeline.Stop()
	}

	if publisher != nil {
		_ = publisher.Close()
	}

	if active {
		if err := h.server.machine.DisconnectSession(); err != nil {
			log.Printf("delaydeck-relay: session stop state transition: %v", err)
		}
	}

	h.server.mu.Lock()
	if h.server.activeHandler == h {
		h.server.activeHandler = nil
	}
	h.server.mu.Unlock()
}
