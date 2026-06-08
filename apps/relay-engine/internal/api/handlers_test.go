package api_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/delaydeck/delaydeck/apps/relay-engine/internal/api"
	"github.com/delaydeck/delaydeck/apps/relay-engine/internal/state"
)

const testToken = "test-session-token"

func newTestServer(t *testing.T) (*api.Server, *state.Machine) {
	t.Helper()
	machine := state.NewMachine(512*1024*1024, 0)
	server := api.NewServer(machine, testToken)

	must(t, machine.Start())
	must(t, machine.MarkReady())
	must(t, machine.MockConnectInput())
	must(t, machine.MockConnectOutput())

	return server, machine
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func authRequest(method, target, body string) *http.Request {
	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Authorization", "Bearer "+testToken)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func TestHealthEndpoint(t *testing.T) {
	server, _ := newTestServer(t)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload["healthy"] != true {
		t.Fatalf("expected healthy=true, got %v", payload["healthy"])
	}
	if payload["mode"] != "mock" {
		t.Fatalf("expected mode=mock, got %v", payload["mode"])
	}
}

func TestStatusRequiresAuth(t *testing.T) {
	server, _ := newTestServer(t)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/status", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestStatusEndpoint(t *testing.T) {
	server, _ := newTestServer(t)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, authRequest(http.MethodGet, "/v1/status", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if payload["state"] != "REALTIME" {
		t.Fatalf("expected REALTIME, got %v", payload["state"])
	}
}

func TestEnableDelayConflictFromReady(t *testing.T) {
	machine := state.NewMachine(512*1024*1024, 0)
	server := api.NewServer(machine, testToken)
	must(t, machine.Start())
	must(t, machine.MarkReady())

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, authRequest(http.MethodPost, "/v1/control/enable-delay", `{"target_delay_seconds":30}`))

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestEnableDelayAccepted(t *testing.T) {
	server, machine := newTestServer(t)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, authRequest(http.MethodPost, "/v1/control/enable-delay", `{"target_delay_seconds":30}`))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}

	waitForState(t, machine, state.Delayed, 2*time.Second)
}

func TestReturnLiveAndDumpBuffer(t *testing.T) {
	server, machine := newTestServer(t)

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, authRequest(http.MethodPost, "/v1/control/enable-delay", `{"target_delay_seconds":30}`))
	waitForState(t, machine, state.Delayed, 2*time.Second)

	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, authRequest(http.MethodPost, "/v1/control/return-live", ""))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("return-live expected 202, got %d", rec.Code)
	}
	waitForState(t, machine, state.Realtime, 2*time.Second)

	must(t, machine.EnableDelay(30))
	waitForState(t, machine, state.Delayed, 2*time.Second)

	rec = httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, authRequest(http.MethodPost, "/v1/control/dump-buffer", ""))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("dump-buffer expected 202, got %d", rec.Code)
	}
	waitForState(t, machine, state.Realtime, 2*time.Second)
}

func TestSafeHoldEndpoint(t *testing.T) {
	server, machine := newTestServer(t)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, authRequest(http.MethodPost, "/v1/control/safe-hold", ""))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	if machine.CurrentState() != state.SafeHold {
		t.Fatalf("expected SAFE_HOLD, got %s", machine.CurrentState())
	}
}

func TestWebSocketEvents(t *testing.T) {
	server, _ := newTestServer(t)

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	wsURL := "ws" + ts.URL[len("http"):] + "/v1/events?token=" + testToken
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	defer conn.Close()

	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, authRequest(http.MethodPost, "/v1/control/safe-hold", ""))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("safe-hold failed: %d", rec.Code)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var message map[string]any
	if err := conn.ReadJSON(&message); err != nil {
		t.Fatalf("read websocket event: %v", err)
	}
	if message["type"] != "state.changed" {
		t.Fatalf("expected state.changed event, got %v", message["type"])
	}
}

func waitForState(t *testing.T, machine *state.Machine, want state.State, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if machine.CurrentState() == want && !machine.Snapshot().TransitionPending {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s, current=%s", want, machine.CurrentState())
}
