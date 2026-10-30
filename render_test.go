package main

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"
)

func TestTruncate(t *testing.T) {
	t.Run("should not split a multibyte rune", func(t *testing.T) {
		got := truncate("goAML — mappers", 10)

		if !strings.HasSuffix(got, "…") {
			t.Errorf("got %q, want trailing ellipsis", got)
		}
		if strings.ContainsRune(got, '�') {
			t.Errorf("got replacement char in %q", got)
		}
	})

	t.Run("should not exceed the width for double-width characters", func(t *testing.T) {
		if got := truncate("日本語日本語", 7); runewidth.StringWidth(got) > 7 {
			t.Errorf("got %q (%d cells), want at most 7", got, runewidth.StringWidth(got))
		}
	})

	t.Run("should leave short strings unchanged", func(t *testing.T) {
		if got := truncate("short", 20); got != "short" {
			t.Errorf("got %q", got)
		}
	})
}

func TestPad(t *testing.T) {
	t.Run("should pad by rune count not byte count", func(t *testing.T) {
		if got := padRight("·", 4); len([]rune(got)) != 4 {
			t.Errorf("padRight rune len = %d, want 4", len([]rune(got)))
		}
	})

	t.Run("should right-align with padLeft", func(t *testing.T) {
		if got := padLeft("1d", 4); got != "  1d" {
			t.Errorf("got %q, want '  1d'", got)
		}
	})
}

func TestRenderTerminal(t *testing.T) {
	rows := []Row{
		{Tier: tierNeedsAction, IdleDays: 13, CI: "fail", Merge: "clean", Review: "none", Ref: "glint#20", URL: "https://github.com/me/glint/pull/20"},
		{Tier: tierReady, IdleDays: 2, CI: "ok", Merge: "clean", Review: "approved", Ref: "api#5", URL: "https://github.com/me/api/pull/5", Comments: 3},
	}

	t.Run("should group rows under tier headings", func(t *testing.T) {
		out := renderPanels(rows, 100, false, 0, view{})

		if !strings.Contains(out, "Needs action") || !strings.Contains(out, "Ready to merge") {
			t.Errorf("missing tier headings:\n%s", out)
		}
	})

	t.Run("should show comment count when nonzero", func(t *testing.T) {
		out := renderPanels(rows, 100, false, 0, view{})

		if !strings.Contains(out, "(3)") {
			t.Errorf("missing comment count:\n%s", out)
		}
	})

	t.Run("should group PRs nobody was asked to review under Needs reviewer", func(t *testing.T) {
		out := renderPanels([]Row{{Tier: tierNoReviewer, CI: "ok", Merge: "clean", URL: "https://x/1"}}, 100, false, 0, view{})

		if !strings.Contains(out, "Needs reviewer (1)") || !strings.Contains(out, "1 need reviewer") {
			t.Errorf("missing needs-reviewer heading or count:\n%s", out)
		}
	})

	t.Run("should end each heading with its icon and describe the category when wide", func(t *testing.T) {
		out := renderPanels([]Row{{Tier: tierMerged, URL: "https://x/1"}}, 160, false, 12, view{})

		if !strings.Contains(out, "╭─ Merged today (12) 🎉 ─ Merged since midnight ─") {
			t.Errorf("heading should read name, count, icon, description:\n%s", out)
		}
	})

	t.Run("should drop the description when the terminal is too narrow for it", func(t *testing.T) {
		out := renderPanels([]Row{{Tier: tierMerged, URL: "https://x/1"}}, 30, false, 0, view{})

		if strings.Contains(out, "Merged since midnight") {
			t.Errorf("description should be dropped on narrow width:\n%s", out)
		}
	})

	t.Run("should have one category per tier", func(t *testing.T) {
		if len(categories) != tierMerged+1 {
			t.Errorf("categories = %d, want %d", len(categories), tierMerged+1)
		}
	})

	t.Run("should emit no ANSI escapes when color is off", func(t *testing.T) {
		out := renderPanels(rows, 100, false, 0, view{})

		if strings.Contains(out, "\033") {
			t.Errorf("found ANSI escape with color off:\n%q", out)
		}
	})

	t.Run("should emit ANSI escapes when color is on", func(t *testing.T) {
		out := renderPanels(rows, 100, true, 0, view{})

		if !strings.Contains(out, "\033[") {
			t.Error("expected ANSI escapes with color on")
		}
	})

	t.Run("should render merged rows and exclude them from the open count", func(t *testing.T) {
		mixed := []Row{
			{Tier: tierNeedsAction, CI: "fail", Merge: "clean", Review: "none", URL: "https://x/1"},
			{Tier: tierMerged, CI: "none", Merge: "merged", Review: "none", URL: "https://x/2"},
		}

		out := renderPanels(mixed, 120, false, 0, view{})

		if !strings.Contains(out, "Merged today") || !strings.Contains(out, "merged") {
			t.Errorf("missing merged section:\n%s", out)
		}
		if !strings.Contains(out, "1 open") || !strings.Contains(out, "1 merged") {
			t.Errorf("summary should count 1 open, 1 merged:\n%s", out)
		}
	})

	t.Run("should show merge time instead of idle days for merged rows", func(t *testing.T) {
		mergedAt := time.Date(2026, 8, 4, 14, 32, 0, 0, time.UTC)
		mixed := []Row{
			{Tier: tierReady, CI: "ok", Merge: "clean", Review: "approved", URL: "https://x/1"},
			{Tier: tierMerged, Merge: "merged", MergedAt: &mergedAt, URL: "https://x/2"},
		}

		out := renderPanels(mixed, 120, false, 0, view{})

		want := mergedAt.Local().Format("15:04")
		if !strings.Contains(out, want) {
			t.Errorf("missing merge time %q:\n%s", want, out)
		}
	})

	t.Run("should state the real merged-today total in the heading when capped", func(t *testing.T) {
		mixed := []Row{
			{Tier: tierMerged, Merge: "merged", URL: "https://x/2"},
		}

		out := renderPanels(mixed, 120, false, 12, view{})

		if !strings.Contains(out, "Merged today (12)") {
			t.Errorf("heading should show total 12:\n%s", out)
		}
	})

	t.Run("should report empty state", func(t *testing.T) {
		if got := renderPanels(nil, 100, false, 0, view{}); !strings.Contains(got, "No open PRs") {
			t.Errorf("got %q", got)
		}
	})
}

func TestRenderHeader(t *testing.T) {
	t.Run("should render the header box with wordmark", func(t *testing.T) {
		out := renderHeader(0, false, 60, 40, headerInfo{})

		if !strings.Contains(out, "╔") || !strings.Contains(out, "╝") {
			t.Errorf("missing box:\n%s", out)
		}
		if !strings.Contains(out, "███") {
			t.Errorf("missing wordmark:\n%s", out)
		}
	})

	t.Run("should show scope on the left and clock and countdown on the right under watch", func(t *testing.T) {
		out := renderHeader(0, false, 70, 40, headerInfo{left: "acme", right: "15:04:05 · next refresh in 42s"})

		if !strings.Contains(out, "acme") || !strings.Contains(out, "next refresh in 42s") {
			t.Errorf("missing scope or refresh status:\n%s", out)
		}
		for _, ln := range strings.Split(strings.TrimSpace(out), "\n") {
			if got := runewidth.StringWidth(ln); got != 70 {
				t.Errorf("header line is %d wide, want 70: %q", got, ln)
			}
		}
	})

	t.Run("should fill the top border to the exact terminal width", func(t *testing.T) {
		out := renderHeader(0, false, 72, 40, headerInfo{})

		bar := strings.SplitN(out, "\n", 2)[0]

		if got := len([]rune(bar)); got != 72 {
			t.Errorf("border width = %d, want 72:\n%q", got, bar)
		}
	})

	t.Run("should collapse to a one-line bar on short terminals", func(t *testing.T) {
		out := renderHeader(0, false, 80, 10, headerInfo{})

		if strings.Contains(out, "╔") || strings.Contains(out, "███") {
			t.Errorf("box should collapse on short height:\n%s", out)
		}
		if !strings.Contains(out, appName) {
			t.Errorf("missing app name:\n%s", out)
		}
	})

	t.Run("should animate the wordmark colors on each tick", func(t *testing.T) {
		out0 := renderHeader(0, true, 60, 40, headerInfo{})
		out1 := renderHeader(1, true, 60, 40, headerInfo{})

		if out0 == out1 {
			t.Error("header should change between ticks")
		}
	})
}

func TestStatusColumn(t *testing.T) {
	t.Run("should have a theme entry for every status code", func(t *testing.T) {
		for tier := tierNeedsAction; tier <= tierMerged; tier++ {
			for _, queue := range []string{"", "UNMERGEABLE", "QUEUED"} {
				for _, ci := range []string{"ok", "fail"} {
					code := statusFor(tier, ci, "conflict", queue)
					if _, ok := statuses[code]; !ok {
						t.Errorf("no theme entry for status %q", code)
					}
				}
			}
		}
	})

	t.Run("should show the queue position", func(t *testing.T) {
		got := statusLabel(Row{Status: "queued", QueuePos: 2})

		if !strings.HasSuffix(got, "Queued #2") {
			t.Errorf("got %q", got)
		}
	})

	t.Run("should pad by display width so emoji rows align", func(t *testing.T) {
		rows := []Row{{Status: "ci_failed"}, {Status: "queued", QueuePos: 12}}
		w := statusWidth(rows)

		for _, r := range rows {
			if got := runewidth.StringWidth(statusField(r, w, colors(false))); got != w {
				t.Errorf("%s: width %d, want %d", r.Status, got, w)
			}
		}
	})
}

func TestPanels(t *testing.T) {
	rows := []Row{
		{Tier: tierNeedsAction, Status: "ci_failed", Repository: "api", Title: "Fix 日本語 title", URL: "https://github.com/me/api/pull/1"},
		{Tier: tierNeedsAction, Status: "comments", Repository: "web", Title: "Short", URL: "https://github.com/me/web/pull/22"},
		{Tier: tierDrafts, Status: "draft", Repository: "api", Title: "WIP", URL: "https://github.com/me/api/pull/3"},
	}

	for _, width := range []int{40, 100, 160} {
		t.Run("should draw every panel line at exactly the terminal width", func(t *testing.T) {
			out := renderPanels(rows, width, false, 0, view{})

			for _, ln := range strings.Split(out, "\n") {
				if strings.HasPrefix(ln, "│") || strings.HasPrefix(ln, "╭") || strings.HasPrefix(ln, "╰") {
					if got := runewidth.StringWidth(ln); got < width || got != runewidth.StringWidth(strings.Split(out, "\n")[2]) {
						t.Errorf("width %d: line is %d wide, panel not rectangular: %q", width, got, ln)
					}
				}
			}
		})
	}

	t.Run("should draw one panel per non-empty tier", func(t *testing.T) {
		out := renderPanels(rows, 100, false, 0, view{})

		if n := strings.Count(out, "╭"); n != 2 {
			t.Errorf("got %d panels, want 2:\n%s", n, out)
		}
	})
}

func TestPanelsWithView(t *testing.T) {
	rows := []Row{
		{Tier: tierNeedsAction, Status: "ci_failed", Repository: "api", Title: "Broken", URL: "https://x/1"},
		{Tier: tierReady, Status: "ready", Repository: "api", Title: "Fine", URL: "https://x/2"},
	}

	t.Run("should mark only the cursor row", func(t *testing.T) {
		out := renderPanels(rows, 100, false, 0, view{cursor: "https://x/2", interactive: true})

		if strings.Count(out, "❯") != 1 || !strings.Contains(out, "❯ ✅ Ready") {
			t.Errorf("cursor marker wrong:\n%s", out)
		}
	})

	t.Run("should emphasise the selected title without breaking alignment", func(t *testing.T) {
		out := renderPanels(rows, 100, true, 0, view{cursor: "https://x/2", interactive: true})

		if !strings.Contains(out, "\033[1mFine") {
			t.Errorf("selected title not bold:\n%s", out)
		}
		ansi := regexp.MustCompile("\x1b\\[[0-9;]*m")
		for _, l := range strings.Split(out, "\n") {
			plain := ansi.ReplaceAllString(l, "")
			if strings.Contains(plain, "│") && runewidth.StringWidth(plain) != 100 {
				t.Errorf("misaligned (%d): %q", runewidth.StringWidth(plain), plain)
			}
		}
	})

	t.Run("should fold a category to its border and keep the box aligned", func(t *testing.T) {
		out := renderPanels(rows, 100, false, 0, view{collapsed: map[int]bool{tierNeedsAction: true}, interactive: true})

		if strings.Contains(out, "Broken") || !strings.Contains(out, "Needs action (1) 🚨 ▸") {
			t.Errorf("folded panel should show a ▸ heading and no rows:\n%s", out)
		}
		for _, ln := range strings.Split(out, "\n") {
			if (strings.HasPrefix(ln, "╭") || strings.HasPrefix(ln, "╰")) && runewidth.StringWidth(ln) != 100 {
				t.Errorf("border is %d wide: %q", runewidth.StringWidth(ln), ln)
			}
		}
	})
}

func TestHeaderInfoNeverOverflows(t *testing.T) {
	info := headerInfo{left: "a-very-long-organisation/and-repo", right: "15:04:05 · next refresh in 30s"}

	for _, tc := range []struct{ width, height int }{{40, 40}, {46, 40}, {60, 40}, {60, 10}, {30, 10}, {20, 10}} {
		out := renderHeader(0, false, tc.width, tc.height, info)

		for _, ln := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
			if ln == "" {
				continue
			}
			if got := runewidth.StringWidth(ln); got > tc.width {
				t.Errorf("%dx%d: line is %d wide: %q", tc.width, tc.height, got, ln)
			}
		}
	}
}
