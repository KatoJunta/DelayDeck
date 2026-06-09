package ingest_test

import (
	"testing"
	"time"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/ingest"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/scheduler"
	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/state"
)

func TestForwardingCoordinatorRequiresActiveSession(t *testing.T) {
	coordinator := ingest.NewForwardingCoordinator()

	err := coordinator.BeginEnableDelay(30)
	if err == nil {
		t.Fatal("expected error without active pipeline")
	}
}

func TestStaleCompleteEnableDelayIgnoredForNewSession(t *testing.T) {
	machine := state.NewMachine(512*1024*1024, 0)
	coordinator := ingest.NewForwardingCoordinator()
	machine.SetForwardingCoordinator(coordinator)

	mustApply(t, machine.Start())
	mustApply(t, machine.MarkReady())
	mustApply(t, machine.ConnectInput())
	mustApply(t, machine.ConnectOutput())

	oldPipeline := ingest.NewPipelineForTest(machine, 512*1024*1024)
	coordinator.SetPipeline(oldPipeline)

	if err := machine.EnableDelay(30); err != nil {
		t.Fatalf("EnableDelay: %v", err)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if machine.CurrentState() == state.SafeHold {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	oldPipeline.Stop()
	coordinator.ClearPipeline()
	mustApply(t, machine.DisconnectSession())

	newPipeline := ingest.NewPipelineForTest(machine, 512*1024*1024)
	coordinator.SetPipeline(newPipeline)

	coordinator.CompleteEnableDelay()

	if got := newPipeline.OutputPolicy(); got != scheduler.OutputPassthrough {
		t.Fatalf("expected passthrough on new session, got %v", got)
	}
}

func mustApply(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
