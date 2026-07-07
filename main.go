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
)

const usage = `gh pr-dash — rank your open PRs by what needs action

Usage: gh pr-dash [--org <name>] [--json] [max]

  --org <name>   Scope to a single org/owner
  --json         Emit raw JSON rows (for scripts)
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
	org    string
	limit  int
	asJSON bool
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

	fmt.Print(renderTerminal(rows, 120))
}
