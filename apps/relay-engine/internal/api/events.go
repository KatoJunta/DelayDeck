package api

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/state"
)

type EventHub struct {
	mu          sync.RWMutex
	subscribers map[chan state.ChangeEvent]struct{}
}

func NewEventHub() *EventHub {
	return &EventHub{
		subscribers: make(map[chan state.ChangeEvent]struct{}),
	}
}

func (h *EventHub) Publish(event state.ChangeEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for ch := range h.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (h *EventHub) Subscribe() (chan state.ChangeEvent, func()) {
	ch := make(chan state.ChangeEvent, 16)

	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()

	unsubscribe := func() {
		h.mu.Lock()
		delete(h.subscribers, ch)
		close(ch)
		h.mu.Unlock()
	}

	return ch, unsubscribe
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if !s.authorize(r) {
		writeError(w, http.StatusUnauthorized, "authentication_failed", "missing or invalid session token")
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	events, unsubscribe := s.eventHub.Subscribe()
	defer unsubscribe()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			if err := conn.WriteJSON(map[string]any{
				"type":    "state.changed",
				"payload": event,
			}); err != nil {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}
