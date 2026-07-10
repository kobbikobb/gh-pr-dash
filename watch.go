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

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		fmt.Print("\033[H")
		if err := fetchAndRender(os.Stdout, opts); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		fmt.Print("\033[J")

		select {
		case <-sig:
			fmt.Println()
			return
		case <-ticker.C:
		}
	}
}
