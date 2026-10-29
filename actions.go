package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// startDetached starts cmd and reaps it in the background so finished children
// don't linger as zombies. Output is discarded.
func startDetached(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return startDetached(exec.Command("open", url))
	case "linux":
		return startDetached(exec.Command("xdg-open", url))
	case "windows":
		return startDetached(exec.Command("rundll32", "url.dll,FileProtocolHandler", url))
	}
	return fmt.Errorf("opening a browser is not supported on %s", runtime.GOOS)
}

func copyToClipboard(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		cmd = exec.Command("xclip", "-selection", "clipboard")
	case "windows":
		cmd = exec.Command("clip")
	default:
		return fmt.Errorf("clipboard is not supported on %s", runtime.GOOS)
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// deliverToAgent hands msg to the command in GH_PR_DASH_AGENT_CMD (message as
// its last argument) and reports what actually happened. The agent runs
// detached with its output discarded, so "started" means the process launched,
// not that it succeeded. Without the variable it copies msg to the clipboard.
func deliverToAgent(msg string) (string, error) {
	if fields := strings.Fields(os.Getenv("GH_PR_DASH_AGENT_CMD")); len(fields) > 0 {
		if err := startDetached(exec.Command(fields[0], append(fields[1:], msg)...)); err != nil {
			return "", err
		}
		return "started " + fields[0], nil
	}
	if err := copyToClipboard(msg); err != nil {
		return "", fmt.Errorf("GH_PR_DASH_AGENT_CMD is unset and clipboard unavailable: %w", err)
	}
	return "GH_PR_DASH_AGENT_CMD unset, copied to clipboard", nil
}

func agentMessage(header string, rows []Row) string {
	var sb strings.Builder
	sb.WriteString(header + "\n\n")
	for _, r := range rows {
		fmt.Fprintf(&sb, "- [%s] %s - %s\n", r.Ref, r.Title, r.URL)
	}
	return sb.String()
}
