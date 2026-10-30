package main

import (
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
)

// category is how one tier presents itself: heading, one-line description,
// emoji and heading color. Indexed by tier, so it must stay in tier order.
type category struct {
	name  string
	desc  string
	icon  string
	color func(palette) string
	agent string // prompt prefix when the whole category is sent to an agent
}

// Icons are emoji that are two cells wide without a variation selector, so
// terminals agree on their width.
var categories = []category{
	{"Needs action", "PRs with failing checks, conflicts, comments or a rejected queue entry", "🚨", func(p palette) string { return p.r }, "Please review and take action on PRs that need attention (CI failures, conflicts, unresolved comments):"},
	{"Ready to merge", "Approved, passing checks, clean, not blocked", "✅", func(p palette) string { return p.g }, "Please review and merge ready PRs (approved, green, no conflicts):"},
	{"In progress", "Approved, but CI pending or running, or in the merge queue", "⏳", func(p palette) string { return p.y }, "Please monitor PRs waiting on CI or in the merge queue:"},
	{"Needs reviewer", "Nobody asked for or reviewed yet", "🙋", func(p palette) string { return p.y }, "Please find reviewers for PRs with no reviewer:"},
	{"In review", "Reviewer requested, reviewed but not approved", "👀", func(p palette) string { return p.c }, "Please review PRs waiting for review:"},
	{"Drafts", "Draft PRs, any CI state", "📝", func(p palette) string { return p.d }, "Please review draft PRs:"},
	{"Merged today", "Merged since midnight", "🎉", func(p palette) string { return p.d }, "Please summarize recently merged PRs:"},
}

// statusInfo is how one Row.Status code is shown: emoji, short label, color.
type statusInfo struct {
	icon  string
	label string
	color func(palette) string
}

var statuses = map[string]statusInfo{
	"queue_failed": {"🚫", "Queue failed", func(p palette) string { return p.r }},
	"ci_failed":    {"❌", "CI failed", func(p palette) string { return p.r }},
	"conflict":     {"💥", "Conflicts", func(p palette) string { return p.r }},
	"comments":     {"💬", "Comments", func(p palette) string { return p.r }},
	"ready":        {"✅", "Ready", func(p palette) string { return p.g }},
	"queued":       {"🚦", "Queued", func(p palette) string { return p.y }},
	"building":     {"⏳", "Building", func(p palette) string { return p.y }},
	"no_reviewer":  {"🙋", "No reviewer", func(p palette) string { return p.y }},
	"in_review":    {"👀", "In review", func(p palette) string { return p.c }},
	"draft":        {"📝", "Draft", func(p palette) string { return p.d }},
	"merged":       {"🎉", "Merged", func(p palette) string { return p.d }},
}

// statusLabel is the plain text of a row's status, with the queue position
// appended for queued PRs.
func statusLabel(r Row) string {
	s, ok := statuses[r.Status]
	if !ok {
		return "? " + r.Status
	}
	label := s.icon + " " + s.label
	if r.Status == "queued" && r.QueuePos > 0 {
		label += " #" + strconv.Itoa(r.QueuePos)
	}
	return label
}

// statusField is the padded, colored status cell. Padding uses display width
// because emoji are two cells wide.
func statusField(r Row, w int, p palette) string {
	label := statusLabel(r)
	pad := strings.Repeat(" ", max(w-runewidth.StringWidth(label), 0))
	if s, ok := statuses[r.Status]; ok {
		return s.color(p) + label + p.z + pad
	}
	return p.d + label + p.z + pad
}

// statusWidth is the widest status label, so the column aligns.
func statusWidth(rows []Row) int {
	w := 0
	for _, r := range rows {
		w = max(w, runewidth.StringWidth(statusLabel(r)))
	}
	return w
}
