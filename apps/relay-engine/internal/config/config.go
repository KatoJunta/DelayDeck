package config

import (
	"flag"
	"fmt"
	"os"
	"time"
)

type Config struct {
	Mode                RunMode
	ListenAddress       string
	IngestListenAddress string
	SessionToken        string
	OutputURL           string
	OutputStreamKey     string
	MockAutoConnect     bool
	TransitionDelay     time.Duration
	BufferCapacityBytes int64
}

func ParseFlags(args []string) (*Config, error) {
	fs := flag.NewFlagSet("delaydeck-relay", flag.ContinueOnError)

	mode := fs.String("mode", string(RunModeMock), "relay mode: mock or forwarding")
	listen := fs.String("listen", "127.0.0.1:9400", "HTTP listen address")
	ingestListen := fs.String("ingest-listen", "127.0.0.1:9401", "ingest listen address")
	token := fs.String("token", "", "session token (auto-generated when empty)")
	outputURL := fs.String("output-url", "", "output RTMP server URL (forwarding mode)")
	outputStreamKey := fs.String("output-stream-key", "", "output stream key (forwarding mode)")
	mockAutoConnect := fs.Bool("mock-auto-connect", true, "auto-advance READY to REALTIME in mock mode")
	transitionMS := fs.Int("transition-ms", 50, "mock transition delay in milliseconds")
	bufferCapacity := fs.Int64("buffer-capacity-bytes", 512*1024*1024, "mock buffer capacity in bytes")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	sessionToken := *token
	if sessionToken == "" {
		sessionToken = os.Getenv("DELAYDECK_SESSION_TOKEN")
	}
	if sessionToken == "" {
		generated, err := generateToken()
		if err != nil {
			return nil, fmt.Errorf("generate session token: %w", err)
		}
		sessionToken = generated
	}

	if *transitionMS < 0 {
		return nil, fmt.Errorf("transition-ms must be >= 0")
	}
	if *bufferCapacity <= 0 {
		return nil, fmt.Errorf("buffer-capacity-bytes must be > 0")
	}

	runMode := RunMode(*mode)
	if !runMode.Valid() {
		return nil, fmt.Errorf("mode must be mock or forwarding")
	}

	resolvedOutputURL := *outputURL
	if resolvedOutputURL == "" {
		resolvedOutputURL = os.Getenv("DELAYDECK_OUTPUT_URL")
	}
	resolvedOutputKey := *outputStreamKey
	if resolvedOutputKey == "" {
		resolvedOutputKey = os.Getenv("DELAYDECK_OUTPUT_STREAM_KEY")
	}

	autoConnect := *mockAutoConnect
	if runMode == RunModeForwarding {
		autoConnect = false
		if resolvedOutputURL == "" {
			return nil, fmt.Errorf("forwarding mode requires --output-url or DELAYDECK_OUTPUT_URL")
		}
		if resolvedOutputKey == "" {
			return nil, fmt.Errorf("forwarding mode requires --output-stream-key or DELAYDECK_OUTPUT_STREAM_KEY")
		}
	}

	return &Config{
		Mode:                runMode,
		ListenAddress:       *listen,
		IngestListenAddress: *ingestListen,
		SessionToken:        sessionToken,
		OutputURL:           resolvedOutputURL,
		OutputStreamKey:     resolvedOutputKey,
		MockAutoConnect:     autoConnect,
		TransitionDelay:     time.Duration(*transitionMS) * time.Millisecond,
		BufferCapacityBytes: *bufferCapacity,
	}, nil
}
