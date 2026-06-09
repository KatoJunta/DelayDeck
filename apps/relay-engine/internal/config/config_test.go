package config

import "testing"

func TestParseFlagsRequiresOutput(t *testing.T) {
	_, err := ParseFlags([]string{"--token", "test-token"})
	if err == nil {
		t.Fatal("expected error when output destination is missing")
	}
}

func TestParseFlagsLoadsOutputFromArgs(t *testing.T) {
	cfg, err := ParseFlags([]string{
		"--token", "test-token",
		"--output-url", "rtmp://127.0.0.1/live",
		"--output-stream-key", "test-key",
	})
	if err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if cfg.Mode != RunModeForwarding {
		t.Fatalf("mode = %q", cfg.Mode)
	}
	if cfg.OutputURL != "rtmp://127.0.0.1/live" {
		t.Fatalf("output URL = %q", cfg.OutputURL)
	}
	if cfg.OutputStreamKey != "test-key" {
		t.Fatalf("output stream key = %q", cfg.OutputStreamKey)
	}
}

func TestParseFlagsLoadsFixedDelayFromEnv(t *testing.T) {
	t.Setenv("DELAYDECK_FIXED_DELAY_SECONDS", "30")

	cfg, err := ParseFlags([]string{
		"--token", "test-token",
		"--output-url", "rtmp://127.0.0.1/live",
		"--output-stream-key", "test-key",
	})
	if err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if cfg.FixedDelaySeconds != 30 {
		t.Fatalf("fixed delay = %d", cfg.FixedDelaySeconds)
	}
}
