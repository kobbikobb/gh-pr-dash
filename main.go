// Command gh-pr-dash lists all your open pull requests across every repo,
// ranked by what needs your action. It is a precompiled gh extension.
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const usage = `gh pr-dash — rank your open PRs by what needs action

Usage: gh pr-dash [--org <name>] [--repo <name>] [--json] [--interval <dur>] [max]

  --org <name>       Scope to a single org/owner
  --repo <name>      Scope to a single repo
  --json             Print raw JSON rows once and exit (for scripts)
  --interval <dur>   Refresh interval, e.g. 30s, 2m (default 1m)
  max                Max PRs to fetch (default 100)
`

type options struct {
	org      string
	repo     string
	limit    int
	asJSON   bool
	interval time.Duration
}

func parseArgs(args []string) (options, bool) {
	o := options{limit: 100, interval: time.Minute}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--org":
			if i++; i < len(args) {
				o.org = args[i]
			}
		case "--repo":
			if i++; i < len(args) {
				o.repo = args[i]
			}
		case "--interval":
			if i++; i < len(args) {
				d, err := time.ParseDuration(args[i])
				if err != nil || d < time.Second {
					fmt.Fprintf(os.Stderr, "invalid interval: %s\n", args[i])
					return o, false
				}
				o.interval = d
			}
		case "--json":
			o.asJSON = true
		case "--watch":
			// The dashboard is always live now; accepted so old aliases keep working.
		case "-h", "--help":
			return o, false
		default:
			n, err := strconv.Atoi(args[i])
			if err != nil {
				fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
				return o, false
			}
			if n <= 0 {
				fmt.Fprintf(os.Stderr, "max must be positive: %s\n", args[i])
				return o, false
			}
			o.limit = n
		}
	}
	return o, true
}

func main() {
	opts, ok := parseArgs(os.Args[1:])
	if !ok {
		fmt.Print(usage)
		return
	}

	if opts.asJSON {
		if err := printJSON(os.Stdout, opts); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	if !detectTerminal().isTTY {
		fmt.Fprintln(os.Stderr, "error: gh pr-dash is an interactive dashboard and needs a terminal; use --json for scripts")
		os.Exit(1)
	}
	watchLoop(opts)
}
