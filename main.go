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

Usage: gh pr-dash [--org <name>] [--json] [--watch] [--interval <dur>] [max]

  --org <name>       Scope to a single org/owner
  --json             Emit raw JSON rows (for scripts)
  --watch            Refresh on an interval
  --interval <dur>   Watch refresh interval, e.g. 30s, 2m (default 1m)
  max                Max PRs to fetch (default 100)
`

type options struct {
	org      string
	limit    int
	asJSON   bool
	watch    bool
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
			o.watch = true
		case "-h", "--help":
			return o, false
		default:
			if n, err := strconv.Atoi(args[i]); err == nil {
				o.limit = n
			} else {
				fmt.Fprintf(os.Stderr, "unknown flag: %s\n", args[i])
				return o, false
			}
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

	if opts.watch {
		watchLoop(opts)
		return
	}

	if err := fetchAndRender(os.Stdout, opts, 0); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
