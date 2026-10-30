package main

import (
	"encoding/json"
	"errors"
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
        author{ login }
        repository{ nameWithOwner }
        comments{ totalCount }
        reviewRequests{ totalCount }
        mergeQueueEntry{ position state }
        reviewThreads(first:100){ nodes{ isResolved } }
        latestReviews(first:20){ nodes{ author{ login } } }
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
	isTTY    bool
}

func detectTerminal() terminal {
	fd := int(os.Stdout.Fd())
	isTTY := term.IsTerminal(fd)
	useColor := os.Getenv("NO_COLOR") == "" && isTTY
	width, height := 120, 40
	if w, h, err := term.GetSize(fd); err == nil && w > 0 {
		width = w
		if h > 0 {
			height = h
		}
	}
	return terminal{width: width, height: height, useColor: useColor, isTTY: isTTY}
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
	// GitHub's GraphQL endpoint times out now and then; retry gateway errors.
	for attempt := 0; attempt < 3; attempt++ {
		if err = client.Do(searchQuery, vars, &resp); err == nil || !isGatewayError(err) {
			break
		}
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	if err != nil {
		return nil, 0, err
	}
	return resp.Search.Nodes, resp.Search.IssueCount, nil
}

// mergedQuery scopes PRs merged since local midnight. GitHub evaluates search
// date qualifiers at UTC, so a bare date would shift the boundary by the local
// UTC offset; pass the local-midnight instant with its own offset instead.
func mergedQuery(now time.Time, org string) string {
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	q := "author:@me is:pr merged:>=" + midnight.Format("2006-01-02T15:04:05Z07:00")
	if org != "" {
		q += " org:" + org
	}
	return q
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
	mergedToday := 0
	if merged, total, err := fetchPRs(mergedQuery(now, opts.org), mergedLimit); err == nil {
		mergedToday = total
		rows = append(rows, buildMergedRows(merged, opts.org, now)...)
	}
	return rows, mergedToday, nil
}

// printJSON fetches once and writes the ranked rows as a JSON array.
func printJSON(w io.Writer, opts options) error {
	rows, _, err := fetchRows(opts, time.Now())
	if err != nil {
		return err
	}
	out, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(out))
	return err
}

func isGatewayError(err error) bool {
	var httpErr *api.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode >= 502 && httpErr.StatusCode <= 504
	}
	return false
}
