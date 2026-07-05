// Command gh-pr-dash lists all your open pull requests across every repo,
// ranked by what needs your action. It is a precompiled gh extension.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"golang.org/x/term"
)

const usage = `gh pr-dash — rank your open PRs by what needs action

Usage: gh pr-dash [--org <name>] [--json] [--no-color] [max]

  --org <name>   Scope to a single org/owner
  --json         Emit raw JSON rows (for scripts)
  --no-color     Disable ANSI color and hyperlinks
  max            Max PRs to fetch (default 100)
`

const searchQuery = `query($q:String!,$n:Int!){
  search(query:$q,type:ISSUE,first:$n){
    nodes{
      ... on PullRequest{
        number title url isDraft reviewDecision updatedAt mergeable
        repository{ nameWithOwner }
        comments{ totalCount }
        latestOpinionatedReviews(first:20){ nodes{ state } }
        commits(last:1){ nodes{ commit{ statusCheckRollup{ state } } } }
      }
    }
  }
}`

type options struct {
	org     string
	limit   int
	asJSON  bool
	noColor bool
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
		case "--no-color":
			o.noColor = true
		case "-h", "--help":
			return o, false
		default:
			if n, err := strconv.Atoi(args[i]); err == nil {
				o.limit = n
			}
		}
	}
	return o, true
}

func fetchPRs(limit int) ([]ghPR, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, err
	}
	var resp struct {
		Search struct{ Nodes []ghPR }
	}
	vars := map[string]any{"q": "author:@me is:pr is:open", "n": limit}
	if err := client.Do(searchQuery, vars, &resp); err != nil {
		return nil, err
	}
	return resp.Search.Nodes, nil
}

func main() {
	opts, ok := parseArgs(os.Args[1:])
	if !ok {
		fmt.Print(usage)
		return
	}

	prs, err := fetchPRs(opts.limit)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	rows := buildRows(prs, opts.org, time.Now())

	if opts.asJSON {
		out, err := json.Marshal(rows)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
		return
	}

	fd := int(os.Stdout.Fd())
	useColor := !opts.noColor && os.Getenv("NO_COLOR") == "" && term.IsTerminal(fd)
	width := 120
	if w, _, err := term.GetSize(fd); err == nil && w > 0 {
		width = w
	}
	fmt.Print(renderTerminal(rows, width, useColor))
}
