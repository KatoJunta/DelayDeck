package config

import (
	"flag"
	"fmt"
	"os"
	"time"
)

type Config struct {
	ListenAddress       string
	SessionToken        string
	MockAutoConnect     bool
	TransitionDelay     time.Duration
	BufferCapacityBytes int64
}

func ParseFlags(args []string) (*Config, error) {
	fs := flag.NewFlagSet("delaydeck-relay", flag.ContinueOnError)

	listen := fs.String("listen", "127.0.0.1:9400", "HTTP listen address")
	token := fs.String("token", "", "session token (auto-generated when empty)")
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

	return &Config{
		ListenAddress:       *listen,
		SessionToken:        sessionToken,
		MockAutoConnect:     *mockAutoConnect,
		TransitionDelay:     time.Duration(*transitionMS) * time.Millisecond,
		BufferCapacityBytes: *bufferCapacity,
	}, nil
}
