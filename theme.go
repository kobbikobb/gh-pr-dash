package main

// category is how one tier presents itself: heading, one-line description,
// emoji and heading color. Indexed by tier, so it must stay in tier order.
type category struct {
	name  string
	desc  string
	icon  string
	color func(palette) string
}

// Icons are emoji that are two cells wide without a variation selector, so
// terminals agree on their width.
var categories = []category{
	{"Needs action", "PRs with failing checks, conflicts, comments or a rejected queue entry", "🚨", func(p palette) string { return p.r }},
	{"Ready to merge", "Approved, passing checks, clean, not blocked", "✅", func(p palette) string { return p.g }},
	{"In progress", "Approved, but CI pending or running, or in the merge queue", "⏳", func(p palette) string { return p.y }},
	{"Needs reviewer", "Nobody asked for or reviewed yet", "🙋", func(p palette) string { return p.y }},
	{"In review", "Reviewer requested, reviewed but not approved", "👀", func(p palette) string { return p.c }},
	{"Drafts", "Draft PRs, any CI state", "📝", func(p palette) string { return p.d }},
	{"Merged today", "Merged since midnight", "🎉", func(p palette) string { return p.d }},
}
