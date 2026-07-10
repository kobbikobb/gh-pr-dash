// Command gh-pr-dash lists all your open pull requests across every repo,
// ranked by what needs your action. It is a precompiled gh extension.
package main

import (
	"fmt"
	"os"
	"strconv"
)

const usage = `gh pr-dash — rank your open PRs by what needs action

Usage: gh pr-dash [--org <name>] [--json] [--watch] [max]

  --org <name>   Scope to a single org/owner
  --json         Emit raw JSON rows (for scripts)
  --watch        Refresh every minute
  max            Max PRs to fetch (default 100)
`

type options struct {
	org    string
	limit  int
	asJSON bool
	watch  bool
}

func parseArgs(args []string) (options, bool) {
	o := options{limit: 100}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--org":
			if i++; i < len(args) {
				o.org = args[i]
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

	if err := fetchAndRender(os.Stdout, opts); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
