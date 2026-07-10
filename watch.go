package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func watchLoop(opts options) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sig)

	fmt.Print("\033[?1049h\033[?25l")
	defer fmt.Print("\033[?25h\033[?1049l")

	dataTicker := time.NewTicker(opts.interval)
	animTicker := time.NewTicker(200 * time.Millisecond)
	defer dataTicker.Stop()
	defer animTicker.Stop()

	tick := 0
	lastWidth := 0
	var cachedRows []Row
	var errMsg string
	var lastFetch, nextRefresh time.Time

	fetch := func() {
		prs, err := fetchPRs(opts.limit)
		nextRefresh = time.Now().Add(opts.interval)
		if err != nil {
			errMsg = err.Error()
			return
		}
		cachedRows = buildRows(prs, opts.org, time.Now())
		lastFetch = time.Now()
		errMsg = ""
	}

	fetch()

	for {
		// Detect the size every frame so a resize reflows immediately; a full
		// clear on width change wipes the wrapped residue \033[J can't reach.
		t := detectTerminal()
		if t.width != lastWidth {
			fmt.Print("\033[2J")
			lastWidth = t.width
		}
		renderWatch(opts, tick, cachedRows, errMsg, refreshStatus(lastFetch, nextRefresh), t)

		select {
		case <-sig:
			return
		case <-dataTicker.C:
			fetch()
			tick++
		case <-animTicker.C:
			// Just animate (tick increments, no data fetch)
			tick++
		}
	}
}

func refreshStatus(lastFetch, nextRefresh time.Time) string {
	if lastFetch.IsZero() {
		return ""
	}
	secs := int(time.Until(nextRefresh).Seconds())
	if secs < 0 {
		secs = 0
	}
	return fmt.Sprintf("refreshed %s · next %ds", lastFetch.Format("15:04"), secs)
}

func renderWatch(opts options, tick int, rows []Row, errMsg, status string, t terminal) {
	fmt.Print("\033[H")
	p := colors(t.useColor)
	out := renderHeader(tick, true, t.useColor, t.width, t.height, status)
	if errMsg != "" {
		out += p.r + "  ⚠ " + errMsg + p.z + "\n\n"
	}
	if !opts.asJSON {
		out += renderTerminal(rows, t.width, t.useColor)
	}
	_, _ = fmt.Fprint(os.Stdout, out)
	fmt.Print("\033[J")
}
