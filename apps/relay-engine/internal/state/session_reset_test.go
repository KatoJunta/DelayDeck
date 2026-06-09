package state_test

import (
	"testing"
	"time"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/state"
)

func TestDisconnectSessionFromDelayed(t *testing.T) {
	m := state.NewMachine(512*1024*1024, 0)
	bootToRealtime(t, m)
	mustApply(t, m.EnableDelay(30))
	waitForState(t, m, state.Delayed, 3*time.Second)

	mustApply(t, m.DisconnectSession())
	if m.CurrentState() != state.Ready {
		t.Fatalf("expected READY, got %s", m.CurrentState())
	}
	if m.Snapshot().TransitionPending {
		t.Fatal("expected transition pending cleared")
	}
}

func TestDisconnectSessionFromError(t *testing.T) {
	m := state.NewMachine(512*1024*1024, 0)
	bootToRealtime(t, m)
	mustApply(t, m.MarkError("output write failed"))

	mustApply(t, m.DisconnectSession())
	if m.CurrentState() != state.Ready {
		t.Fatalf("expected READY, got %s", m.CurrentState())
	}
}

func TestInterruptedEnableDelayDoesNotAdvanceState(t *testing.T) {
	m := state.NewMachine(512*1024*1024, 0)
	bootToRealtime(t, m)

	if err := m.EnableDelay(30); err != nil {
		t.Fatalf("EnableDelay: %v", err)
	}

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if m.CurrentState() == state.SafeHold {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	mustApply(t, m.DisconnectSession())
	if m.CurrentState() != state.Ready {
		t.Fatalf("expected READY after interrupt, got %s", m.CurrentState())
	}

	time.Sleep(300 * time.Millisecond)
	if m.CurrentState() != state.Ready {
		t.Fatalf("stale transition moved state to %s", m.CurrentState())
	}
}
