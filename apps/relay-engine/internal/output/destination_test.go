package output

import "testing"

func TestParseDestination(t *testing.T) {
	dest, err := ParseDestination("rtmp://a.rtmp.youtube.com/live2", "test-key")
	if err != nil {
		t.Fatalf("parse destination: %v", err)
	}
	if dest.HostPort != "a.rtmp.youtube.com:1935" {
		t.Fatalf("hostPort = %q", dest.HostPort)
	}
	if dest.App != "live2" {
		t.Fatalf("app = %q", dest.App)
	}
	if dest.StreamKey != "test-key" {
		t.Fatalf("streamKey = %q", dest.StreamKey)
	}
	if dest.TCURL != "rtmp://a.rtmp.youtube.com/live2" {
		t.Fatalf("tcURL = %q", dest.TCURL)
	}
}

func TestParseDestinationRequiresAppPath(t *testing.T) {
	_, err := ParseDestination("rtmp://127.0.0.1:1935", "key")
	if err == nil {
		t.Fatal("expected error for missing app path")
	}
}

func TestParseDestinationRejectsEmptyKey(t *testing.T) {
	_, err := ParseDestination("rtmp://127.0.0.1/live", "")
	if err == nil {
		t.Fatal("expected error for empty stream key")
	}
}
