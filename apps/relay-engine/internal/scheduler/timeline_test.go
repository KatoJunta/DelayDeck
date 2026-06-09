package scheduler_test

import (
	"testing"

	"github.com/KatoJunta/DelayDeck/apps/relay-engine/internal/scheduler"
)

func TestAdvanceTimelineContinuesFromLiveOutput(t *testing.T) {
	var tl scheduler.OutputTimeline

	liveOut := tl.Video(60_000)
	slateOut := tl.AdvanceVideo(1)

	if slateOut <= liveOut {
		t.Fatalf("expected slate timestamp after live (%d), got %d", liveOut, slateOut)
	}
}
