package main

import (
	"strings"
	"time"
)

var spinner = [9]rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇'}

var dashArt = []string{
	` ▄▄▄  ▄  ▄ ▄▄▄ ▄  ▄ ▄▄▄ ▄  ▄ ▄  ▄`,
	` █▄▄  █▄█   █   █▄█   █ █▄▄█ █▄▄█`,
	` ▀▀▀  ▀ ▀ ▀▀▀ ▀▀▀ ▀▀▀ ▀ ▀ ▀ ▀  ▀`,
}

const borderChars = "═══════════════════════════════════════════════════════════════"

func centerPad(s string, width int) string {
	runeCount := len([]rune(s))
	if runeCount >= width {
		return s
	}
	pad := (width - runeCount) / 2
	return strings.Repeat(" ", pad) + s + strings.Repeat(" ", width-runeCount-pad)
}

func renderHeader(tick int, watch bool, color bool, width int) string {
	p := colors(color)

	if !watch {
		return renderStaticHeader(p, width)
	}
	return renderWatchHeader(tick, p, width)
}

func renderStaticHeader(p palette, width int) string {
	var b strings.Builder

	border := truncate(borderChars, width-2)
	b.WriteString(p.c + "╔" + border + "╗" + p.z + "\n")
	b.WriteString(p.c + "║" + centerPad("", width-2) + "║" + p.z + "\n")
	for _, line := range dashArt {
		b.WriteString(p.c + "║" + p.z + p.b + centerPad(line, width-2) + p.z + p.c + "║" + p.z + "\n")
	}
	b.WriteString(p.c + "║" + centerPad("", width-2) + "║" + p.z + "\n")
	b.WriteString(p.c + "║" + p.z + p.d + centerPad("by kobbikobb", width-2) + p.z + p.c + "║" + p.z + "\n")
	b.WriteString(p.c + "╚" + border + "╝" + p.z + "\n")

	return b.String() + "\n"
}

func renderWatchHeader(tick int, p palette, width int) string {
	spin := spinner[tick%len(spinner)]
	now := time.Now().Format("15:04:05")

	border := truncate(borderChars, width-2)

	// Animate border color: cycle through cyan, yellow, green
	borderColors := []string{p.c, p.y, p.g}
	bc := borderColors[(tick/2)%len(borderColors)]

	var b strings.Builder

	b.WriteString(bc + "╔" + border + "╗" + p.z + "\n")
	b.WriteString(bc + "║" + centerPad("", width-2) + "║" + p.z + "\n")
	for i, line := range dashArt {
		// Animate each line of the art with a color shift
		lineColors := []string{p.r, p.y, p.g, p.c}
		lc := lineColors[(tick+i)%len(lineColors)]
		b.WriteString(bc + "║" + p.z + lc + p.b + centerPad(line, width-2) + p.z + bc + "║" + p.z + "\n")
	}
	b.WriteString(bc + "║" + centerPad("", width-2) + "║" + p.z + "\n")

	// Animated status line
	statusBar := renderStatusBar(spin, now, width-4)
	b.WriteString(bc + "║" + p.z + statusBar + bc + "║" + p.z + "\n")

	b.WriteString(bc + "╚" + border + "╝" + p.z + "\n")

	return b.String() + "\n"
}

func renderStatusBar(spin rune, timestamp string, width int) string {
	spinStr := string(spin)
	left := spinStr + " "
	right := timestamp
	middle := width - len([]rune(left)) - len([]rune(right))
	if middle < 1 {
		middle = 1
	}
	return left + strings.Repeat(" ", middle) + right
}
