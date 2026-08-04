package main

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
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
	var cachedMergedTotal int
	var errMsg string
	var lastFetch, nextRefresh time.Time

	// Fetch off the render loop so the network never stalls the animation or a
	// Ctrl-C; each fetch delivers its result over the channel as a select case.
	results := make(chan fetchResult, 1)
	startFetch := func() {
		nextRefresh = time.Now().Add(opts.interval)
		go func() {
			rows, mergedTotal, err := fetchRows(opts, time.Now())
			results <- fetchResult{rows, mergedTotal, err}
		}()
	}

	startFetch()

	for {
		t := detectTerminal()
		if t.width != lastWidth {
			fmt.Print("\033[2J")
			lastWidth = t.width
		}
		renderWatch(opts, tick, cachedRows, cachedMergedTotal, errMsg, refreshStatus(lastFetch, nextRefresh), t)

		select {
		case <-sig:
			return
		case r := <-results:
			if r.err != nil {
				errMsg = r.err.Error()
			} else {
				cachedRows = r.rows
				cachedMergedTotal = r.mergedTotal
				lastFetch = time.Now()
				errMsg = ""
			}
		case <-dataTicker.C:
			startFetch()
			tick++
		case <-animTicker.C:
			tick++
		}
	}
}

type fetchResult struct {
	rows        []Row
	mergedTotal int
	err         error
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

func renderWatch(opts options, tick int, rows []Row, mergedTotal int, errMsg, status string, t terminal) {
	fmt.Print("\033[H")
	p := colors(t.useColor)
	m := gutter(t.width)
	inner := t.width - 2*m
	out := renderHeader(tick, true, t.useColor, inner, t.height, status)
	if errMsg != "" {
		out += p.r + "  ⚠ " + errMsg + p.z + "\n\n"
	}
	switch {
	case opts.asJSON:
	case len(rows) == 0 && errMsg == "" && status == "":
		out += p.d + "  loading…" + p.z + "\n"
	default:
		filtered := filterRows(rows, opts.repo)
		if opts.repo != "" && len(filtered) == 0 {
			out += p.y + "  ⚠ --repo \"" + opts.repo + "\" matched no PRs" + p.z + "\n"
		}
		total := mergedTotal
		if opts.repo != "" {
			total = 0
		}
		out += renderTerminal(filtered, inner, t.useColor, total)
	}
	out = strings.ReplaceAll(indent(out, m), "\n", "\033[K\n")
	_, _ = fmt.Fprint(os.Stdout, out)
	fmt.Print("\033[J")
}
