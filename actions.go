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

// agentTools are tried in order when GH_PR_DASH_AGENT_CMD is unset; the message
// is appended as the final argument.
var agentTools = []struct {
	bin  string
	args []string
}{
	{"opencode", []string{"run", "--"}},
	{"claude", []string{"-p"}},
	{"aider", []string{"--message"}},
	{"cursor", []string{"--command", "workbench.action.chat.open"}},
	{"code", []string{"--command", "workbench.action.chat.open"}},
}

// deliverToAgent hands msg to an agent and reports what actually happened.
// Agents run detached with their output discarded, so "started" means the
// process launched, not that it succeeded. With no agent available it falls
// back to the clipboard and says so.
func deliverToAgent(msg string) (string, error) {
	if fields := strings.Fields(os.Getenv("GH_PR_DASH_AGENT_CMD")); len(fields) > 0 {
		if err := startDetached(exec.Command(fields[0], append(fields[1:], msg)...)); err != nil {
			return "", err
		}
		return "started " + fields[0], nil
	}
	for _, t := range agentTools {
		if _, err := exec.LookPath(t.bin); err != nil {
			continue
		}
		args := append(append([]string{}, t.args...), msg)
		if err := startDetached(exec.Command(t.bin, args...)); err != nil {
			return "", err
		}
		return "started " + t.bin, nil
	}
	if err := copyToClipboard(msg); err != nil {
		return "", fmt.Errorf("no agent found and clipboard unavailable: %w", err)
	}
	return "no agent found, copied to clipboard", nil
}

var agentHeaders = map[int]string{
	tierNeedsAction: "Please review and take action on PRs that need attention (CI failures/conflicts):",
	tierReady:       "Please review and merge ready PRs (approved, green, no conflicts):",
	tierBuilding:    "Please monitor PRs waiting on CI:",
	tierWaiting:     "Please review PRs waiting for review:",
	tierDrafts:      "Please review draft PRs:",
	tierMerged:      "Please summarize recently merged PRs:",
}

func agentHeader(group string) string {
	if group == "a" {
		return "Please review all open PRs:"
	}
	if tier, ok := groupTiers[group]; ok {
		return agentHeaders[tier]
	}
	return "Please review and take action on these PRs:"
}

func agentMessage(header string, rows []Row) string {
	var sb strings.Builder
	sb.WriteString(header + "\n\n")
	for _, r := range rows {
		fmt.Fprintf(&sb, "- [%s] %s - %s\n", r.Ref, r.Title, r.URL)
	}
	return sb.String()
}
