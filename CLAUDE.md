# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`gh pr-dash` — a precompiled `gh` extension (Go) that lists all your open PRs across every repo, ranked by what needs your action. No LLM, no config, no state; ships as one binary. Reuses `gh`'s auth via `go-gh`.

## Commands

```bash
make test       # go test ./...
make lint       # gofmt -l check + go vet (CI also runs golangci-lint)
make fmt        # gofmt -w .
make build      # go build -o gh-pr-dash .
make install    # build + gh extension install .
make uninstall  # gh extension remove pr-dash

go test -run TestName ./...   # single test
```

Default branch is `main`.

## Architecture

Flat `package main`, one concern per file. Pipeline: **fetch → classify/rank → render**.

- `main.go` — arg parsing (`parseArgs`), dispatch: `--json` prints once via `printJSON`; otherwise the live dashboard (`watchLoop`), which requires a TTY. `--watch` is accepted as a no-op.
- `dash.go` — GitHub GraphQL query + `fetchPRs`; `detectTerminal` (width + `NO_COLOR`/TTY detection); `fetchRows` and `printJSON`.
- `pr.go` — the core logic. `ghPR` (raw GraphQL shape) → `Row` (ranked, render-ready). `buildRows` filters by org and sorts. **Ranking = 7 tiers** (`tierNeedsAction` < `tierReady` < `tierBuilding` < `tierNoReviewer` < `tierWaiting` < `tierDrafts` < `tierMerged`), most-idle-first within each tier. Each `Row` also carries a `Status` code (`statusFor`): the single top reason for its tier.
- `panel.go` — bordered category panel helpers (top border with title + description, side-bordered lines, bottom). Padding is plain spaces computed from display width, before any color.
- `render.go` — terminal table rendering. Status stored as stable codes (ci: ok/fail/pending/none, merge: clean/conflict/unknown, review: approved/changes/review/none), styled here.
- `theme.go` — per-category presentation (name, description, emoji, heading color), indexed by tier and kept in tier order; also the `statuses` table (icon, label, color per `Status` code) and the status cell helpers.
- `header.go` — ASCII-art box header, animated header box; its last line shows scope (org/repo) on the left and clock + refresh countdown on the right (`headerInfo`).
- `watch.go` — Bubble Tea model (the default and only interactive mode): single `Update` loop, 1-min data ticker + 200ms animation ticker, cursor model (cursor tracked by PR URL, `o`/`m`/`a` on the selection, capitals on its whole category, `1-7` jump, `enter` fold; fetch results are held only while a merge is being confirmed or running).
- `actions.go` — side effects (open browser, deliver to agent/clipboard); each returns what actually happened so the status line never claims success it didn't get.

### Conventions that matter

- **`Row` holds status codes, not display strings** — renderers map codes → glyphs/colors. Keep display logic out of `pr.go`.
- **Padding before color.** ANSI escapes must never count toward column width — pad the plain string, then wrap in color (see `renderPanels`). Display-width (`go-runewidth`), not byte or rune count, for widths (`truncate`/`padRight`/`padLeft`).
- **URLs stay plain text** so they remain cmd-clickable through tmux — never color or decorate the URL column.
- `reviewCode` honors `latestOpinionatedReviews` as well as `reviewDecision`, because `reviewDecision` is empty in repos that don't *require* review.

## Release

Tag `vX.Y.Z` → `release.yml` builds cross-platform binaries via `cli/gh-extension-precompile`.
