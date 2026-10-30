package main

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

const appName = "gh pr-dash"

// glyphs is a 5-row, 4-column block font. dashArt spells the wordmark from it.
var glyphs = map[rune][5]string{
	'P': {"███ ", "█  █", "███ ", "█   ", "█   "},
	'R': {"███ ", "█  █", "███ ", "█ █ ", "█  █"},
	'-': {"    ", "    ", " ██ ", "    ", "    "},
	'D': {"███ ", "█  █", "█  █", "█  █", "███ "},
	'A': {" ██ ", "█  █", "████", "█  █", "█  █"},
	'S': {" ███", "█   ", " ██ ", "   █", "███ "},
	'H': {"█  █", "█  █", "████", "█  █", "█  █"},
}

var dashArt = banner("PR-DASH")

// borderColor cycles the frame through cyan→yellow→green.
func borderColor(tick int, p palette) string {
	return []string{p.c, p.y, p.g}[(tick/3)%3]
}

func banner(s string) []string {
	rows := make([]string, 5)
	for i := range rows {
		parts := make([]string, 0, len(s))
		for _, ch := range s {
			parts = append(parts, glyphs[ch][i])
		}
		rows[i] = strings.Join(parts, " ")
	}
	return rows
}

// headerInfo is the line under the wordmark: what is being
// shown on the left (org/repo scope), the clock and refresh countdown on the
// right.
type headerInfo struct{ left, right string }

// fit shrinks info so left, a one-space gap and right fit in avail cells: the
// scope is truncated first, then dropped, then the right side is truncated.
func (i headerInfo) fit(avail int) headerInfo {
	rw := runewidth.StringWidth(i.right)
	if runewidth.StringWidth(i.left)+1+rw <= avail {
		return i
	}
	if room := avail - rw - 1; room >= 1 {
		return headerInfo{runewidth.Truncate(i.left, room, "…"), i.right}
	}
	return headerInfo{"", runewidth.Truncate(i.right, max(avail, 0), "…")}
}

// renderHeader draws the wordmark inside a box. The border and each art row
// cycle colors for a wave.
// Padding is applied to plain runes so ANSI codes never skew the frame width.
// On short terminals the box would eat the view, so it collapses to one line.
// info is the scope/clock line under the wordmark.
func renderHeader(tick int, color bool, width, height int, info headerInfo) string {
	if height < 16 || width < 40 {
		return renderCompactHeader(tick, color, width, info)
	}

	p := colors(color)
	w := width

	border := borderColor(tick, p)
	art := []string{p.r, p.y, p.g, p.c}
	artColorAt := func(i int) string { return art[(tick+i)%len(art)] }

	var b strings.Builder
	edge := border + "║" + p.z

	center := func(vis int, colored string) {
		left := (w - 2 - vis) / 2
		if left < 0 {
			left = 0
		}
		right := w - 2 - vis - left
		if right < 0 {
			right = 0
		}
		b.WriteString(edge + strings.Repeat(" ", left) + colored + strings.Repeat(" ", right) + edge + "\n")
	}

	b.WriteString(border + "╔" + strings.Repeat("═", w-2) + "╗" + p.z + "\n")
	b.WriteString(edge + strings.Repeat(" ", w-2) + edge + "\n")

	for i, row := range dashArt {
		center(len([]rune(row)), artColorAt(i)+p.b+row+p.z)
	}

	// Two spaces of margin inside each edge; the gap between left and right
	// is plain spaces so ANSI codes never skew the frame width.
	info = info.fit(w - 2 - 4)
	gap := max(w-2-4-runewidth.StringWidth(info.left)-runewidth.StringWidth(info.right), 1)
	b.WriteString(edge + "  " + p.d + info.left + p.z + strings.Repeat(" ", gap) + p.d + info.right + p.z + "  " + edge + "\n")

	b.WriteString(border + "╚" + strings.Repeat("═", w-2) + "╝" + p.z + "\n")
	return b.String() + "\n"
}

func renderCompactHeader(tick int, color bool, width int, info headerInfo) string {
	p := colors(color)
	if width < 20 {
		width = 20
	}

	border := borderColor(tick, p)
	info = info.fit(max(width-runewidth.StringWidth(appName)-2-2-1, 0))
	text := strings.TrimSpace(info.left + " " + info.right)
	right := "  " + p.d + text + p.z
	rightVis := 2 + runewidth.StringWidth(text)

	name := " " + appName + " "
	fill := width - len([]rune(name)) - rightVis
	if fill < 0 {
		fill = 0
	}

	return border + p.b + name + p.z + border + strings.Repeat("─", fill) + p.z + right + "\n\n"
}
