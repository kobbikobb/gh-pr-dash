package main

import (
	"strings"
	"testing"
	"time"
)

func TestRefreshStatus(t *testing.T) {
	t.Run("should be empty before the first successful fetch", func(t *testing.T) {
		got := refreshStatus(time.Time{}, time.Time{})

		if got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("should show last fetch time and a countdown to the next", func(t *testing.T) {
		now := time.Now()

		got := refreshStatus(now, now.Add(30*time.Second))

		if !strings.Contains(got, "refreshed") || !strings.Contains(got, "next") {
			t.Errorf("got %q, want refreshed/next", got)
		}
	})
}
