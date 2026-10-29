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
	"Waiting on CI / queue",
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

// tierEmoji ends each group heading, indexed by tier.
var tierEmoji = []string{"🚨", "✅", "⏳", "🙋", "👀", "📝", "🎉"}

// ciGlyph is one cell wide, so it never disturbs column alignment.
func ciGlyph(code string, p palette) string {
	switch code {
	case "ok":
		return p.g + "✓" + p.z
	case "fail":
		return p.r + "✗" + p.z
	case "pending":
		return p.y + "•" + p.z
	default:
		return p.d + "·" + p.z
	}
}

// emojiW is the display width of the trailing status emoji. Every emoji used
// below is two cells wide without a variation selector, so terminals agree.
const emojiW = 2

// statusEmoji is the emoji for a needs-action row's reason, chosen by its
// status code.
func statusEmoji(status string) string {
	switch status {
	case "ci_failed":
		return "🔴"
	case "conflict":
		return "🚧"
	case "comments":
		return "💬"
	case "queue_failed":
		return "🚫"
	case "queued":
		return "🚦"
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

// showStatus reports whether a row ends with its own status: only needs-action
// rows do, since their reason varies while other groups are named by the
// heading.
func showStatus(r Row) bool { return r.Tier == tierNeedsAction }

// queueCell sits right after the CI glyph; the position varies per row.
func queueCell(r Row) string {
	if r.Status != "queued" {
		return ""
	}
	return statusEmoji("queued") + " #" + strconv.Itoa(r.QueuePos)
}

func statusText(r Row) string {
	switch r.Status {
	case "ci_failed":
		return "CI failed"
	case "conflict":
		return "conflict"
	case "queue_failed":
		return "queue failed"
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
	case "ci_failed", "conflict", "queue_failed":
		return p.r
	case "comments", "building", "queued", "no_reviewer":
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
	longest, urlW, longestRepo, statusW, queueW := 0, 0, 0, 0, 0
	for _, r := range rows {
		longest = max(longest, runewidth.StringWidth(r.Title))
		urlW = max(urlW, runewidth.StringWidth(r.URL))
		longestRepo = max(longestRepo, runewidth.StringWidth(r.Repository))
		queueW = max(queueW, runewidth.StringWidth(queueCell(r)))
		if showStatus(r) {
			statusW = max(statusW, runewidth.StringWidth(statusText(r)))
		}
	}
	// Needs-action rows end with an emoji and label; the URL is padded so
	// those line up.
	trailW, urlPad := 0, 0
	if statusW > 0 {
		trailW = 2 + emojiW + 1 + statusW
		urlPad = urlW
	}
	// The queue column exists only when some row is queued.
	queueCol := 0
	if queueW > 0 {
		queueCol = queueW + 2
	}
	prefixW := 1 + numW + 2 + 1 + 2 + queueCol + idleW + 2
	repoW := longestRepo
	titleW := width - prefixW - 2 - 3 - urlW - repoW - trailW
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
				name = fmt.Sprintf("%s (%d) %s", tierNames[tier], count, tierEmoji[tier])
			}
			color := p.d
			if tier >= 0 && tier < len(headColor) {
				color = headColor[tier]
			}
			b.WriteString(p.b + color + name + p.z + "\n")
		}

		num := p.d + padRight(strconv.Itoa(i+1)+".", numW) + p.z
		title := padRight(truncate(r.Title, titleW), titleW)
		queue := ""
		if queueCol > 0 {
			queue = statusColor("queued", p) + padRight(queueCell(r), queueW) + p.z + "  "
		}
		status := ""
		if showStatus(r) {
			status = "  " + padRight(statusEmoji(r.Status), emojiW) + " " + statusColor(r.Status, p) + statusText(r) + p.z
		}
		idle := idleField(r, p)
		repo := p.d + padRight(truncate(r.Repository, repoW), repoW) + p.z
		url := p.d + padRight(r.URL, urlPad) + p.z

		fmt.Fprintf(&b, " %s  %s  %s%s  %s  %s   %s%s\n",
			num, ciGlyph(r.CI, p), queue, idle, repo, title, url, status)
	}
	fmt.Fprintf(&b, "\n%s%d open · %d need action · %d ready · %d need reviewer · %d merged%s\n",
		p.d, len(rows)-counts[tierMerged], counts[tierNeedsAction], counts[tierReady], counts[tierNoReviewer], mergedShown, p.z)
	return b.String()
}
