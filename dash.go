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
    nodes{
      ... on PullRequest{
        number title url isDraft reviewDecision updatedAt mergedAt mergeable
        repository{ nameWithOwner }
        comments{ totalCount }
        latestOpinionatedReviews(first:20){ nodes{ state } }
        commits(last:1){ nodes{ commit{ statusCheckRollup{ state } } } }
      }
    }
  }
}`

const mergedWindow = 24 * time.Hour

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

func fetchPRs(query string, limit int) ([]ghPR, error) {
	client, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, err
	}
	var resp struct {
		Search struct{ Nodes []ghPR }
	}
	vars := map[string]any{"q": query, "n": limit}
	if err := client.Do(searchQuery, vars, &resp); err != nil {
		return nil, err
	}
	return resp.Search.Nodes, nil
}

// fetchRows returns the ranked open PRs followed by PRs merged within the
// window; tierMerged sorts last, so appending keeps the sections in order.
func fetchRows(opts options, now time.Time) ([]Row, error) {
	open, err := fetchPRs("author:@me is:pr is:open", opts.limit)
	if err != nil {
		return nil, err
	}
	rows := buildRows(open, opts.org, now)

	// The open list is the primary job; a failure on the secondary merged
	// search just drops that section rather than blanking everything.
	since := now.Add(-mergedWindow).UTC().Format("2006-01-02T15:04:05Z")
	mergedQ := fmt.Sprintf("author:@me is:pr is:merged merged:>=%s sort:updated-desc", since)
	if merged, err := fetchPRs(mergedQ, opts.limit); err == nil {
		rows = append(rows, buildMergedRows(merged, opts.org, now)...)
	}
	return rows, nil
}

func fetchAndRender(w io.Writer, opts options) error {
	rows, err := fetchRows(opts, time.Now())
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
	header := renderHeader(0, opts.watch, t.useColor, t.width, t.height, "")
	_, err = fmt.Fprint(w, header+renderTerminal(rows, t.width, t.useColor))
	return err
}
