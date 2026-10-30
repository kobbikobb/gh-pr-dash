package main

import (
	"os"
	"path/filepath"
	"strconv"
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

		if !strings.Contains(got, "next refresh in") {
			t.Errorf("got %q, want a countdown", got)
		}
	})
}

func press(m watchModel, keys ...string) watchModel {
	for _, k := range keys {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		}
		next, _ := m.Update(msg)
		m = next.(watchModel)
	}
	return m
}

func testModel() watchModel {
	m := newWatchModel(options{})
	m.rows = []Row{
		{Ref: "a#1", Title: "broken", Tier: tierNeedsAction, URL: "u1"},
		{Ref: "a#2", Title: "good", Tier: tierReady, URL: "u2"},
		{Ref: "a#3", Title: "also good", Tier: tierReady, URL: "u3"},
		{Ref: "a#4", Title: "waiting", Tier: tierWaiting, URL: "u4"},
	}
	return m.fixCursor()
}

func TestStaleFetchIsIgnored(t *testing.T) {
	m := testModel()
	m.gen = 3

	next, _ := m.Update(fetchMsg{gen: 2, rows: []Row{{Ref: "old#1"}}})

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

func TestMergeResultDiscardsPreMergeFetches(t *testing.T) {
	m := testModel()
	m.merging = true
	m.deferred = &fetchMsg{rows: []Row{{Ref: "stale#1", Tier: tierReady}}}

	next, cmd := m.Update(mergeMsg{merged: 1})
	m = next.(watchModel)

	if m.rows[0].Ref != "a#1" || m.deferred != nil || cmd == nil {
		t.Errorf("rows=%v deferred=%v cmd=%v: stale fetch applied or no refetch", m.rows, m.deferred, cmd)
	}
	next, _ = m.Update(fetchMsg{gen: 0, rows: []Row{{Ref: "stale#1"}}})
	if next.(watchModel).rows[0].Ref != "a#1" {
		t.Error("in-flight pre-merge fetch overwrote rows")
	}
}

func TestFooterSurvivesShortTerminal(t *testing.T) {
	rows := make([]Row, 30)
	for i := range rows {
		rows[i] = Row{Ref: "a#1", Title: "t", Tier: tierWaiting, URL: "u"}
	}

	out := renderWatch(options{}, 0, rows, 0, "", "x", "FOOTER-LINE", terminal{width: 100, height: 12}, view{})

	if !strings.Contains(out, "FOOTER-LINE") || strings.Count(out, "\n")+1 > 12 {
		t.Errorf("footer missing or frame taller than terminal:\n%s", out)
	}
}

func fakeBin(t *testing.T, name, script string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestMergeCmdReportsSuccessAndFailure(t *testing.T) {
	fakeBin(t, "gh", `case "$3" in *bad*) echo "not mergeable" >&2; exit 1;; esac`)
	rows := []Row{{Ref: "a#1", URL: "https://x/good"}, {Ref: "a#2", URL: "https://x/bad"}}

	res := mergeCmd(rows)().(mergeMsg)

	if res.merged != 1 || len(res.failed) != 1 || res.failed[0] != "a#2: not mergeable" {
		t.Errorf("got %+v", res)
	}
}

func TestDeliverToAgentReportsWhatHappened(t *testing.T) {
	t.Setenv("GH_PR_DASH_AGENT_CMD", "true")
	if how, err := deliverToAgent("msg"); err != nil || how != "started true" {
		t.Errorf("how=%q err=%v", how, err)
	}

	t.Setenv("GH_PR_DASH_AGENT_CMD", "/nonexistent/agent")
	if _, err := deliverToAgent("msg"); err == nil {
		t.Error("expected an error for a missing agent binary")
	}
}

func TestPasteNeverTriggersActions(t *testing.T) {
	m := testModel()
	m.confirm = []Row{m.rows[1]}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y"), Paste: true})

	if got := next.(watchModel); got.merging || got.confirm == nil || cmd != nil {
		t.Error("pasted 'y' confirmed a merge")
	}
}

func TestCursorMovesAndStaysInBounds(t *testing.T) {
	m := testModel()
	if m.cursor != "u1" {
		t.Fatalf("cursor=%q, want first row", m.cursor)
	}

	m = press(m, "down", "j", "down", "down", "down")
	if m.cursor != "u4" {
		t.Errorf("cursor=%q, want clamped to last row u4", m.cursor)
	}

	m = press(m, "up", "k", "k", "k", "k")
	if m.cursor != "u1" {
		t.Errorf("cursor=%q, want clamped to first row u1", m.cursor)
	}
}

func TestCursorFollowsPRAcrossRefresh(t *testing.T) {
	m := press(testModel(), "down", "down") // u3

	next, _ := m.Update(fetchMsg{rows: []Row{
		{Ref: "n#0", Tier: tierNeedsAction, URL: "u0"},
		{Ref: "a#3", Tier: tierReady, URL: "u3"},
	}})

	if got := next.(watchModel).cursor; got != "u3" {
		t.Errorf("cursor=%q, want to stay on u3 after rows shifted", got)
	}
}

func TestCursorReseatsWhenPRDisappears(t *testing.T) {
	m := press(testModel(), "down", "down", "down") // u4, last

	next, _ := m.Update(fetchMsg{rows: []Row{{Ref: "z#1", Tier: tierReady, URL: "z1"}}})

	if got := next.(watchModel).cursor; got != "z1" {
		t.Errorf("cursor=%q, want clamped onto the remaining row", got)
	}
}

func TestMergeSelectedOnlyWhenReady(t *testing.T) {
	m := press(testModel(), "m") // cursor on needs-action PR
	if m.confirm != nil || !strings.Contains(m.status, "can't merge") {
		t.Errorf("confirm=%v status=%q, want refusal", m.confirm, m.status)
	}

	m = press(m, "down", "m")
	if len(m.confirm) != 1 || m.confirm[0].Ref != "a#2" {
		t.Errorf("confirm=%v, want only a#2", m.confirm)
	}
}

func TestMergeCategoryTakesEveryReadyPR(t *testing.T) {
	m := press(testModel(), "down", "M")
	if len(m.confirm) != 2 {
		t.Fatalf("confirm=%v, want both ready PRs", m.confirm)
	}

	m = press(m, "x") // stray key must not cancel or confirm
	if m.confirm == nil || m.merging {
		t.Fatal("stray key changed the confirmation")
	}

	m = press(m, "n")
	if m.confirm != nil || m.merging {
		t.Errorf("cancel left confirm=%v merging=%v", m.confirm, m.merging)
	}
}

func TestAgentCategoryUsesItsHeader(t *testing.T) {
	t.Setenv("GH_PR_DASH_AGENT_CMD", "true")

	m := press(testModel(), "down", "A")

	if !strings.Contains(m.status, "(2 PR(s))") {
		t.Errorf("status=%q, want both ready PRs sent", m.status)
	}
	got := agentMessage(categories[tierReady].agent, []Row{{Ref: "a#2", Title: "good", URL: "u"}})
	if !strings.Contains(got, "merge ready PRs") || !strings.Contains(got, "- [a#2] good - u") {
		t.Errorf("got %q", got)
	}
}

func TestNumberKeyJumpsAndUnfolds(t *testing.T) {
	m := testModel()

	m = press(m, "enter") // fold Needs action; cursor must leave it
	if m.cursor != "u2" || !m.collapsed[tierNeedsAction] {
		t.Fatalf("cursor=%q collapsed=%v, want u2 and Needs action folded", m.cursor, m.collapsed)
	}

	m = press(m, "1")
	if m.cursor != "u1" || m.collapsed[tierNeedsAction] {
		t.Errorf("cursor=%q collapsed=%v, want jumped to u1 and unfolded", m.cursor, m.collapsed)
	}

	m = press(m, "4") // Needs reviewer is empty
	if m.cursor != "u1" || !strings.Contains(m.status, "no PRs") {
		t.Errorf("cursor=%q status=%q, want unchanged with a message", m.cursor, m.status)
	}
}

func TestFoldingNeverStrandsTheCursor(t *testing.T) {
	m := testModel()
	m.rows = m.rows[:1] // only one category

	m = press(m, "enter")

	if m.collapsed[tierNeedsAction] || m.cursor != "u1" {
		t.Errorf("folded the only category: collapsed=%v cursor=%q", m.collapsed, m.cursor)
	}
}

func TestHelpTogglesAndEscClears(t *testing.T) {
	m := press(testModel(), "?")
	if !m.help || !strings.Contains(m.footer(), "fold / unfold") {
		t.Fatalf("help=%v footer=%q", m.help, m.footer())
	}

	m = press(m, "esc")
	if m.help {
		t.Error("esc left help open")
	}
}

func TestCursorRowIsMarkedAndStaysVisible(t *testing.T) {
	rows := make([]Row, 30)
	for i := range rows {
		rows[i] = Row{Ref: "a#1", Title: "t", Tier: tierWaiting, URL: "u" + strconv.Itoa(i)}
	}

	out := renderWatch(options{}, 0, rows, 0, "", "x", "FOOT", terminal{width: 100, height: 12}, view{cursor: "u25", interactive: true})

	if !strings.Contains(out, "❯") {
		t.Errorf("cursor row scrolled off a short terminal:\n%s", out)
	}
}

func TestScopeLabel(t *testing.T) {
	cases := []struct {
		opts options
		want string
	}{
		{options{}, "all orgs"},
		{options{org: "acme"}, "acme"},
		{options{org: "acme", repo: "api"}, "acme/api"},
		{options{repo: "api"}, "api"},
	}
	for _, c := range cases {
		if got := scopeLabel(c.opts); got != c.want {
			t.Errorf("scopeLabel(%+v) = %q, want %q", c.opts, got, c.want)
		}
	}
}
