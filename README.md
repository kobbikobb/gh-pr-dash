# gh-pr-dash

A [GitHub CLI](https://cli.github.com) extension that lists **all your open pull requests across every repo**, ranked by what needs your action.

No LLM, no config, no state — deterministic. Same input, same output. Ships as a single precompiled binary, so there's nothing else to install.

```
Needs action (CI fail / conflict)
  ✗ conflict ·         1d api#1008  extract idle watchdog into run-watchdog       https://github.com/me/api/pull/1008
  ✗ clean    review    2d api#12442 map user_id from transactions to goaml (1)    https://github.com/me/api/pull/12442

Ready to merge
  ✓ clean    approved  2d api#12385 Validate regulator config on save (1)         https://github.com/me/api/pull/12385
```

Columns: **CI** (✓ pass / ✗ fail / • pending / · none), **merge** (clean / conflict / unknown), **review** (approved / changes / review / none), **idle** (days since last update), the PR ref, the title (with comment count), and the full PR URL. The URL is plain text so it stays cmd-clickable everywhere — including inside tmux, where OSC-8 hyperlinks don't reliably pass through.

## Install

```bash
gh extension install kobbikobb/gh-pr-dash
```

Upgrade with `gh extension upgrade pr-dash`.

## Usage

```bash
gh pr-dash                    # all your open PRs, ranked
gh pr-dash --org my-org       # scope to one org/owner
gh pr-dash --json             # raw JSON rows (for scripts)
gh pr-dash --no-color         # plain text, no ANSI / hyperlinks
gh pr-dash 50                 # cap at 50 PRs (default 100)
```

Color and hyperlinks are enabled automatically when writing to a terminal and disabled when piped or when `NO_COLOR` is set.

## How PRs are ranked

Four tiers, oldest-first within each:

| Tier | Meaning |
|---|---|
| **Needs action** | CI failing **or** merge conflict |
| **Ready to merge** | approved, CI green, no conflicts |
| **Waiting on review** | everything else that's open |
| **Drafts** | draft PRs |

Review status honors a standing approval or changes-request as well as the repo's `reviewDecision`, since `reviewDecision` is empty in repos that don't *require* review.

## Requirements

- [`gh`](https://cli.github.com), authenticated (`gh auth login`). The extension reuses gh's auth — no token setup.

## Development

```bash
make build     # build ./gh-pr-dash
make test      # go test ./...
make lint      # gofmt check + go vet
make install   # build and install as a local gh extension
make uninstall # remove the local gh extension
```

CI runs gofmt, `go vet`, build, tests, and golangci-lint on every push and PR. Tagging `vX.Y.Z` builds cross-platform release binaries via [`cli/gh-extension-precompile`](https://github.com/cli/gh-extension-precompile).

## License

MIT
