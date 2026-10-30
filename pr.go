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
	Author           struct{ Login string }
	Repository       struct{ NameWithOwner string }
	Comments         struct{ TotalCount int }
	ReviewRequests   struct{ TotalCount int }
	MergeQueueEntry  *struct {
		Position int
		State    string
	}

	ReviewThreads struct {
		Nodes []struct{ IsResolved bool }
	}
	LatestReviews struct {
		Nodes []struct{ Author struct{ Login string } }
	}

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
//	status: ci_failed | conflict | comments | ready | building |
//	        queued | queue_failed | no_reviewer | in_review | draft | merged
//
// Status is the single most important reason for the row's tier.
type Row struct {
	Tier       int        `json:"tier"`
	IdleDays   int        `json:"idle_days"`
	CI         string     `json:"ci"`
	Merge      string     `json:"merge"`
	Review     string     `json:"review"`
	Status     string     `json:"status"`
	Ref        string     `json:"pr"`
	Repository string     `json:"repo"`
	URL        string     `json:"url"`
	Comments   int        `json:"comments"`
	Unresolved int        `json:"unresolved_threads"`
	QueuePos   int        `json:"queue_position,omitempty"`
	Title      string     `json:"title"`
	MergedAt   *time.Time `json:"merged_at,omitempty"`
}

// Tier values, lowest sorts first.
const (
	tierNeedsAction = iota
	tierReady
	tierBuilding
	tierNoReviewer
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
	// Some approval exists but GitHub still wants more (e.g. a required team),
	// so the PR is not approved yet.
	case pr.ReviewDecision == "REVIEW_REQUIRED":
		return "review"
	case approved:
		return "approved"
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

// unresolvedThreads counts review threads nobody has resolved yet. The query
// fetches at most 100 threads (the GraphQL maximum), so more are not counted.
func unresolvedThreads(pr ghPR) int {
	n := 0
	for _, t := range pr.ReviewThreads.Nodes {
		if !t.IsResolved {
			n++
		}
	}
	return n
}

// hasReviewer reports whether anyone other than the author was asked to
// review or has reviewed. The author's own replies create COMMENTED reviews,
// so they must not count.
func hasReviewer(pr ghPR) bool {
	if pr.ReviewRequests.TotalCount > 0 {
		return true
	}
	for _, r := range pr.LatestReviews.Nodes {
		// A deleted reviewer has no login; they are not someone to chase.
		if r.Author.Login != "" && r.Author.Login != pr.Author.Login {
			return true
		}
	}
	return false
}

// queueState is the merge-queue entry state, or "" when the PR isn't queued.
func queueState(pr ghPR) string {
	if pr.MergeQueueEntry == nil {
		return ""
	}
	return pr.MergeQueueEntry.State
}

func tierFor(pr ghPR, ci, merge, review string, unresolved int) int {
	switch {
	case pr.IsDraft:
		return tierDrafts
	// A queued PR is out of the user's hands unless the queue rejected it.
	case queueState(pr) == "UNMERGEABLE":
		return tierNeedsAction
	case pr.MergeQueueEntry != nil:
		return tierBuilding
	case ci == "fail" || merge == "conflict" || unresolved > 0:
		return tierNeedsAction
	case review == "approved" && merge == "clean":
		switch ci {
		case "pending":
			return tierBuilding
		case "ok":
			// Green and approved but GitHub still blocks the merge (e.g. a
			// required check or branch rule), so it is not ready for the user.
			if pr.MergeStateStatus == "BLOCKED" {
				return tierWaiting
			}
			return tierReady
		default:
			// No rollup: ready only when GitHub says nothing blocks the merge;
			// a non-CLEAN state means required checks just haven't reported yet.
			if pr.MergeStateStatus == "CLEAN" {
				return tierReady
			}
			return tierBuilding
		}
	case !hasReviewer(pr):
		return tierNoReviewer
	default:
		return tierWaiting
	}
}

// statusFor picks the one status a row shows. Within needs-action the order
// is: rejected by the merge queue, failed build, conflict, then unresolved
// comments.
func statusFor(tier int, ci, merge, queue string) string {
	switch tier {
	case tierNeedsAction:
		switch {
		case queue == "UNMERGEABLE":
			return "queue_failed"
		case ci == "fail":
			return "ci_failed"
		case merge == "conflict":
			return "conflict"
		default:
			return "comments"
		}
	case tierReady:
		return "ready"
	case tierBuilding:
		if queue != "" {
			return "queued"
		}
		return "building"
	case tierNoReviewer:
		return "no_reviewer"
	case tierWaiting:
		return "in_review"
	case tierDrafts:
		return "draft"
	default:
		return "merged"
	}
}

func classify(pr ghPR, now time.Time) Row {
	ci := ciCode(rollupState(pr))
	merge := mergeCode(pr.Mergeable)
	review := reviewCode(pr)
	unresolved := unresolvedThreads(pr)
	tier := tierFor(pr, ci, merge, review, unresolved)
	queuePos := 0
	if pr.MergeQueueEntry != nil {
		queuePos = pr.MergeQueueEntry.Position
	}

	return Row{
		Tier:       tier,
		Status:     statusFor(tier, ci, merge, queueState(pr)),
		QueuePos:   queuePos,
		Unresolved: unresolved,
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
		Status:     "merged",
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
