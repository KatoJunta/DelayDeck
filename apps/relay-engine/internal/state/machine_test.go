package state_test

import (
	"testing"
	"time"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/state"
)

func newTestMachine() *state.Machine {
	return state.NewMachine(512*1024*1024, 0)
}

func bootToRealtime(t *testing.T, m *state.Machine) {
	t.Helper()
	mustApply(t, m.Start())
	mustApply(t, m.MarkReady())
	mustApply(t, m.MockConnectInput())
	mustApply(t, m.MockConnectOutput())
	if got := m.CurrentState(); got != state.Realtime {
		t.Fatalf("expected REALTIME, got %s", got)
	}
}

func mustApply(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartupTransitions(t *testing.T) {
	m := newTestMachine()
	if m.CurrentState() != state.Stopped {
		t.Fatalf("expected STOPPED, got %s", m.CurrentState())
	}

	mustApply(t, m.Start())
	if m.CurrentState() != state.Starting {
		t.Fatalf("expected STARTING, got %s", m.CurrentState())
	}

	mustApply(t, m.MarkReady())
	if m.CurrentState() != state.Ready {
		t.Fatalf("expected READY, got %s", m.CurrentState())
	}
}

func TestMockConnectionTransitions(t *testing.T) {
	m := newTestMachine()
	mustApply(t, m.Start())
	mustApply(t, m.MarkReady())

	mustApply(t, m.MockConnectInput())
	if m.CurrentState() != state.Ingesting {
		t.Fatalf("expected INGESTING, got %s", m.CurrentState())
	}

	mustApply(t, m.MockConnectOutput())
	if m.CurrentState() != state.Realtime {
		t.Fatalf("expected REALTIME, got %s", m.CurrentState())
	}
}

func TestEnableDelayInvalidFromReady(t *testing.T) {
	m := newTestMachine()
	mustApply(t, m.Start())
	mustApply(t, m.MarkReady())

	err := m.EnableDelay(30)
	if err == nil {
		t.Fatal("expected error enabling delay from READY")
	}
	var transition *state.TransitionError
	if !errorsAsTransition(t, err, &transition) {
		return
	}
	if transition.From != state.Ready {
		t.Fatalf("expected from READY, got %s", transition.From)
	}
}

func TestEnableDelaySequence(t *testing.T) {
	m := newTestMachine()
	bootToRealtime(t, m)

	err := m.EnableDelay(30)
	if err != nil {
		t.Fatalf("EnableDelay: %v", err)
	}

	waitForState(t, m, state.Delayed, 2*time.Second)

	snapshot := m.Snapshot()
	if snapshot.TargetDelaySeconds != 30 {
		t.Fatalf("expected target delay 30, got %d", snapshot.TargetDelaySeconds)
	}
	if snapshot.ActiveDelaySeconds != 30 {
		t.Fatalf("expected active delay 30, got %d", snapshot.ActiveDelaySeconds)
	}
}

func TestReturnLiveSequence(t *testing.T) {
	m := newTestMachine()
	bootToRealtime(t, m)
	mustApply(t, m.EnableDelay(30))
	waitForState(t, m, state.Delayed, 2*time.Second)

	err := m.ReturnLive()
	if err != nil {
		t.Fatalf("ReturnLive: %v", err)
	}

	waitForState(t, m, state.Realtime, 5*time.Second)

	snapshot := m.Snapshot()
	if snapshot.BufferUsagePercent != 0 {
		t.Fatalf("expected empty buffer after return-live drain, got %.2f%%",
			snapshot.BufferUsagePercent)
	}
	if snapshot.ActiveDelaySeconds != 0 {
		t.Fatalf("expected active delay 0 after return-live, got %d",
			snapshot.ActiveDelaySeconds)
	}
}

func TestEnableDelaySetsSlateDuringTransition(t *testing.T) {
	m := state.NewMachine(512*1024*1024, 20*time.Millisecond)
	bootToRealtime(t, m)

	if err := m.EnableDelay(30); err != nil {
		t.Fatalf("EnableDelay: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	sawSlate := false
	for time.Now().Before(deadline) {
		snapshot := m.Snapshot()
		if snapshot.SlateMessage != "" && snapshot.CountdownSeconds > 0 {
			sawSlate = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !sawSlate {
		t.Fatal("expected slate_message and countdown during enable-delay")
	}

	waitForState(t, m, state.Delayed, 5*time.Second)
	if m.Snapshot().SlateMessage != "" {
		t.Fatalf("expected slate cleared after DELAYED, got %q", m.Snapshot().SlateMessage)
	}
}

func TestDumpBufferSequence(t *testing.T) {
	m := newTestMachine()
	bootToRealtime(t, m)
	mustApply(t, m.EnableDelay(30))
	waitForState(t, m, state.Delayed, 2*time.Second)

	err := m.DumpBuffer()
	if err != nil {
		t.Fatalf("DumpBuffer: %v", err)
	}

	waitForState(t, m, state.Realtime, 2*time.Second)

	snapshot := m.Snapshot()
	if snapshot.BufferUsagePercent != 0 {
		t.Fatalf("expected empty buffer after dump, got %.2f%%", snapshot.BufferUsagePercent)
	}
}

func TestSafeHoldFromRealtime(t *testing.T) {
	m := newTestMachine()
	bootToRealtime(t, m)

	mustApply(t, m.SafeHold())
	if m.CurrentState() != state.SafeHold {
		t.Fatalf("expected SAFE_HOLD, got %s", m.CurrentState())
	}

	mustApply(t, m.ResumeLive())
	if m.CurrentState() != state.Realtime {
		t.Fatalf("expected REALTIME, got %s", m.CurrentState())
	}
}

func TestSafeHoldFromDelayed(t *testing.T) {
	m := newTestMachine()
	bootToRealtime(t, m)
	mustApply(t, m.EnableDelay(30))
	waitForState(t, m, state.Delayed, 2*time.Second)

	mustApply(t, m.SafeHold())
	if m.CurrentState() != state.SafeHold {
		t.Fatalf("expected SAFE_HOLD, got %s", m.CurrentState())
	}

	mustApply(t, m.ResumeDelayed())
	if m.CurrentState() != state.Delayed {
		t.Fatalf("expected DELAYED, got %s", m.CurrentState())
	}
}

func TestTransitionPendingRejectsConcurrentControl(t *testing.T) {
	m := state.NewMachine(512*1024*1024, 200*time.Millisecond)
	bootToRealtime(t, m)

	mustApply(t, m.EnableDelay(30))
	if m.CurrentState() != state.BufferingToDelay {
		t.Fatalf("expected BUFFERING_TO_DELAY, got %s", m.CurrentState())
	}

	err := m.ReturnLive()
	if err == nil {
		t.Fatal("expected transition pending error")
	}
	if err != state.ErrTransitionPending {
		t.Fatalf("expected ErrTransitionPending, got %v", err)
	}
}

func TestChangeEventsEmitted(t *testing.T) {
	m := newTestMachine()
	events := make(chan state.ChangeEvent, 4)
	m.OnChange(func(event state.ChangeEvent) {
		events <- event
	})

	mustApply(t, m.Start())

	select {
	case event := <-events:
		if event.CurrentState != state.Starting {
			t.Fatalf("expected STARTING event, got %s", event.CurrentState)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for change event")
	}
}

func waitForState(t *testing.T, m *state.Machine, want state.State, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m.CurrentState() == want && !m.Snapshot().TransitionPending {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for state %s, current=%s pending=%v", want, m.CurrentState(), m.Snapshot().TransitionPending)
}

func errorsAsTransition(t *testing.T, err error, target **state.TransitionError) bool {
	t.Helper()
	var transition *state.TransitionError
	if !errorsAs(err, &transition) {
		t.Fatalf("expected TransitionError, got %v", err)
		return false
	}
	*target = transition
	return true
}

func errorsAs(err error, target **state.TransitionError) bool {
	transition, ok := err.(*state.TransitionError)
	if !ok {
		return false
	}
	*target = transition
	return true
}

func TestDisconnectSessionFromRealtime(t *testing.T) {
	m := newTestMachine()
	bootToRealtime(t, m)

	mustApply(t, m.DisconnectSession())
	if m.CurrentState() != state.Ready {
		t.Fatalf("expected READY, got %s", m.CurrentState())
	}

	snap := m.Snapshot()
	if snap.InputConnected || snap.OutputConnected {
		t.Fatalf("expected disconnected flags, got input=%v output=%v",
			snap.InputConnected, snap.OutputConnected)
	}
}

func TestReconnectAfterDisconnectSession(t *testing.T) {
	m := newTestMachine()
	bootToRealtime(t, m)
	mustApply(t, m.DisconnectSession())

	mustApply(t, m.ConnectInput())
	mustApply(t, m.ConnectOutput())
	if m.CurrentState() != state.Realtime {
		t.Fatalf("expected REALTIME after reconnect, got %s", m.CurrentState())
	}
}
