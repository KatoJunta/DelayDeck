package ingest

import (
	"net"
	"testing"
	"time"
)

func TestStartMockListenerAcceptsTCPConnection(t *testing.T) {
	listener, err := StartMockListener("127.0.0.1:0")
	if err != nil {
		t.Fatalf("start mock listener: %v", err)
	}
	defer listener.Close()

	conn, err := net.DialTimeout("tcp", listener.Address(), 2*time.Second)
	if err != nil {
		t.Fatalf("dial mock ingest: %v", err)
	}
	conn.Close()
}
