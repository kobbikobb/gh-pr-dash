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

	dataTicker := time.NewTicker(1 * time.Minute)
	animTicker := time.NewTicker(200 * time.Millisecond)
	defer dataTicker.Stop()
	defer animTicker.Stop()

	tick := 0
	var cachedRows []Row
	var cachedTerminal terminal

	// Initial fetch
	prs, err := fetchPRs(opts.limit)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	} else {
		cachedRows = buildRows(prs, opts.org, time.Now())
	}
	cachedTerminal = detectTerminal()

	for {
		// Always render with current tick (for animation)
		renderWatch(opts, tick, cachedRows, cachedTerminal)

		select {
		case <-sig:
			fmt.Println()
			return
		case <-dataTicker.C:
			// Refetch data
			prs, err := fetchPRs(opts.limit)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
			} else {
				cachedRows = buildRows(prs, opts.org, time.Now())
			}
			cachedTerminal = detectTerminal()
			tick++
		case <-animTicker.C:
			// Just animate (tick increments, no data fetch)
			tick++
		}
	}
}

func renderWatch(opts options, tick int, rows []Row, t terminal) {
	fmt.Print("\033[H")
	header := renderHeader(tick, true, t.useColor, t.width)
	var table string
	if opts.asJSON {
		table = ""
	} else {
		table = renderTerminal(rows, t.width, t.useColor)
	}
	_, _ = fmt.Fprint(os.Stdout, header+table)
	fmt.Print("\033[J")
}
