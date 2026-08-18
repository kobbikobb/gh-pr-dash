package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

func watchLoop(opts options) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sig)

	fmt.Print("\033[?1049h\033[?25l")
	defer fmt.Print("\033[?25h\033[?1049l")

	defer os.Stdin.Close()
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err == nil {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}

	dataTicker := time.NewTicker(opts.interval)
	animTicker := time.NewTicker(200 * time.Millisecond)
	defer dataTicker.Stop()
	defer animTicker.Stop()

	keyCh := make(chan byte, 32)
	if err == nil {
		go func() {
			buf := make([]byte, 1)
			for {
				if _, err := os.Stdin.Read(buf); err != nil {
					return
				}
				keyCh <- buf[0]
			}
		}()
	}

	tick := 0
	lastWidth := 0
	var cachedRows []Row
	var cachedMergedTotal int
	var errMsg string
	var lastFetch, nextRefresh time.Time
	var digitBuf []byte
	var debounceCh <-chan time.Time

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
		case b := <-keyCh:
			switch {
			case b == 'q' || b == 3: // q or Ctrl-C
				return
			case b >= '0' && b <= '9':
				digitBuf = append(digitBuf, b)
				debounceCh = time.After(500 * time.Millisecond)
			case b == 13: // Enter
				debounceCh = nil
				fireNumber(digitBuf, cachedRows, opts.repo)
				digitBuf = digitBuf[:0]
			case b == 27: // Escape
				debounceCh = nil
				digitBuf = digitBuf[:0]
			}
		case <-debounceCh:
			debounceCh = nil
			fireNumber(digitBuf, cachedRows, opts.repo)
			digitBuf = digitBuf[:0]
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
	out = strings.ReplaceAll(indent(out, m), "\n", "\033[K\r\n")
	_, _ = fmt.Fprint(os.Stdout, out)
	fmt.Print("\033[J")
}

func openBrowser(url string) {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("open", url).Start()
	case "linux":
		exec.Command("xdg-open", url).Start()
	case "windows":
		exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
}

func fireNumber(digits []byte, rows []Row, repo string) {
	num, err := strconv.Atoi(string(digits))
	if err != nil || num <= 0 {
		return
	}
	filtered := filterRows(rows, repo)
	if num <= len(filtered) {
		openBrowser(filtered[num-1].URL)
	}
}
