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

A single command that answers *"what should I touch next?"* — no LLM, no config, no state. Deterministic: same input, same output. Ships as one precompiled binary, so there's nothing else to install.

```
Needs action (CI fail / conflict)
  ✗ conflict ·   1d api#1008  extract idle watchdog into run-watchdog    https://github.com/me/api/pull/1008
  ✗ clean    review 2d api#12442 map user_id from transactions to goaml (1) https://github.com/me/api/pull/12442

Ready to merge
  ✓ clean    approved 2d api#12385 Validate regulator config on save (1)   https://github.com/me/api/pull/12385

Waiting on review
  ✓ clean    review 2d api#12423 Replace getMany with internal endpoint (1) https://github.com/me/api/pull/12423
```

## Install

```bash
gh extension install kobbikobb/gh-pr-dash
```

Upgrade any time with `gh extension upgrade pr-dash`. Requires [`gh`](https://cli.github.com), already authenticated (`gh auth login`) — the extension reuses gh's auth, so there's no token to set up.

## Use

```bash
gh pr-dash                # all your open PRs, ranked
gh pr-dash --org my-org   # scope to one org/owner
gh pr-dash --json         # raw JSON rows (for scripts)
gh pr-dash --no-color     # plain text, no ANSI
gh pr-dash 50             # cap at 50 PRs (default 100)
```

Color turns on automatically for a terminal and off when piped or when `NO_COLOR` is set. The PR URL is printed as plain text so it stays cmd-clickable everywhere — including inside tmux, where OSC-8 hyperlinks don't reliably pass through.

## What the columns mean

| Column | Values |
|---|---|
| **CI** | `✓` pass · `✗` fail · `•` pending · `·` none |
| **merge** | clean · conflict · unknown |
| **review** | approved · changes · review · none |
| **idle** | days since the PR last moved |
| **ref** | `repo#number` |
| **title** | with comment count in `(n)` |
| **url** | full PR link |

## How PRs are ranked

Four tiers, oldest-first within each — so the most-stale thing that needs you sits at the top:

| Tier | Meaning |
|---|---|
| **Needs action** | CI failing **or** merge conflict |
| **Ready to merge** | approved, CI green, no conflicts |
| **Waiting on review** | everything else that's open |
| **Drafts** | draft PRs |

Review status honors a standing approval or changes-request as well as the repo's `reviewDecision`, since `reviewDecision` is empty in repos that don't *require* review.

## Development

```bash
make build      # build ./gh-pr-dash
make test       # go test ./...
make lint       # gofmt check + go vet
make install    # build and install as a local gh extension
make uninstall  # remove the local gh extension
```

CI runs gofmt, `go vet`, build, tests, and golangci-lint on every push and PR. Tagging `vX.Y.Z` builds cross-platform release binaries via [`cli/gh-extension-precompile`](https://github.com/cli/gh-extension-precompile).

## License

MIT
