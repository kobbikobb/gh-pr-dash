package main

import (
	"strings"
	"time"
)

var spinner = [9]rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇'}

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

// renderHeader draws the wordmark inside a box. Under --watch the border and
// each art row cycle colors for a wave; otherwise it renders in one steady color.
// Padding is applied to plain runes so ANSI codes never skew the frame width.
// On short terminals the box would eat the view, so it collapses to one line.
func renderHeader(tick int, watch bool, color bool, width, height int) string {
	if height < 16 {
		return renderCompactHeader(tick, watch, color, width)
	}

	p := colors(color)
	if width < 40 {
		width = 40
	}
	w := width

	border := p.c
	artColorAt := func(int) string { return p.c }
	if watch {
		bc := []string{p.c, p.y, p.g}
		border = bc[(tick/3)%len(bc)]
		art := []string{p.r, p.y, p.g, p.c}
		artColorAt = func(i int) string { return art[(tick+i)%len(art)] }
	}

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

	if watch {
		spin := string(spinner[tick%len(spinner)])
		clock := time.Now().Format("15:04:05")
		mid := w - 2 - 4 - (len(clock) + 2)
		if mid < 1 {
			mid = 1
		}
		b.WriteString(edge + "  " + p.g + spin + p.z + " " + strings.Repeat(" ", mid) + p.d + clock + p.z + "  " + edge + "\n")
	} else {
		center(len(appName), p.d+appName+p.z)
	}

	b.WriteString(border + "╚" + strings.Repeat("═", w-2) + "╝" + p.z + "\n")
	return b.String() + "\n"
}

func renderCompactHeader(tick int, watch bool, color bool, width int) string {
	p := colors(color)
	if width < 20 {
		width = 20
	}

	border := p.c
	right, rightVis := "", 0
	if watch {
		border = []string{p.c, p.y, p.g}[(tick/3)%3]
		spin := string(spinner[tick%len(spinner)])
		clock := time.Now().Format("15:04:05")
		right = "  " + p.g + spin + p.z + " " + p.d + clock + p.z
		rightVis = 2 + 1 + 1 + len(clock)
	}

	name := " " + appName + " "
	fill := width - len([]rune(name)) - rightVis
	if fill < 0 {
		fill = 0
	}

	return border + p.b + name + p.z + border + strings.Repeat("─", fill) + p.z + right + "\n\n"
}
