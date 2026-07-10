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
        number title url isDraft reviewDecision updatedAt mergeable
        repository{ nameWithOwner }
        comments{ totalCount }
        latestOpinionatedReviews(first:20){ nodes{ state } }
        commits(last:1){ nodes{ commit{ statusCheckRollup{ state } } } }
      }
    }
  }
}`

type terminal struct {
	width   int
	useColor bool
}

func detectTerminal() terminal {
	fd := int(os.Stdout.Fd())
	useColor := os.Getenv("NO_COLOR") == "" && term.IsTerminal(fd)
	width := 120
	if w, _, err := term.GetSize(fd); err == nil && w > 0 {
		width = w
	}
	return terminal{width: width, useColor: useColor}
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

func fetchAndRender(w io.Writer, opts options, tick int) error {
	prs, err := fetchPRs(opts.limit)
	if err != nil {
		return err
	}
	rows := buildRows(prs, opts.org, time.Now())

	if opts.asJSON {
		out, err := json.Marshal(rows)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(out))
		return err
	}

	t := detectTerminal()
	header := renderHeader(tick, opts.watch, t.useColor)
	_, err = fmt.Fprint(w, header+renderTerminal(rows, t.width, t.useColor))
	return err
}
