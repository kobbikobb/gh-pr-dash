package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mattn/go-runewidth"
)

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
// multibyte and double-width (CJK, emoji) titles neither get sliced mid-character
// nor throw off column alignment.
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

// headTitle is the plain panel title: "Name (N) icon".
func headTitle(tier int, counts []int, mergedShown int) string {
	if tier < 0 || tier >= len(categories) {
		return "?"
	}
	count := counts[tier]
	if tier == tierMerged {
		count = mergedShown
	}
	c := categories[tier]
	return fmt.Sprintf("%s (%d) %s", c.name, count, c.icon)
}

// renderTerminal produces the aligned, grouped table. mergedTotal is the real
// count of PRs merged today (from the search issueCount), so the merged
// heading can state the total even when only mergedLimit rows are listed.
// Padding is applied to the plain text before color so ANSI escapes never
// throw off column widths.
func renderTerminal(rows []Row, width int, color bool, mergedTotal int) string {
	return renderPanels(rows, width, color, mergedTotal, view{})
}

// view is the interactive state a render needs: which row the cursor is on
// (by URL, so it survives refreshes) and which categories are folded away.
// The zero value renders the plain one-shot table.
type view struct {
	cursor      string
	collapsed   map[int]bool
	interactive bool
}

func renderPanels(rows []Row, width int, color bool, mergedTotal int, v view) string {
	if len(rows) == 0 {
		return "No open PRs.\n"
	}
	p := colors(color)

	counts := make([]int, len(categories))
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
	const markW = 2 // cursor marker and its space
	statusW := statusWidth(rows)
	contentW := width - panelOverhead
	prefixW := markW + statusW + 2 + idleW + 2
	longest, urlW, longestRepo := 0, 0, 0
	titles := make([]string, len(rows))
	for i, r := range rows {
		t := r.Title
		if r.Comments > 0 {
			t += " (" + strconv.Itoa(r.Comments) + ")"
		}
		titles[i] = t
		if n := runewidth.StringWidth(t); n > longest {
			longest = n
		}
		if n := runewidth.StringWidth(r.URL); n > urlW {
			urlW = n
		}
		if n := runewidth.StringWidth(r.Repository); n > longestRepo {
			longestRepo = n
		}
	}
	repoW := longestRepo
	titleW := contentW - prefixW - 2 - 3 - urlW - repoW
	if titleW > longest {
		titleW = longest
	}
	if titleW < 12 {
		titleW = 12
	}

	// The URL and the minimum title are never shrunk, so on a very narrow
	// terminal the panel grows to fit its rows rather than mis-aligning borders.
	width = max(width, panelOverhead+prefixW+repoW+2+titleW+3+urlW)

	var b strings.Builder
	tier := -1
	var border string
	for i, r := range rows {
		if r.Tier != tier {
			if tier != -1 {
				b.WriteString(panelBottom(width, border, p) + "\n\n")
			}
			tier = r.Tier
			desc := ""
			border = p.d
			if tier >= 0 && tier < len(categories) {
				desc = categories[tier].desc
				border = categories[tier].color(p)
			}
			title := headTitle(tier, counts, mergedShown)
			if v.interactive {
				title += " " + chevron(v.collapsed[tier])
			}
			b.WriteString(panelTop(title, desc, width, border, p) + "\n")
		}

		mark := "  "
		if v.cursor != "" && r.URL == v.cursor {
			mark = p.b + p.c + "❯" + p.z + " "
		}
		if v.collapsed[tier] {
			continue
		}
		title := padRight(truncate(titles[i], titleW), titleW)
		status := statusField(r, statusW, p)
		idle := idleField(r, p)
		repo := p.d + padRight(truncate(r.Repository, repoW), repoW) + p.z
		url := p.d + r.URL + p.z

		line := fmt.Sprintf("%s%s  %s  %s  %s   %s", mark, status, idle, repo, title, url)
		plainW := prefixW + repoW + 2 + titleW + 3 + len([]rune(r.URL))
		b.WriteString(panelLine(line, plainW, width, border, p) + "\n")
	}
	b.WriteString(panelBottom(width, border, p) + "\n")
	fmt.Fprintf(&b, "\n%s%d open · %d need action · %d ready · %d need reviewer · %d merged%s\n",
		p.d, len(rows)-counts[tierMerged], counts[tierNeedsAction], counts[tierReady], counts[tierNoReviewer], mergedShown, p.z)
	return b.String()
}

func chevron(collapsed bool) string {
	if collapsed {
		return "▸"
	}
	return "▾"
}
