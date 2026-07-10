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

	// Fetch off the render loop so the network never stalls the animation or a
	// Ctrl-C; each fetch delivers its result over the channel as a select case.
	results := make(chan fetchResult, 1)
	startFetch := func() {
		nextRefresh = time.Now().Add(opts.interval)
		go func() {
			prs, err := fetchPRs(opts.limit)
			results <- fetchResult{prs, err}
		}()
	}

	startFetch()

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
		case r := <-results:
			if r.err != nil {
				errMsg = r.err.Error()
			} else {
				cachedRows = buildRows(r.prs, opts.org, time.Now())
				lastFetch = time.Now()
				errMsg = ""
			}
		case <-dataTicker.C:
			startFetch()
			tick++
		case <-animTicker.C:
			// Just animate (tick increments, no data fetch)
			tick++
		}
	}
}

type fetchResult struct {
	prs []ghPR
	err error
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
	switch {
	case opts.asJSON:
	case len(rows) == 0 && errMsg == "" && status == "":
		out += p.d + "  loading…" + p.z + "\n"
	default:
		out += renderTerminal(rows, t.width, t.useColor)
	}
	_, _ = fmt.Fprint(os.Stdout, out)
	fmt.Print("\033[J")
}
