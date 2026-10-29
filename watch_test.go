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

func TestParseTargets(t *testing.T) {
	rows := []Row{{Ref: "a#1"}, {Ref: "a#2"}, {Ref: "a#3"}}

	got := parseTargets("3, 1 3 9 x", rows)

	if len(got) != 2 || got[0].Ref != "a#3" || got[1].Ref != "a#1" {
		t.Errorf("got %v, want a#3 then a#1 (deduped, out-of-range ignored)", got)
	}
}
