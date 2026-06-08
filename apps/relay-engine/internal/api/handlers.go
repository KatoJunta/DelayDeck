package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/state"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/version"
)

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type HealthResponse struct {
	Healthy bool   `json:"healthy"`
	Version string `json:"version"`
	Mode    string `json:"mode"`
	Uptime  string `json:"uptime_seconds"`
}

type ControlRequest struct {
	TargetDelaySeconds int `json:"target_delay_seconds"`
}

type ControlResponse struct {
	Accepted bool                 `json:"accepted"`
	Status   state.StatusSnapshot `json:"status"`
}

type Server struct {
	machine      *state.Machine
	sessionToken string
	startedAt    time.Time
	eventHub     *EventHub
}

func NewServer(machine *state.Machine, sessionToken string) *Server {
	hub := NewEventHub()
	machine.OnChange(hub.Publish)

	return &Server{
		machine:      machine,
		sessionToken: sessionToken,
		startedAt:    time.Now().UTC(),
		eventHub:     hub,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/health", s.handleHealth)
	mux.HandleFunc("GET /v1/status", s.withAuth(s.handleStatus))
	mux.HandleFunc("POST /v1/control/enable-delay", s.withAuth(s.handleEnableDelay))
	mux.HandleFunc("POST /v1/control/return-live", s.withAuth(s.handleReturnLive))
	mux.HandleFunc("POST /v1/control/dump-buffer", s.withAuth(s.handleDumpBuffer))
	mux.HandleFunc("POST /v1/control/safe-hold", s.withAuth(s.handleSafeHold))
	mux.HandleFunc("GET /v1/events", s.handleEvents)
	return mux
}

func (s *Server) EventHub() *EventHub {
	return s.eventHub
}

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authorize(r) {
			writeError(w, http.StatusUnauthorized, "authentication_failed", "missing or invalid session token")
			return
		}
		next(w, r)
	}
}

func (s *Server) authorize(r *http.Request) bool {
	token := extractToken(r)
	return token != "" && token == s.sessionToken
}

func extractToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	}
	return r.URL.Query().Get("token")
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	current := s.machine.CurrentState()
	healthy := current != state.Error && current != state.Stopped

	writeJSON(w, http.StatusOK, HealthResponse{
		Healthy: healthy,
		Version: version.Version,
		Mode:    "mock",
		Uptime:  formatUptime(time.Since(s.startedAt)),
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.machine.Snapshot())
}

func (s *Server) handleEnableDelay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "POST required")
		return
	}

	var req ControlRequest
	if r.Body != nil {
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid_request", "request body must be JSON")
			return
		}
	}

	if req.TargetDelaySeconds <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_target_delay", "target_delay_seconds must be greater than zero")
		return
	}

	if err := s.machine.EnableDelay(req.TargetDelaySeconds); err != nil {
		writeControlError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, ControlResponse{
		Accepted: true,
		Status:   s.machine.Snapshot(),
	})
}

func (s *Server) handleReturnLive(w http.ResponseWriter, r *http.Request) {
	if err := s.machine.ReturnLive(); err != nil {
		writeControlError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, ControlResponse{
		Accepted: true,
		Status:   s.machine.Snapshot(),
	})
}

func (s *Server) handleDumpBuffer(w http.ResponseWriter, r *http.Request) {
	if err := s.machine.DumpBuffer(); err != nil {
		writeControlError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, ControlResponse{
		Accepted: true,
		Status:   s.machine.Snapshot(),
	})
}

func (s *Server) handleSafeHold(w http.ResponseWriter, r *http.Request) {
	if err := s.machine.SafeHold(); err != nil {
		writeControlError(w, err)
		return
	}

	writeJSON(w, http.StatusAccepted, ControlResponse{
		Accepted: true,
		Status:   s.machine.Snapshot(),
	})
}

func writeControlError(w http.ResponseWriter, err error) {
	var transition *state.TransitionError
	switch {
	case errors.As(err, &transition):
		writeError(w, http.StatusConflict, "invalid_state_transition",
			"cannot "+transition.Trigger+" while in state "+string(transition.From))
	case errors.Is(err, state.ErrTransitionPending):
		writeError(w, http.StatusConflict, "transition_in_progress", "another transition is already in progress")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "control request failed")
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{
		Error: ErrorBody{
			Code:    code,
			Message: message,
		},
	})
}

func formatUptime(d time.Duration) string {
	seconds := int(d.Seconds())
	if seconds < 0 {
		seconds = 0
	}
	return itoa(seconds)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	buf := make([]byte, 0, 12)
	for v > 0 {
		buf = append(buf, byte('0'+v%10))
		v /= 10
	}
	if neg {
		buf = append(buf, '-')
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}
