package main

import (
	"fmt"
	"strconv"
	"strings"
)

var tierNames = []string{
	"Needs action — CI fail / conflict",
	"Ready to merge",
	"Waiting on CI",
	"Waiting on review",
	"Drafts",
	"Recently merged — last 10",
}

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

// truncate and the pad helpers count runes, not bytes, so multibyte titles
// never get sliced mid-character.
func truncate(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	if w <= 1 {
		return string(r[:max(w, 0)])
	}
	return string(r[:w-1]) + "…"
}

func padRight(s string, w int) string {
	if n := w - len([]rune(s)); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

func padLeft(s string, w int) string {
	if n := w - len([]rune(s)); n > 0 {
		return strings.Repeat(" ", n) + s
	}
	return s
}

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

func mergeColor(code string, p palette) string {
	switch code {
	case "conflict":
		return p.r
	case "unknown":
		return p.y
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

// renderTerminal produces the aligned, grouped table. Padding is applied to the
// plain text before color so ANSI escapes never throw off column widths.
func renderTerminal(rows []Row, width int, color bool) string {
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

	// Title column is as wide as the longest title, but no wider than the space
	// left once the fixed columns and the URL are accounted for — so the URL sits
	// right after the titles rather than flush against a (possibly mis-detected)
	// right edge. Avoids both a huge gap on wide terminals and overflow on narrow.
	const prefixW = 2 + 1 + 2 + 8 + 2 + 4 + 2 // indent + ci + merge(8) + idle(4) + double-spaces
	longest, urlW, longestRepo := 0, 0, 0
	titles := make([]string, len(rows))
	for i, r := range rows {
		t := r.Title
		if r.Comments > 0 {
			t += " (" + strconv.Itoa(r.Comments) + ")"
		}
		titles[i] = t
		if n := len([]rune(t)); n > longest {
			longest = n
		}
		if n := len([]rune(r.URL)); n > urlW {
			urlW = n
		}
		if n := len([]rune(r.Repository)); n > longestRepo {
			longestRepo = n
		}
	}
	repoW := longestRepo
	titleW := width - prefixW - 2 - 3 - urlW - repoW
	if titleW > longest {
		titleW = longest
	}
	if titleW < 12 {
		titleW = 12
	}

	headColor := []string{p.r, p.g, p.y, p.c, p.d, p.d}
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
				name = fmt.Sprintf("%s (%d)", tierNames[tier], counts[tier])
			}
			color := p.d
			if tier >= 0 && tier < len(headColor) {
				color = headColor[tier]
			}
			b.WriteString(p.b + color + name + p.z + "\n")
		}

		title := padRight(truncate(titles[i], titleW), titleW)
		merge := mergeColor(r.Merge, p) + padRight(r.Merge, 8) + p.z
		idle := idleColor(r.IdleDays, p) + padLeft(strconv.Itoa(r.IdleDays)+"d", 4) + p.z
		repo := p.d + padRight(truncate(r.Repository, repoW), repoW) + p.z
		url := p.d + r.URL + p.z

		fmt.Fprintf(&b, "  %s  %s  %s  %s  %s   %s\n",
			ciGlyph(r.CI, p), merge, idle, repo, title, url)
	}
	fmt.Fprintf(&b, "\n%s%d open · %d need action · %d ready · %d merged%s\n",
		p.d, len(rows)-counts[tierMerged], counts[tierNeedsAction], counts[tierReady], counts[tierMerged], p.z)
	return b.String()
}
