# gh-pr-dash

A [GitHub CLI](https://cli.github.com) extension that lists **all your open pull requests across every repo**, ranked by what needs your action.

No LLM, no config, no state — a deterministic pipeline over `gh` + `jq`. Same input, same output.

```
## 🔴 Needs action (CI fail / conflict)
| PR | Title | CI | Merge | Review | Idle | 💬 | URL |
|---|---|---|---|---|---|---|---|
| app#20   | Google Calendar OAuth   | ❌ | clean       | —         | 13d | 0 | … |
| api#1008 | extract idle watchdog   | ❌ | ❌ conflict | —         | 1d  | 0 | … |

## 🟢 Ready to merge
| api#12385 | Validate regulator config | ✅ | clean | ✅ approved | 2d | 1 | … |

## 🟡 Waiting on review
| api#12423 | Replace getMany endpoint  | ✅ | clean | 🔶 review   | 2d | 1 | … |
```

## Install

```bash
gh extension install kobbikobb/gh-pr-dash
```

## Usage

```bash
gh pr-dash                    # all your open PRs, ranked
gh pr-dash --org my-org       # scope to one org/owner
gh pr-dash --json             # raw JSON rows (for scripts)
gh pr-dash --no-emoji         # plain text status
gh pr-dash 50                 # cap at 50 PRs (default 100)
```

## How PRs are ranked

Four tiers, oldest-first within each:

| Tier | Meaning |
|---|---|
| 🔴 **Needs action** | CI failing **or** merge conflict |
| 🟢 **Ready to merge** | approved, CI green, no conflicts |
| 🟡 **Waiting on review** | everything else that's open |
| ⚪ **Drafts** | draft PRs |

Columns: **CI** (rolled-up check state), **Merge** (mergeable / conflict), **Review** (approved / changes / review-required), **Idle** (days since last update), **💬** (comment count).

Review status also honors a standing approval or changes-request in `latestReviews`, since `reviewDecision` is empty in repos that don't *require* review.

## Requirements

- [`gh`](https://cli.github.com) (authenticated: `gh auth login`)
- [`jq`](https://jqlang.github.io/jq/)
- `bash`, `awk`, `xargs` (standard on macOS/Linux)

## License

MIT
