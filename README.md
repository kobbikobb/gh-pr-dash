<div align="center">

<img src="assets/banner.svg" width="620" alt="gh pr-dash — all your open PRs, ranked by what needs your action">

<h1>gh pr-dash</h1>

<p><strong>All your open pull requests, across every repo, ranked by what needs your action.</strong></p>

<p>
  <a href="https://github.com/kobbikobb/gh-pr-dash/actions/workflows/ci.yml"><img src="https://github.com/kobbikobb/gh-pr-dash/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/kobbikobb/gh-pr-dash/releases/latest"><img src="https://img.shields.io/github/v/release/kobbikobb/gh-pr-dash?color=3fb950" alt="release"></a>
  <img src="https://img.shields.io/badge/license-MIT-blue" alt="MIT">
  <img src="https://img.shields.io/badge/gh-extension-58a6ff?logo=github" alt="gh extension">
</p>

</div>

No LLM, no config, no state — deterministic, and ships as one precompiled binary.

```
Needs action — CI fail / conflict (2)
  ✗ clean    conflict     1d api   extract idle watchdog into run-watchdog    https://github.com/me/api/pull/1008
  ✗ clean    clean        2d api   map user_id from transactions to goaml (1) https://github.com/me/api/pull/12442

Ready to merge (1)
  ✓ clean    clean        2d api   Validate regulator config on save (1)       https://github.com/me/api/pull/12385
```

Each row: **CI** (`✓` pass `✗` fail `•` pending), **merge** state, **repo**, **idle** days (or the merge time for merged rows), title (with comment count), and the full PR URL — plain text so it stays cmd-clickable, even through tmux.

## Install

```bash
gh extension install kobbikobb/gh-pr-dash
```

Requires [`gh`](https://cli.github.com), authenticated (`gh auth login`) — the extension reuses gh's auth.

## Use

```bash
gh pr-dash               # all your open PRs, ranked
gh pr-dash --org my-org  # scope to one org/owner
gh pr-dash --json        # raw JSON rows (for scripts)
gh pr-dash 50            # cap at 50 PRs (default 100)
gh pr-dash --watch       # interactive dashboard, refreshes every minute
```

### Watch mode keys

Pick an action, then a target. Nothing happens on a bare number.

| Key | Action |
|---|---|
| `o` | open PRs in the browser (max 10 at once) |
| `m` | merge — only **ready** PRs; lists them and waits for `y` |
| `x` | send PRs to an agent |
| `s` | Slack — not implemented |
| `q` | quit |

Targets: PR numbers then Enter (`1,3 5`), or a group key straight after the action — `n` needs action, `r` ready, `c` CI, `w` waiting, `d` drafts, `t` merged today, `a` all open. Esc cancels. Refreshes pause while you're mid-action so numbers don't shift.

Environment:

- `GH_PR_DASH_MERGE_FLAGS` — flags for `gh pr merge` (default `--squash`).
- `GH_PR_DASH_AGENT_CMD` — command to run with the PR list as its last argument (e.g. `claude -p`). The agent runs detached and its output is discarded. Unset, the list is copied to the clipboard instead. The status line says which happened.

## Ranking

Six tiers, most-stale-first within each:

| Tier | Meaning |
|---|---|
| **Needs action** | CI failing or merge conflict |
| **Ready to merge** | approved, CI green, no conflicts |
| **Waiting on CI** | approved but CI still running |
| **Waiting on review** | everything else that's open |
| **Drafts** | draft PRs |
| **Merged today** | merged since midnight (up to 10 listed, count shows the real total) |

Review status honors a standing approval or changes-request as well as `reviewDecision`, which is empty in repos that don't *require* review.

## Development

```bash
make test       # go test ./...
make lint       # gofmt check + go vet
make install    # build and install locally
make uninstall  # remove the local extension
```

Tagging `vX.Y.Z` builds cross-platform release binaries via [`cli/gh-extension-precompile`](https://github.com/cli/gh-extension-precompile).

## License

MIT
