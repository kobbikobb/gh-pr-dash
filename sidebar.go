package main

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

const (
	sidebarWidth = 30
	sidebarGap   = 2
	// Below this total width the panels need all the room, so no sidebar.
	sidebarMinTotal = 130
)

// statusOrder is the legend order, most urgent first.
var statusOrder = []string{
	"queue_failed", "ci_failed", "conflict", "comments", "ready", "queued",
	"building", "no_reviewer", "in_review", "draft", "merged",
}

// renderSidebar is a boxed legend: every status icon and each category with
// its current count.
func renderSidebar(rows []Row, p palette) []string {
	line := func(plain, styled string) string {
		return panelLine(styled, runewidth.StringWidth(plain), sidebarWidth, p.d, p)
	}
	section := func(s string) string { return line(s, p.b+s+p.z) }

	counts := make([]int, len(categories))
	for _, r := range rows {
		if r.Tier >= 0 && r.Tier < len(counts) {
			counts[r.Tier]++
		}
	}

	out := []string{panelTop("Legend", "", sidebarWidth, p.d, p), section("Status")}
	for _, code := range statusOrder {
		s := statuses[code]
		out = append(out, line(s.icon+" "+s.label, s.color(p)+s.icon+" "+s.label+p.z))
	}
	out = append(out, line("", ""), section("Categories"))
	for i, c := range categories {
		n := strconv.Itoa(counts[i])
		label := c.icon + " " + c.name
		gap := max(sidebarWidth-panelOverhead-runewidth.StringWidth(label)-len(n), 1)
		plain := label + strings.Repeat(" ", gap) + n
		out = append(out, line(plain, c.color(p)+label+p.z+strings.Repeat(" ", gap)+n))
	}
	return append(out, panelBottom(sidebarWidth, p.d, p))
}

// withSidebar places the sidebar to the right of the body, top-aligned. Width
// is measured ignoring ANSI so colored lines pad correctly.
func withSidebar(body string, side []string, bodyW int) string {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	for i := 0; i < max(len(lines), len(side)); i++ {
		l := ""
		if i < len(lines) {
			l = lines[i]
		}
		if i >= len(lines) {
			lines = append(lines, "")
		}
		pad := strings.Repeat(" ", max(bodyW+sidebarGap-lipgloss.Width(l), 0))
		if i < len(side) {
			lines[i] = l + pad + side[i]
		} else {
			lines[i] = l
		}
	}
	return strings.Join(lines, "\n") + "\n"
}
