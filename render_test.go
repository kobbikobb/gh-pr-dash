package main

import (
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

	t.Run("should truncate by display width so wide characters fit", func(t *testing.T) {
		got := truncate("🎉🎉🎉🎉", 5)

		if w := runewidth.StringWidth(got); w > 5 {
			t.Errorf("width = %d, want <= 5 (%q)", w, got)
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

	t.Run("should pad emoji by display width", func(t *testing.T) {
		if got := padRight("🎉", 4); runewidth.StringWidth(got) != 4 {
			t.Errorf("padRight width = %d, want 4", runewidth.StringWidth(got))
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
		{Tier: tierNeedsAction, Status: "ci_failed", IdleDays: 13, CI: "fail", Merge: "clean", Review: "none", Ref: "glint#20", URL: "https://github.com/me/glint/pull/20"},
		{Tier: tierReady, Status: "ready", IdleDays: 2, CI: "ok", Merge: "clean", Review: "approved", Ref: "api#5", URL: "https://github.com/me/api/pull/5"},
	}

	t.Run("should group rows under tier headings", func(t *testing.T) {
		out := renderTerminal(rows, 100, false, 0)

		if !strings.Contains(out, "Needs action") || !strings.Contains(out, "Ready to merge") {
			t.Errorf("missing tier headings:\n%s", out)
		}
	})

	t.Run("should show unresolved comment count for the comments status", func(t *testing.T) {
		commented := []Row{{Tier: tierNeedsAction, Status: "comments", Unresolved: 3, Repository: "api", URL: "https://x/1"}}

		out := renderTerminal(commented, 100, false, 0)

		if !strings.Contains(out, "💬") || !strings.Contains(out, "3 comments") {
			t.Errorf("missing comment status:\n%s", out)
		}
	})

	t.Run("should show a status emoji only on needs-action rows", func(t *testing.T) {
		mixed := []Row{
			{Tier: tierNeedsAction, Status: "ci_failed", Repository: "api", Title: "t", URL: "https://x/1"},
			{Tier: tierWaiting, Status: "in_review", Repository: "api", Title: "t", URL: "https://x/2"},
		}

		out := renderTerminal(mixed, 120, false, 0)

		for _, ln := range strings.Split(out, "\n") {
			switch {
			case strings.Contains(ln, "https://x/1"):
				if !strings.Contains(ln, "🔴") || !strings.Contains(ln, "CI failed") {
					t.Errorf("needs-action row missing status: %q", ln)
				}
			case strings.Contains(ln, "https://x/2"):
				if strings.Contains(ln, "👀") || strings.Contains(ln, "in review") {
					t.Errorf("waiting row should not repeat its heading: %q", ln)
				}
			}
		}
	})

	t.Run("should put each tier's emoji in its heading", func(t *testing.T) {
		out := renderTerminal([]Row{{Tier: tierNoReviewer, Status: "no_reviewer", URL: "https://x/1"}}, 120, false, 0)

		if !strings.Contains(out, "🙋 Needs reviewer (1)") {
			t.Errorf("heading missing emoji:\n%s", out)
		}
	})

	t.Run("should omit the status column when no row needs action", func(t *testing.T) {
		out := renderTerminal([]Row{{Tier: tierDrafts, Status: "draft", Repository: "api", URL: "https://x/1"}}, 120, false, 0)

		if strings.Contains(out, "draft  ") || strings.Contains(out, "📝 draft") {
			t.Errorf("unexpected status text:\n%s", out)
		}
	})

	t.Run("should align every column across groups using display width", func(t *testing.T) {
		mixed := []Row{
			{Tier: tierNeedsAction, Status: "ci_failed", Repository: "api", Title: "one", URL: "https://x/1"},
			{Tier: tierNoReviewer, Status: "no_reviewer", Repository: "gitops-tenants", Title: "two", URL: "https://x/2"},
			{Tier: tierWaiting, Status: "in_review", Repository: "web", Title: "three", URL: "https://x/3"},
		}

		out := renderTerminal(mixed, 140, false, 0)

		col := -1
		for _, ln := range strings.Split(out, "\n") {
			i := strings.Index(ln, "https://x/")
			if i < 0 {
				continue
			}
			w := runewidth.StringWidth(ln[:i])
			if col != -1 && w != col {
				t.Errorf("URL column at %d, want %d in %q", w, col, ln)
			}
			col = w
		}
	})

	t.Run("should show the clickable PR url as plain text", func(t *testing.T) {
		out := renderTerminal(rows, 200, false, 0)

		if !strings.Contains(out, "https://github.com/me/glint/pull/20") {
			t.Errorf("missing PR url:\n%s", out)
		}
	})

	t.Run("should emit no ANSI escapes when color is off", func(t *testing.T) {
		out := renderTerminal(rows, 100, false, 0)

		if strings.Contains(out, "\033") {
			t.Errorf("found ANSI escape with color off:\n%q", out)
		}
	})

	t.Run("should emit ANSI escapes when color is on", func(t *testing.T) {
		out := renderTerminal(rows, 100, true, 0)

		if !strings.Contains(out, "\033[") {
			t.Error("expected ANSI escapes with color on")
		}
	})

	t.Run("should render merged rows and exclude them from the open count", func(t *testing.T) {
		mixed := []Row{
			{Tier: tierNeedsAction, Status: "ci_failed", URL: "https://x/1"},
			{Tier: tierMerged, Status: "merged", URL: "https://x/2"},
		}

		out := renderTerminal(mixed, 120, false, 0)

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
			{Tier: tierReady, Status: "ready", URL: "https://x/1"},
			{Tier: tierMerged, Status: "merged", MergedAt: &mergedAt, URL: "https://x/2"},
		}

		out := renderTerminal(mixed, 120, false, 0)

		want := mergedAt.Local().Format("15:04")
		if !strings.Contains(out, want) {
			t.Errorf("missing merge time %q:\n%s", want, out)
		}
	})

	t.Run("should state the real merged-today total in the heading when capped", func(t *testing.T) {
		mixed := []Row{
			{Tier: tierMerged, Merge: "merged", URL: "https://x/2"},
		}

		out := renderTerminal(mixed, 120, false, 12)

		if !strings.Contains(out, "Merged today (12)") {
			t.Errorf("heading should show total 12:\n%s", out)
		}
	})

	t.Run("should report empty state", func(t *testing.T) {
		if got := renderTerminal(nil, 100, false, 0); !strings.Contains(got, "No open PRs") {
			t.Errorf("got %q", got)
		}
	})
}

func TestRenderHeader(t *testing.T) {
	t.Run("should render static header box with wordmark", func(t *testing.T) {
		out := renderHeader(0, false, false, 60, 40, "")

		if !strings.Contains(out, "╔") || !strings.Contains(out, "╝") {
			t.Errorf("missing box:\n%s", out)
		}
		if !strings.Contains(out, "███") || !strings.Contains(out, appName) {
			t.Errorf("missing wordmark:\n%s", out)
		}
	})

	t.Run("should render the refresh status under watch", func(t *testing.T) {
		out := renderHeader(0, true, false, 60, 40, "refreshed 15:04 · next 42s")

		if !strings.Contains(out, "next 42s") {
			t.Errorf("missing refresh status:\n%s", out)
		}
	})

	t.Run("should fill the top border to the exact terminal width", func(t *testing.T) {
		out := renderHeader(0, true, false, 72, 40, "")

		bar := strings.SplitN(out, "\n", 2)[0]

		if got := len([]rune(bar)); got != 72 {
			t.Errorf("border width = %d, want 72:\n%q", got, bar)
		}
	})

	t.Run("should collapse to a one-line bar on short terminals", func(t *testing.T) {
		out := renderHeader(0, false, false, 80, 10, "")

		if strings.Contains(out, "╔") || strings.Contains(out, "███") {
			t.Errorf("box should collapse on short height:\n%s", out)
		}
		if !strings.Contains(out, appName) {
			t.Errorf("missing app name:\n%s", out)
		}
	})

	t.Run("should animate the wordmark colors on each tick", func(t *testing.T) {
		out0 := renderHeader(0, true, true, 60, 40, "")
		out1 := renderHeader(1, true, true, 60, 40, "")

		if out0 == out1 {
			t.Error("header should change between ticks")
		}
	})
}
