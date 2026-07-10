package main

import (
	"fmt"
	"time"
)

var spinner = [9]rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇'}

func renderHeader(tick int, watch bool, color bool) string {
	p := colors(color)

	if !watch {
		return p.b + "gh pr-dash" + p.z + "\n\n"
	}

	spin := spinner[tick%len(spinner)]
	now := time.Now().Format("15:04:05")

	return fmt.Sprintf("%s%c%s %sgh pr-dash%s  %s%s%s\n\n",
		p.c, spin, p.z,
		p.b, p.z,
		p.d, now, p.z,
	)
}
