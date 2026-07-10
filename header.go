package main

import (
	"strings"
	"time"
)

var spinner = [9]rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇'}

var dashArt = []string{
	`▄▄▄  ▄  ▄ ▄▄▄ ▄  ▄ ▄▄▄ ▄  ▄ ▄  ▄`,
	`█▄▄  █▄█   █   █▄█   █ █▄▄█ █▄▄█`,
	`▀▀▀  ▀ ▀ ▀▀▀ ▀▀▀ ▀▀▀ ▀ ▀ ▀ ▀  ▀`,
}

func repeat(ch rune, n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(string(ch), n)
}

func renderHeader(tick int, watch bool, color bool, width int) string {
	p := colors(color)

	if !watch {
		return renderStaticHeader(p, width)
	}
	return renderWatchHeader(tick, p, width)
}

func renderStaticHeader(p palette, width int) string {
	if width < 40 {
		width = 40
	}
	w := width

	var b strings.Builder

	// Top border
	b.WriteString(p.c + "╔" + repeat('═', w-2) + "╗" + p.z + "\n")

	// Empty line
	b.WriteString(p.c + "║" + repeat(' ', w-2) + "║" + p.z + "\n")

	// ASCII art lines - centered
	for _, line := range dashArt {
		lineW := len([]rune(line))
		leftPad := (w - 2 - lineW) / 2
		rightPad := w - 2 - lineW - leftPad
		b.WriteString(p.c + "║" + p.z)
		b.WriteString(repeat(' ', leftPad))
		b.WriteString(p.b + line + p.z)
		b.WriteString(repeat(' ', rightPad))
		b.WriteString(p.c + "║" + p.z + "\n")
	}

	// Empty line
	b.WriteString(p.c + "║" + repeat(' ', w-2) + "║" + p.z + "\n")

	// Subtitle
	sub := "by kobbikobb"
	subW := len([]rune(sub))
	leftPad := (w - 2 - subW) / 2
	rightPad := w - 2 - subW - leftPad
	b.WriteString(p.c + "║" + p.z)
	b.WriteString(repeat(' ', leftPad))
	b.WriteString(p.d + sub + p.z)
	b.WriteString(repeat(' ', rightPad))
	b.WriteString(p.c + "║" + p.z + "\n")

	// Bottom border
	b.WriteString(p.c + "╚" + repeat('═', w-2) + "╝" + p.z + "\n")

	return b.String() + "\n"
}

func renderWatchHeader(tick int, p palette, width int) string {
	if width < 40 {
		width = 40
	}
	w := width

	spin := spinner[tick%len(spinner)]
	now := time.Now().Format("15:04:05")

	// Border color cycles
	borderColors := []string{p.c, p.y, p.g}
	bc := borderColors[(tick/2)%len(borderColors)]

	// Art line colors cycle independently
	artColors := []string{p.r, p.y, p.g, p.c}

	var b strings.Builder

	// Top border
	b.WriteString(bc + "╔" + repeat('═', w-2) + "╗" + p.z + "\n")

	// Empty line
	b.WriteString(bc + "║" + repeat(' ', w-2) + "║" + p.z + "\n")

	// ASCII art lines - centered with color animation
	for i, line := range dashArt {
		lineW := len([]rune(line))
		leftPad := (w - 2 - lineW) / 2
		rightPad := w - 2 - lineW - leftPad
		lc := artColors[(tick+i)%len(artColors)]
		b.WriteString(bc + "║" + p.z)
		b.WriteString(repeat(' ', leftPad))
		b.WriteString(lc + p.b + line + p.z)
		b.WriteString(repeat(' ', rightPad))
		b.WriteString(bc + "║" + p.z + "\n")
	}

	// Empty line
	b.WriteString(bc + "║" + repeat(' ', w-2) + "║" + p.z + "\n")

	// Status line: spinner on left, timestamp on right
	statusW := w - 4
	left := string(spin) + " "
	right := now
	mid := statusW - len([]rune(left)) - len([]rune(right))
	if mid < 1 {
		mid = 1
	}
	b.WriteString(bc + "║" + p.z)
	b.WriteString(p.g + left + p.z)
	b.WriteString(repeat(' ', mid))
	b.WriteString(p.d + right + p.z)
	b.WriteString(bc + "║" + p.z + "\n")

	// Bottom border
	b.WriteString(bc + "╚" + repeat('═', w-2) + "╝" + p.z + "\n")

	return b.String() + "\n"
}
