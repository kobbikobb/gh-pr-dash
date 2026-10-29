package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
)

var tierNames = []string{
	"Needs action — CI fail / conflict / comments",
	"Ready to merge",
	"Waiting on CI",
	"Needs reviewer",
	"Waiting on review",
	"Drafts",
	"Merged today",
}

// idleW fits "14:32" for merged rows and "14d" for open rows.
const idleW = 5

type palette struct{ b, d, r, g, y, c, z string }

func colors(on bool) palette {
	if !on {
		return palette{}
	}
	return palette{
		b: "\033[1m", d: "\033[2m", r: "\033[31m",
		g: "\033[32m", y: "\033[33m", c: "\033[36m", z: "\033[0m",
	}
}

// truncate and the pad helpers count display cells, not bytes or runes, so
// multibyte titles are never sliced mid-character and wide characters (emoji,
// CJK) keep columns aligned.
func truncate(s string, w int) string {
	if runewidth.StringWidth(s) <= w {
		return s
	}
	if w <= 1 {
		return runewidth.Truncate(s, max(w, 0), "")
	}
	return runewidth.Truncate(s, w, "…")
}

func padRight(s string, w int) string {
	if n := w - runewidth.StringWidth(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

func padLeft(s string, w int) string {
	if n := w - runewidth.StringWidth(s); n > 0 {
		return strings.Repeat(" ", n) + s
	}
	return s
}

// gutter is the horizontal margin on each side; dropped on narrow terminals so
// the compact layout keeps its full width.
func gutter(width int) int {
	if width < 40 {
		return 0
	}
	return 2
}

func indent(s string, n int) string {
	if n == 0 {
		return s
	}
	pad := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		if ln != "" {
			lines[i] = pad + ln
		}
	}
	return strings.Join(lines, "\n")
}

// tierEmoji heads each group so rows need not repeat what the heading says.
var tierEmoji = []string{"🚨", "✅", "⏳", "🙋", "👀", "📝", "🎉"}

// emojiW is the display width of the status emoji column. Every emoji used
// below is two cells wide without a variation selector, so terminals agree.
const emojiW = 2

// statusEmoji is the one emoji a row shows, chosen by its status code. Only
// needs-action rows show it: their reason varies, unlike every other group.
func statusEmoji(status string) string {
	switch status {
	case "ci_failed":
		return "🔴"
	case "conflict":
		return "🚧"
	case "comments":
		return "💬"
	case "ready":
		return "✅"
	case "building":
		return "⏳"
	case "no_reviewer":
		return "🙋"
	case "in_review":
		return "👀"
	case "draft":
		return "📝"
	case "merged":
		return "🎉"
	default:
		return ""
	}
}

func showStatus(r Row) bool { return r.Tier == tierNeedsAction }

func statusText(r Row) string {
	switch r.Status {
	case "ci_failed":
		return "CI failed"
	case "conflict":
		return "conflict"
	case "comments":
		if r.Unresolved == 1 {
			return "1 comment"
		}
		return strconv.Itoa(r.Unresolved) + " comments"
	case "ready":
		return "ready"
	case "building":
		return "CI running"
	case "no_reviewer":
		return "no reviewer"
	case "in_review":
		return "in review"
	case "draft":
		return "draft"
	case "merged":
		return "merged"
	default:
		return ""
	}
}

func statusColor(status string, p palette) string {
	switch status {
	case "ci_failed", "conflict":
		return p.r
	case "comments", "building", "no_reviewer":
		return p.y
	case "ready":
		return p.g
	case "merged":
		return p.c
	default:
		return p.d
	}
}

func idleColor(days int, p palette) string {
	switch {
	case days >= 14:
		return p.r
	case days >= 7:
		return p.y
	default:
		return p.d
	}
}

// idleField shows idle days for open PRs, but merged PRs are always 0d, so
// their column shows the merge time instead (local time).
func idleField(r Row, p palette) string {
	if r.MergedAt != nil {
		return p.d + padLeft(r.MergedAt.Local().Format("15:04"), idleW) + p.z
	}
	return idleColor(r.IdleDays, p) + padLeft(strconv.Itoa(r.IdleDays)+"d", idleW) + p.z
}

// renderTerminal produces the aligned, grouped table. mergedTotal is the real
// count of PRs merged today (from the search issueCount), so the merged
// heading can state the total even when only mergedLimit rows are listed.
// Padding is applied to the plain text before color so ANSI escapes never
// throw off column widths.
func renderTerminal(rows []Row, width int, color bool, mergedTotal int) string {
	if len(rows) == 0 {
		return "No open PRs.\n"
	}
	p := colors(color)

	counts := make([]int, len(tierNames))
	for _, r := range rows {
		if r.Tier >= 0 && r.Tier < len(counts) {
			counts[r.Tier]++
		}
	}
	// mergedShown is what the merged heading and footer agree on: the listed
	// rows, or the real merged-today total when more than mergedLimit are shown.
	mergedShown := counts[tierMerged]
	if mergedTotal > mergedShown {
		mergedShown = mergedTotal
	}

	// Title column is as wide as the longest title, but no wider than the space
	// left once the fixed columns and the URL are accounted for — so the URL sits
	// right after the titles rather than flush against a (possibly mis-detected)
	// right edge. Avoids both a huge gap on wide terminals and overflow on narrow.
	numW := len(strconv.Itoa(len(rows))) + 1 // digits + dot
	longest, urlW, longestRepo, statusW := 0, 0, 0, 0
	for _, r := range rows {
		longest = max(longest, runewidth.StringWidth(r.Title))
		urlW = max(urlW, runewidth.StringWidth(r.URL))
		longestRepo = max(longestRepo, runewidth.StringWidth(r.Repository))
		if showStatus(r) {
			statusW = max(statusW, runewidth.StringWidth(statusText(r)))
		}
	}
	// The status column exists only when some row shows it.
	statusCol := 0
	if statusW > 0 {
		statusCol = emojiW + 1 + statusW + 2
	}
	prefixW := 1 + numW + 2 + statusCol + idleW + 2
	repoW := longestRepo
	titleW := width - prefixW - 2 - 3 - urlW - repoW
	if titleW > longest {
		titleW = longest
	}
	if titleW < 12 {
		titleW = 12
	}

	headColor := []string{p.r, p.g, p.y, p.y, p.c, p.d, p.d}
	var b strings.Builder
	tier := -1
	for i, r := range rows {
		if r.Tier != tier {
			if tier != -1 {
				b.WriteString("\n")
			}
			tier = r.Tier
			name := "?"
			if tier >= 0 && tier < len(tierNames) {
				count := counts[tier]
				if tier == tierMerged {
					count = mergedShown
				}
				name = fmt.Sprintf("%s %s (%d)", tierEmoji[tier], tierNames[tier], count)
			}
			color := p.d
			if tier >= 0 && tier < len(headColor) {
				color = headColor[tier]
			}
			b.WriteString(p.b + color + name + p.z + "\n")
		}

		num := p.d + padRight(strconv.Itoa(i+1)+".", numW) + p.z
		title := padRight(truncate(r.Title, titleW), titleW)
		status := ""
		if statusCol > 0 {
			emoji, text := "", ""
			if showStatus(r) {
				emoji, text = statusEmoji(r.Status), statusText(r)
			}
			status = padRight(emoji, emojiW) + " " +
				statusColor(r.Status, p) + padRight(text, statusW) + p.z + "  "
		}
		idle := idleField(r, p)
		repo := p.d + padRight(truncate(r.Repository, repoW), repoW) + p.z
		url := p.d + r.URL + p.z

		fmt.Fprintf(&b, " %s  %s%s  %s  %s   %s\n",
			num, status, idle, repo, title, url)
	}
	fmt.Fprintf(&b, "\n%s%d open · %d need action · %d ready · %d need reviewer · %d merged%s\n",
		p.d, len(rows)-counts[tierMerged], counts[tierNeedsAction], counts[tierReady], counts[tierNoReviewer], mergedShown, p.z)
	return b.String()
}
