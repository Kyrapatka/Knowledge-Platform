package model

import (
	"testing"
	"time"
)

func TestEarlierReview(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct{ delay, want time.Duration }{
		{24 * time.Hour, 23*time.Hour + 30*time.Minute},
		{8 * time.Hour, 7*time.Hour + 30*time.Minute},
		{10 * time.Minute, 0}, {0, 0}, {-time.Hour, -time.Hour},
	} {
		got := EarlierReview(now, now.Add(tc.delay))
		if !got.Equal(now.Add(tc.want)) {
			t.Fatalf("delay=%s: got %s", tc.delay, got)
		}
	}
}
