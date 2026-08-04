package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"golang.org/x/term"
)

const searchQuery = `query($q:String!,$n:Int!){
  search(query:$q,type:ISSUE,first:$n){
    issueCount
    nodes{
      ... on PullRequest{
        number title url isDraft reviewDecision updatedAt mergedAt mergeable mergeStateStatus
        repository{ nameWithOwner }
        comments{ totalCount }
        latestOpinionatedReviews(first:20){ nodes{ state } }
        commits(last:1){ nodes{ commit{ statusCheckRollup{ state } } } }
      }
    }
  }
}`

// Cap on merged-today rows listed. The heading shows the real total from
// issueCount even when more than this many merged today.
const mergedLimit = 10

type terminal struct {
	width    int
	height   int
	useColor bool
}

func detectTerminal() terminal {
	fd := int(os.Stdout.Fd())
	useColor := os.Getenv("NO_COLOR") == "" && term.IsTerminal(fd)
	width, height := 120, 40
	if w, h, err := term.GetSize(fd); err == nil && w > 0 {
		width = w
		if h > 0 {
			height = h
		}
	}
	return terminal{width: width, height: height, useColor: useColor}
}

func fetchPRs(query string, limit int) ([]ghPR, int, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, 0, err
	}
	var resp struct {
		Search struct {
			IssueCount int
			Nodes      []ghPR
		}
	}
	vars := map[string]any{"q": query, "n": limit}
	if err := client.Do(searchQuery, vars, &resp); err != nil {
		return nil, 0, err
	}
	return resp.Search.Nodes, resp.Search.IssueCount, nil
}

// fetchRows returns the ranked open PRs followed by PRs merged today, plus the
// total number of PRs merged today (issueCount, pre-cap) so the heading can
// state the real count even when only mergedLimit rows are listed.
func fetchRows(opts options, now time.Time) ([]Row, int, error) {
	open, _, err := fetchPRs("author:@me is:pr is:open", opts.limit)
	if err != nil {
		return nil, 0, err
	}
	rows := buildRows(open, opts.org, now)

	// The open list is the primary job; a failure on the secondary merged
	// search just drops that section rather than blanking everything.
	mergedQ := "author:@me is:pr merged:>=" + now.Format("2006-01-02")
	if opts.org != "" {
		mergedQ += " org:" + opts.org
	}
	mergedToday := 0
	if merged, total, err := fetchPRs(mergedQ, mergedLimit); err == nil {
		mergedToday = total
		rows = append(rows, buildMergedRows(merged, opts.org, now)...)
	}
	return rows, mergedToday, nil
}

func fetchAndRender(w io.Writer, opts options) error {
	rows, mergedTotal, err := fetchRows(opts, time.Now())
	if err != nil {
		return err
	}

	if opts.asJSON {
		out, err := json.Marshal(rows)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(out))
		return err
	}

	t := detectTerminal()
	m := gutter(t.width)
	inner := t.width - 2*m
	header := renderHeader(0, opts.watch, t.useColor, inner, t.height, "")
	filtered := filterRows(rows, opts.repo)
	if opts.repo != "" && len(filtered) == 0 {
		fmt.Fprintf(os.Stderr, "warning: --repo %q matched no PRs\n", opts.repo)
	}
	_, err = fmt.Fprint(w, indent(header+renderTerminal(filtered, inner, t.useColor, mergedTotal), m))
	return err
}
