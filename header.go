package main

import (
	"fmt"
	"strings"
	"time"
)

var spinner = [9]rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇'}

var dashArt = []string{
	`  ▄▄▄  ▄  ▄ ▄▄▄ ▄  ▄ ▄▄▄ ▄  ▄ ▄  ▄`,
	`  █▄▄  █▄█   █   █▄█   █ █▄▄█ █▄▄█`,
	`  ▀▀▀  ▀ ▀ ▀▀▀ ▀▀▀ ▀▀▀ ▀ ▀ ▀ ▀  ▀`,
}

const (
	boxH  = "━"
	boxV  = "┃"
	boxTL = "┏"
	boxTR = "┓"
	boxBL = "┗"
	boxBR = "┛"
)

func boxLine(content string, width int) string {
	padding := width - 2 - len([]rune(content))
	if padding < 0 {
		padding = 0
	}
	return boxV + content + strings.Repeat(" ", padding) + boxV
}

func renderHeader(tick int, watch bool, color bool) string {
	p := colors(color)

	if !watch {
		return renderStaticHeader(p)
	}
	return renderWatchHeader(tick, p)
}

func renderStaticHeader(p palette) string {
	width := 44
	var b strings.Builder

	b.WriteString(p.c + boxTL + strings.Repeat(boxH, width-2) + boxTR + p.z + "\n")
	for _, line := range dashArt {
		b.WriteString(boxLine(p.b+line+p.z, width) + "\n")
	}
	b.WriteString(boxLine("", width) + "\n")
	b.WriteString(boxLine(p.d+"by kobbikobb"+p.z, width) + "\n")
	b.WriteString(p.c + boxBL + strings.Repeat(boxH, width-2) + boxBR + p.z + "\n")

	return b.String() + "\n"
}

func renderWatchHeader(tick int, p palette) string {
	width := 44
	spin := spinner[tick%len(spinner)]
	now := time.Now().Format("15:04:05")
	var b strings.Builder

	b.WriteString(p.y + boxTL + strings.Repeat(boxH, width-2) + boxTR + p.z + "\n")
	for _, line := range dashArt {
		b.WriteString(boxLine(p.b+line+p.z, width) + "\n")
	}
	b.WriteString(boxLine("", width) + "\n")

	status := fmt.Sprintf("%s%c%s %srefreshing%s  %s%s%s",
		p.g, spin, p.z,
		p.d, p.z,
		p.d, now, p.z,
	)
	b.WriteString(boxLine(status, width) + "\n")
	b.WriteString(p.y + boxBL + strings.Repeat(boxH, width-2) + boxBR + p.z + "\n")

	return b.String() + "\n"
}
