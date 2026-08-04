package main

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

// ghPR is the shape returned by the GitHub GraphQL search query.
type ghPR struct {
	Number           int
	Title            string
	URL              string
	IsDraft          bool
	ReviewDecision   string
	UpdatedAt        time.Time
	MergedAt         time.Time
	Mergeable        string
	MergeStateStatus string
	Repository       struct{ NameWithOwner string }
	Comments         struct{ TotalCount int }

	LatestOpinionatedReviews struct {
		Nodes []struct{ State string }
	}
	Commits struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct{ State string }
			}
		}
	}
}

// Row is a ranked PR ready for rendering. Status fields hold stable codes,
// not display strings, so each renderer can style them independently.
//
//	ci:     ok | fail | pending | none
//	merge:  clean | conflict | unknown | merged
//	review: approved | changes | review | none
type Row struct {
	Tier       int        `json:"tier"`
	IdleDays   int        `json:"idle_days"`
	CI         string     `json:"ci"`
	Merge      string     `json:"merge"`
	Review     string     `json:"review"`
	Ref        string     `json:"pr"`
	Repository string     `json:"repo"`
	URL        string     `json:"url"`
	Comments   int        `json:"comments"`
	Title      string     `json:"title"`
	MergedAt   *time.Time `json:"merged_at,omitempty"`
}

// Tier values, lowest sorts first.
const (
	tierNeedsAction = iota
	tierReady
	tierBuilding
	tierWaiting
	tierDrafts
	tierMerged
)

func ciCode(state string) string {
	switch state {
	case "SUCCESS":
		return "ok"
	case "FAILURE", "ERROR":
		return "fail"
	case "PENDING", "EXPECTED":
		return "pending"
	default:
		return "none"
	}
}

func mergeCode(m string) string {
	switch m {
	case "CONFLICTING":
		return "conflict"
	case "MERGEABLE":
		return "clean"
	default:
		return "unknown"
	}
}

// reviewCode honors latestOpinionatedReviews as well as reviewDecision, since
// reviewDecision is empty in repos that do not *require* review.
func reviewCode(pr ghPR) string {
	if pr.IsDraft {
		return "none"
	}
	changes := pr.ReviewDecision == "CHANGES_REQUESTED"
	approved := pr.ReviewDecision == "APPROVED"
	for _, r := range pr.LatestOpinionatedReviews.Nodes {
		switch r.State {
		case "CHANGES_REQUESTED":
			changes = true
		case "APPROVED":
			approved = true
		}
	}
	switch {
	case changes:
		return "changes"
	case approved:
		return "approved"
	case pr.ReviewDecision == "REVIEW_REQUIRED":
		return "review"
	default:
		return "none"
	}
}

func rollupState(pr ghPR) string {
	if len(pr.Commits.Nodes) == 0 {
		return ""
	}
	if r := pr.Commits.Nodes[0].Commit.StatusCheckRollup; r != nil {
		return r.State
	}
	return ""
}

func tierFor(pr ghPR, ci, merge, review string) int {
	switch {
	case pr.IsDraft:
		return tierDrafts
	case ci == "fail" || merge == "conflict":
		return tierNeedsAction
	case review == "approved" && merge == "clean":
		switch ci {
		case "pending":
			return tierBuilding
		case "ok":
			return tierReady
		default:
			// No rollup: ready only when GitHub says nothing blocks the merge;
			// a non-CLEAN state means required checks just haven't reported yet.
			if pr.MergeStateStatus == "CLEAN" {
				return tierReady
			}
			return tierBuilding
		}
	default:
		return tierWaiting
	}
}

func classify(pr ghPR, now time.Time) Row {
	ci := ciCode(rollupState(pr))
	merge := mergeCode(pr.Mergeable)
	review := reviewCode(pr)

	return Row{
		Tier:       tierFor(pr, ci, merge, review),
		IdleDays:   int(now.Sub(pr.UpdatedAt).Hours() / 24),
		CI:         ci,
		Merge:      merge,
		Review:     review,
		Ref:        shortRef(pr),
		Repository: repoFromRef(shortRef(pr)),
		URL:        pr.URL,
		Comments:   pr.Comments.TotalCount,
		Title:      pr.Title,
	}
}

func shortRef(pr ghPR) string {
	short := pr.Repository.NameWithOwner
	if i := strings.Index(short, "/"); i >= 0 {
		short = short[i+1:]
	}
	return short + "#" + strconv.Itoa(pr.Number)
}

// repoFromRef extracts the repo name from a short ref like "repo#42".
func repoFromRef(ref string) string {
	if i := strings.Index(ref, "#"); i >= 0 {
		return ref[:i]
	}
	return ref
}

func filterRows(rows []Row, repo string) []Row {
	if repo == "" {
		return rows
	}
	var filtered []Row
	for _, r := range rows {
		if r.Repository == repo {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// classifyMerged builds a row for an already-merged PR; CI/review status is
// no longer actionable, so only the merge marker and merge time are shown.
func classifyMerged(pr ghPR, now time.Time) Row {
	t := pr.MergedAt
	return Row{
		Tier:       tierMerged,
		IdleDays:   int(now.Sub(pr.MergedAt).Hours() / 24),
		CI:         "none",
		Merge:      "merged",
		Review:     "none",
		Ref:        shortRef(pr),
		Repository: repoFromRef(shortRef(pr)),
		URL:        pr.URL,
		Comments:   pr.Comments.TotalCount,
		Title:      pr.Title,
		MergedAt:   &t,
	}
}

// buildMergedRows filters by owner and sorts newest-merge-first; most recently
// merged PRs are the ones worth looking at.
func buildMergedRows(prs []ghPR, org string, now time.Time) []Row {
	rows := make([]Row, 0, len(prs))
	for _, pr := range prs {
		if pr.Number == 0 {
			continue
		}
		if org != "" && !strings.HasPrefix(pr.Repository.NameWithOwner, org+"/") {
			continue
		}
		rows = append(rows, classifyMerged(pr, now))
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].MergedAt == nil || rows[j].MergedAt == nil {
			return false
		}
		return rows[i].MergedAt.After(*rows[j].MergedAt)
	})
	return rows
}

// buildRows classifies, filters by owner, and ranks: tier ascending, then
// oldest-first (most idle days) within each tier.
func buildRows(prs []ghPR, org string, now time.Time) []Row {
	rows := make([]Row, 0, len(prs))
	for _, pr := range prs {
		if pr.Number == 0 {
			continue
		}
		if org != "" && !strings.HasPrefix(pr.Repository.NameWithOwner, org+"/") {
			continue
		}
		rows = append(rows, classify(pr, now))
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Tier != rows[j].Tier {
			return rows[i].Tier < rows[j].Tier
		}
		return rows[i].IdleDays > rows[j].IdleDays
	})
	return rows
}
