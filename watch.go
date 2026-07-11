package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"golang.org/x/term"
)

func watchLoop(opts options) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sig)

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err == nil {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}

	keys := make(chan byte, 1)
	go func() {
		buf := make([]byte, 1)
		for {
			if _, err := os.Stdin.Read(buf); err != nil {
				return
			}
			keys <- buf[0]
		}
	}()

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
			rows, err := fetchRows(opts, time.Now())
			results <- fetchResult{rows, err}
		}()
	}

	startFetch()

	for {
		t := detectTerminal()
		if t.width != lastWidth {
			fmt.Print("\033[2J")
			lastWidth = t.width
		}
		renderWatch(opts, tick, cachedRows, errMsg, refreshStatus(lastFetch, nextRefresh), t)

		select {
		case <-sig:
			return
		case k := <-keys:
			switch {
			case k == 0x03 || k == 'q':
				return
			case k >= '1' && k <= '9':
				idx := int(k-'0') - 1
				filtered := filterRows(cachedRows, opts.repo)
				if idx < len(filtered) {
					openURL(filtered[idx].URL)
				}
			}
		case r := <-results:
			if r.err != nil {
				errMsg = r.err.Error()
			} else {
				cachedRows = r.rows
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
	rows []Row
	err  error
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
		filtered := filterRows(rows, opts.repo)
		if opts.repo != "" && len(filtered) == 0 {
			out += p.y + "  ⚠ --repo \"" + opts.repo + "\" matched no PRs" + p.z + "\n"
		}
		out += renderTerminal(filtered, t.width, t.useColor, false)
	}
	_, _ = fmt.Fprint(os.Stdout, out)
	fmt.Print("\033[J")
}

func openURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
