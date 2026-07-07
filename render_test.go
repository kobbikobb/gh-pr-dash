package main

import (
	"strings"
	"testing"
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
		{Tier: tierNeedsAction, IdleDays: 13, CI: "fail", Merge: "clean", Review: "none", Ref: "glint#20", Repo: "glint", URL: "https://github.com/me/glint/pull/20"},
		{Tier: tierReady, IdleDays: 2, CI: "ok", Merge: "clean", Review: "approved", Ref: "api#5", Repo: "api", URL: "https://github.com/me/api/pull/5", Comments: 3},
	}

	t.Run("should group rows under tier headings", func(t *testing.T) {
		out := renderTerminal(rows, 100)

		if !strings.Contains(out, "Needs action") || !strings.Contains(out, "Ready to merge") {
			t.Errorf("missing tier headings:\n%s", out)
		}
	})

	t.Run("should show comment count when nonzero", func(t *testing.T) {
		out := renderTerminal(rows, 100)

		if !strings.Contains(out, "(3)") {
			t.Errorf("missing comment count:\n%s", out)
		}
	})

	t.Run("should show the clickable PR url as plain text", func(t *testing.T) {
		out := renderTerminal(rows, 200)

		if !strings.Contains(out, "https://github.com/me/glint/pull/20") {
			t.Errorf("missing PR url:\n%s", out)
		}
	})

	t.Run("should emit ANSI escapes", func(t *testing.T) {
		out := renderTerminal(rows, 100)

		if !strings.Contains(out, "\033[") {
			t.Error("expected ANSI escapes")
		}
	})

	t.Run("should report empty state", func(t *testing.T) {
		if got := renderTerminal(nil, 100); !strings.Contains(got, "No open PRs") {
			t.Errorf("got %q", got)
		}
	})
}
