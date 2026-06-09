package ingest

import (
	"fmt"
	"net"
	"sync"
)

type MockListener struct {
	mu   sync.Mutex
	ln   net.Listener
	addr string
}

func StartMockListener(listenAddress string) (*MockListener, error) {
	ln, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", listenAddress, err)
	}

	return &MockListener{
		ln:   ln,
		addr: ln.Addr().String(),
	}, nil
}

func (m *MockListener) Address() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.addr
}

func (m *MockListener) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ln == nil {
		return nil
	}
	err := m.ln.Close()
	m.ln = nil
	return err
}
