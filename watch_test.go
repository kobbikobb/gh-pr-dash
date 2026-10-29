package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

func press(m watchModel, keys ...string) watchModel {
	for _, k := range keys {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "backspace":
			msg = tea.KeyMsg{Type: tea.KeyBackspace}
		}
		next, _ := m.Update(msg)
		m = next.(watchModel)
	}
	return m
}

func testModel() watchModel {
	m := newWatchModel(options{})
	m.rows = []Row{
		{Ref: "a#1", Title: "broken", Tier: tierNeedsAction},
		{Ref: "a#2", Title: "good", Tier: tierReady},
		{Ref: "a#3", Title: "also good", Tier: tierReady},
		{Ref: "a#4", Title: "waiting", Tier: tierWaiting},
	}
	return m
}

func TestBareDigitsDoNothing(t *testing.T) {
	m := press(testModel(), "1", "2")

	if m.action != 0 || m.input != "" || m.status != "" {
		t.Errorf("bare digits changed state: %+v", m)
	}
}

func TestMergeOnlyTargetsReadyPRs(t *testing.T) {
	m := press(testModel(), "m", "1", ",", "2", "enter")

	if len(m.confirm) != 1 || m.confirm[0].Ref != "a#2" || m.skipped != 1 {
		t.Errorf("confirm=%v skipped=%d, want only a#2 with 1 skipped", m.confirm, m.skipped)
	}
}

func TestMergeByGroupAndCancel(t *testing.T) {
	m := press(testModel(), "m", "r")
	if len(m.confirm) != 2 {
		t.Fatalf("confirm=%v, want both ready PRs", m.confirm)
	}

	m = press(m, "x") // stray key must not cancel or confirm
	if m.confirm == nil {
		t.Fatal("stray key dismissed the confirmation")
	}

	m = press(m, "n")
	if m.confirm != nil || m.merging {
		t.Errorf("cancel left confirm=%v merging=%v", m.confirm, m.merging)
	}
}

func TestMergeNothingReady(t *testing.T) {
	m := press(testModel(), "m", "n")

	if m.confirm != nil || !strings.Contains(m.status, "nothing to merge") {
		t.Errorf("confirm=%v status=%q", m.confirm, m.status)
	}
}

func TestEscAndBackspaceEditTarget(t *testing.T) {
	m := press(testModel(), "o", "1", "2", "backspace")
	if m.input != "1" {
		t.Errorf("input=%q, want 1", m.input)
	}

	m = press(m, "esc")
	if m.action != 0 || m.input != "" {
		t.Errorf("esc left action=%q input=%q", m.action, m.input)
	}
}

func TestSlackIsNotImplemented(t *testing.T) {
	m := press(testModel(), "s")

	if m.action != 0 || m.status != "slack: not implemented" {
		t.Errorf("action=%q status=%q", m.action, m.status)
	}
}

func TestFetchResultHeldWhileActionPending(t *testing.T) {
	m := press(testModel(), "o", "1")
	fresh := []Row{{Ref: "z#9", Tier: tierReady}}

	next, _ := m.Update(fetchMsg{seq: 5, rows: fresh})
	m = next.(watchModel)
	if m.rows[0].Ref != "a#1" {
		t.Fatalf("rows changed mid-action: %v", m.rows)
	}

	m = press(m, "esc")
	if m.rows[0].Ref != "z#9" {
		t.Errorf("deferred fetch not applied after action ended: %v", m.rows)
	}
}

func TestStaleFetchIsIgnored(t *testing.T) {
	m := testModel()
	m.appliedSeq = 3

	next, _ := m.Update(fetchMsg{seq: 2, rows: []Row{{Ref: "old#1"}}})

	if next.(watchModel).rows[0].Ref != "a#1" {
		t.Error("older fetch overwrote newer data")
	}
}

func TestMergeStatusReportsFailures(t *testing.T) {
	got := mergeStatus(mergeMsg{merged: 1, failed: []string{"a#2: not mergeable"}})

	if !strings.Contains(got, "merged 1, 1 failed") || !strings.Contains(got, "a#2: not mergeable") {
		t.Errorf("got %q", got)
	}
}

func TestAgentMessageListsTargets(t *testing.T) {
	got := agentMessage(agentHeader("r"), []Row{{Ref: "a#2", Title: "good", URL: "u"}})

	if !strings.Contains(got, "merge ready PRs") || !strings.Contains(got, "- [a#2] good - u") {
		t.Errorf("got %q", got)
	}
}
