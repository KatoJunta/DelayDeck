package ingest

import (
	"testing"
	"time"
)

func TestPublishCountdownPublishesEveryTick(t *testing.T) {
	var published []int
	publish := func(_ string, countdown int) {
		published = append(published, countdown)
	}

	publishCountdown(publish, "remaining %d", 10)
	publishCountdown(publish, "remaining %d", 9)
	publishCountdown(publish, "remaining %d", 8)
	publishCountdown(publish, "remaining %d", 0)

	want := []int{10, 9, 8, 1}
	if len(published) != len(want) {
		t.Fatalf("published = %v, want %v", published, want)
	}
	for i := range want {
		if published[i] != want[i] {
			t.Fatalf("published[%d] = %d, want %d", i, published[i], want[i])
		}
	}
}

func TestCountdownSecondsLeftRoundsUpPartialSeconds(t *testing.T) {
	tests := []struct {
		name      string
		remaining time.Duration
		want      int
	}{
		{name: "zero", remaining: 0, want: 0},
		{name: "partial second", remaining: 1500 * time.Millisecond, want: 2},
		{name: "exact second", remaining: 2 * time.Second, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := countdownSecondsLeft(tt.remaining); got != tt.want {
				t.Fatalf("countdownSecondsLeft(%s) = %d, want %d", tt.remaining, got, tt.want)
			}
		})
	}
}
