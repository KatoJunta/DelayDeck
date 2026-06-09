package ingest

import (
	"net"
	"testing"
	"time"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/output"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/state"
)

func TestStartRTMPServerAcceptsTCPConnection(t *testing.T) {
	machine := state.NewMachine(512*1024*1024, 0)
	dest, err := output.ParseDestination("rtmp://127.0.0.1/live", "test-key")
	if err != nil {
		t.Fatalf("parse destination: %v", err)
	}

	server, err := StartRTMPServer("127.0.0.1:0", dest, machine)
	if err != nil {
		t.Fatalf("start RTMP server: %v", err)
	}
	defer server.Close()

	conn, err := net.DialTimeout("tcp", server.Address(), 2*time.Second)
	if err != nil {
		t.Fatalf("dial RTMP ingest: %v", err)
	}
	conn.Close()
}
