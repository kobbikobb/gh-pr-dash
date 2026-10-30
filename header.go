package main

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

const appName = "gh pr-dash"

// dashArt is the 3-row wordmark.
var dashArt = []string{
	"╔═╗╦═╗   ╔╦╗╔═╗╔═╗╦ ╦",
	"╠═╝╠╦╝ ─  ║║╠═╣╚═╗╠═╣",
	"╩  ╩╚═   ═╩╝╩ ╩╚═╝╩ ╩",
}

// borderColor cycles the frame through cyan→yellow→green.
func borderColor(tick int, p palette) string {
	return []string{p.c, p.y, p.g}[(tick/3)%3]
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

// renderHeader draws the wordmark inside a box. The border cycles colors and the
// wordmark sweeps a color wave.
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
	// The wordmark sweeps through these colors left to right as tick advances.
	wave := []string{p.c, p.g, p.y, p.g}

	var b strings.Builder
	edge := border + "│" + p.z

	b.WriteString(border + "╭" + strings.Repeat("─", w-2) + "╮" + p.z + "\n")

	for _, row := range dashArt {
		var art strings.Builder
		vis := 0
		for i, ch := range []rune(row) {
			vis++
			if ch == ' ' {
				art.WriteRune(ch)
				continue
			}
			art.WriteString(wave[(tick+i/3)%len(wave)] + p.b + string(ch) + p.z)
		}
		pad := max(w-2-2-vis, 0)
		b.WriteString(edge + "  " + art.String() + strings.Repeat(" ", pad) + edge + "\n")
	}

	// Two spaces of margin inside each edge; the gap between left and right
	// is plain spaces so ANSI codes never skew the frame width.
	info = info.fit(w - 2 - 4)
	gap := max(w-2-4-runewidth.StringWidth(info.left)-runewidth.StringWidth(info.right), 1)
	b.WriteString(edge + "  " + p.d + info.left + p.z + strings.Repeat(" ", gap) + p.d + info.right + p.z + "  " + edge + "\n")

	b.WriteString(border + "╰" + strings.Repeat("─", w-2) + "╯" + p.z + "\n")
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
