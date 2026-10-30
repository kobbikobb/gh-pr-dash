package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// panelOverhead is the columns a panel adds around its content: border and
// one space of padding on each side.
const panelOverhead = 4

var panelBorder = lipgloss.RoundedBorder()

// panelTop draws "╭─ title ─ description ───╮" at the given outer width. The
// description is dropped when it would not fit. title and desc are plain text;
// color wraps only the border so widths stay exact.
func panelTop(title, desc string, width int, color string, p palette) string {
	left := panelBorder.TopLeft + panelBorder.Top + " "
	titleCell := p.b + color + title + p.z
	used := runewidth.StringWidth(left) + runewidth.StringWidth(title) + 1
	descCell := ""
	if d := runewidth.StringWidth(desc); desc != "" && used+2+d+1+1 <= width {
		descCell = color + panelBorder.Top + p.z + " " + p.d + desc + p.z + " "
		used += 2 + d + 1
	}
	fill := max(width-used-1, 0)
	return color + left + p.z + titleCell + " " + descCell +
		color + strings.Repeat(panelBorder.Top, fill) + panelBorder.TopRight + p.z
}

func panelBottom(width int, color string, p palette) string {
	return color + panelBorder.BottomLeft + strings.Repeat(panelBorder.Bottom, max(width-2, 0)) + panelBorder.BottomRight + p.z
}

// panelLine wraps already-styled content whose plain display width is
// plainW, padding with plain spaces before the closing border.
func panelLine(content string, plainW, width int, color string, p palette) string {
	pad := strings.Repeat(" ", max(width-panelOverhead-plainW, 0))
	return color + panelBorder.Left + p.z + " " + content + pad + " " + color + panelBorder.Right + p.z
}
