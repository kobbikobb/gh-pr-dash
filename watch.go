package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// watchLoop runs the dashboard as a Bubble Tea program. All state changes go
// through Update on a single goroutine, and the renderer diffs and writes one
// frame at a time, so there are no interleaved writes or half-drawn frames.
func watchLoop(opts options) {
	p := tea.NewProgram(newWatchModel(opts), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

const animInterval = 200 * time.Millisecond

type (
	fetchMsg struct {
		rows        []Row
		mergedTotal int
		err         error
	}
	animMsg struct{}
	dataMsg struct{}
)

type watchModel struct {
	opts          options
	width, height int
	useColor      bool
	tick          int
	rows          []Row
	mergedTotal   int
	errMsg        string
	lastFetch     time.Time
	nextRefresh   time.Time
	action        rune   // pending action key (o/m/x/s) awaiting a target
	input         string // typed PR numbers for the pending action
	confirm       []Row  // PRs awaiting y/n confirmation to merge
	status        string // result of the last action
}

type mergeMsg struct{ merged, failed []string }

func newWatchModel(opts options) watchModel {
	return watchModel{
		opts:        opts,
		useColor:    detectTerminal().useColor,
		nextRefresh: time.Now().Add(opts.interval),
	}
}

func (m watchModel) fetchCmd() tea.Cmd {
	opts := m.opts
	return func() tea.Msg {
		rows, total, err := fetchRows(opts, time.Now())
		return fetchMsg{rows, total, err}
	}
}

func (m watchModel) Init() tea.Cmd {
	return tea.Batch(
		m.fetchCmd(),
		tea.Tick(animInterval, func(time.Time) tea.Msg { return animMsg{} }),
		tea.Tick(m.opts.interval, func(time.Time) tea.Msg { return dataMsg{} }),
	)
}

func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case fetchMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
		} else {
			m.rows, m.mergedTotal, m.errMsg = msg.rows, msg.mergedTotal, ""
			m.lastFetch = time.Now()
		}
	case animMsg:
		m.tick++
		return m, tea.Tick(animInterval, func(time.Time) tea.Msg { return animMsg{} })
	case dataMsg:
		m.nextRefresh = time.Now().Add(m.opts.interval)
		return m, tea.Batch(m.fetchCmd(), tea.Tick(m.opts.interval, func(time.Time) tea.Msg { return dataMsg{} }))
	case mergeMsg:
		m.status = mergeStatus(msg)
		return m, m.fetchCmd()
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// groupTiers maps a group key to its tier; 'a' (all open) is handled separately.
var groupTiers = map[string]int{
	"n": tierNeedsAction, "r": tierReady, "c": tierBuilding,
	"w": tierWaiting, "d": tierDrafts, "t": tierMerged,
}

var actionNames = map[string]string{"o": "open", "m": "merge", "x": "agent", "s": "slack"}

func (m watchModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := filterRows(m.rows, m.opts.repo)
	key := msg.String()

	if key == "ctrl+c" {
		return m, tea.Quit
	}

	if m.confirm != nil {
		pending := m.confirm
		m.confirm = nil
		if key == "y" || key == "Y" {
			m.status = fmt.Sprintf("merging %d PR(s)…", len(pending))
			return m, mergeCmd(pending)
		}
		m.status = "merge cancelled"
		return m, nil
	}

	if m.action != 0 {
		return m.handleTarget(key, rows)
	}

	switch key {
	case "q":
		return m, tea.Quit
	case "o", "m", "x", "s":
		m.action, m.input, m.status = rune(key[0]), "", ""
	case "p":
		popupToAgentWaiting(rows)
	}
	return m, nil
}

// handleTarget collects the target of a pending action: PR numbers (1,3 5)
// confirmed with Enter, or a group key that applies to the whole group.
func (m watchModel) handleTarget(key string, rows []Row) (tea.Model, tea.Cmd) {
	switch {
	case key == "esc":
		m.action, m.input = 0, ""
	case key == "backspace":
		if m.input != "" {
			m.input = m.input[:len(m.input)-1]
		}
	case key == "enter":
		return m.run(parseTargets(m.input, rows), "")
	case len(key) == 1 && (key[0] >= '0' && key[0] <= '9' || key[0] == ',' || key[0] == ' '):
		m.input += key
	case m.input == "" && key == "a":
		return m.run(openRows(rows), "a")
	case m.input == "":
		if tier, ok := groupTiers[key]; ok {
			return m.run(tierRows(rows, tier), key)
		}
	}
	return m, nil
}

func openRows(rows []Row) []Row {
	var out []Row
	for _, r := range rows {
		if r.Tier != tierMerged {
			out = append(out, r)
		}
	}
	return out
}

func tierRows(rows []Row, tier int) []Row {
	var out []Row
	for _, r := range rows {
		if r.Tier == tier {
			out = append(out, r)
		}
	}
	return out
}

// parseTargets resolves "1,3 5" against the displayed row numbers, ignoring
// anything out of range.
func parseTargets(input string, rows []Row) []Row {
	var out []Row
	seen := map[int]bool{}
	for _, tok := range strings.FieldsFunc(input, func(r rune) bool { return r == ',' || r == ' ' }) {
		n, err := strconv.Atoi(tok)
		if err != nil || n < 1 || n > len(rows) || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, rows[n-1])
	}
	return out
}

func (m watchModel) run(targets []Row, group string) (tea.Model, tea.Cmd) {
	action := string(m.action)
	m.action, m.input = 0, ""
	if len(targets) == 0 {
		m.status = "no matching PRs"
		return m, nil
	}
	switch action {
	case "o":
		for _, r := range targets {
			openBrowser(r.URL)
		}
		m.status = fmt.Sprintf("opened %d PR(s)", len(targets))
	case "m":
		var mergeable []Row
		for _, r := range targets {
			if r.Tier != tierMerged {
				mergeable = append(mergeable, r)
			}
		}
		if len(mergeable) == 0 {
			m.status = "already merged"
			return m, nil
		}
		m.confirm = mergeable
	case "x":
		if group == "a" {
			sendToAgentAllOpen(targets)
		} else if tier, ok := groupTiers[group]; ok {
			sendToAgentTier(targets, tier)
		} else {
			sendToAgentTier(targets, -1)
		}
		m.status = fmt.Sprintf("sent %d PR(s) to agent", len(targets))
	case "s":
		m.status = "slack: not implemented"
	}
	return m, nil
}

func mergeCmd(rows []Row) tea.Cmd {
	flags := strings.Fields(os.Getenv("GH_PR_DASH_MERGE_FLAGS"))
	if len(flags) == 0 {
		flags = []string{"--squash"}
	}
	return func() tea.Msg {
		var res mergeMsg
		for _, r := range rows {
			args := append([]string{"pr", "merge", r.URL}, flags...)
			if out, err := exec.Command("gh", args...).CombinedOutput(); err != nil {
				res.failed = append(res.failed, r.Ref+": "+strings.TrimSpace(string(out)))
			} else {
				res.merged = append(res.merged, r.Ref)
			}
		}
		return res
	}
}

func mergeStatus(r mergeMsg) string {
	s := fmt.Sprintf("merged %d", len(r.merged))
	if len(r.failed) > 0 {
		s += fmt.Sprintf(", %d failed: %s", len(r.failed), strings.Join(r.failed, "; "))
	}
	return s
}

// footer is the bottom help/prompt line for the current input state.
func (m watchModel) footer(rows []Row) string {
	switch {
	case m.confirm != nil:
		refs := make([]string, len(m.confirm))
		for i, r := range m.confirm {
			refs[i] = r.Ref
		}
		return fmt.Sprintf("Merge %d PR(s): %s ?  [y] confirm · any other key cancels", len(refs), strings.Join(refs, ", "))
	case m.action != 0:
		return fmt.Sprintf("%s › %s▏  numbers (1,3) + enter · group: [n]eeds [r]eady [c]i [w]aiting [d]rafts [t]oday [a]ll · esc cancel",
			actionNames[string(m.action)], m.input)
	}
	help := "[o]pen  [m]erge  [x] agent  [s]lack  then PR numbers + enter, or a group · [p] popup · [q] quit"
	if m.status != "" {
		return m.status + "   |   " + help
	}
	return help
}

func (m watchModel) View() string {
	if m.width == 0 {
		return ""
	}
	t := terminal{width: m.width, height: m.height, useColor: m.useColor}
	return renderWatch(m.opts, m.tick, m.rows, m.mergedTotal, m.errMsg, refreshStatus(m.lastFetch, m.nextRefresh), m.footer(filterRows(m.rows, m.opts.repo)), t)
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

func renderWatch(opts options, tick int, rows []Row, mergedTotal int, errMsg, status, footer string, t terminal) string {
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
	body := strings.Split(indent(out, m), "\n")
	if t.height > 2 && len(body) > t.height-2 {
		body = body[:t.height-2]
	}
	foot := p.d + "  " + truncate(footer, inner-2) + p.z
	return strings.Join(body, "\n") + "\n" + indent(foot, m)
}

func openBrowser(url string) {
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("open", url).Start()
	case "linux":
		_ = exec.Command("xdg-open", url).Start()
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	}
}

func buildAgentMessage(tier int, rows []Row) string {
	var sb strings.Builder
	switch tier {
	case -1:
		sb.WriteString("Please review and take action on these PRs:\n\n")
	case tierNeedsAction:
		sb.WriteString("Please review and take action on PRs that need attention (CI failures/conflicts):\n\n")
	case tierReady:
		sb.WriteString("Please review and merge ready PRs (approved, green, no conflicts):\n\n")
	case tierBuilding:
		sb.WriteString("Please monitor/check PRs waiting on CI:\n\n")
	case tierWaiting:
		sb.WriteString("Please review PRs waiting for review:\n\n")
	case tierDrafts:
		sb.WriteString("Please review draft PRs:\n\n")
	case tierMerged:
		sb.WriteString("Please summarize/review recently merged PRs:\n\n")
	}
	for i, r := range rows {
		if tier != -1 && r.Tier != tier {
			continue
		}
		sb.WriteString(fmt.Sprintf("%d. [%s] %s - %s\n", i+1, r.Ref, r.Title, r.URL))
	}
	return sb.String()
}

func buildAgentMessageAllOpen(rows []Row) string {
	var sb strings.Builder
	sb.WriteString("Please review all open PRs across categories:\n\n")
	for i, r := range rows {
		if r.Tier == tierMerged {
			continue
		}
		sb.WriteString(fmt.Sprintf("%d. [%s] %s - %s\n", i+1, r.Ref, r.Title, r.URL))
	}
	return sb.String()
}

func sendToAgentTier(rows []Row, tier int) {
	msg := buildAgentMessage(tier, rows)
	sendToAgent(msg)
}

func sendToAgentAllOpen(rows []Row) {
	msg := buildAgentMessageAllOpen(rows)
	sendToAgent(msg)
}

func sendToAgent(msg string) {
	tmpfile := "/tmp/gh-pr-dash-agent-msg.txt"
	_ = os.WriteFile(tmpfile, []byte(msg), 0644)

	if cmd := os.Getenv("GH_PR_DASH_AGENT_CMD"); cmd != "" {
		parts := strings.Fields(cmd)
		if len(parts) > 0 {
			c := exec.Command(parts[0], append(parts[1:], msg)...)
			_ = c.Start()
			return
		}
	}

	if _, err := exec.LookPath("opencode"); err == nil {
		_ = exec.Command("opencode", "run", "--", msg).Start()
		return
	}
	if _, err := exec.LookPath("claude"); err == nil {
		_ = exec.Command("claude", "-p", msg).Start()
		return
	}
	if _, err := exec.LookPath("aider"); err == nil {
		_ = exec.Command("aider", "--message", msg).Start()
		return
	}
	if _, err := exec.LookPath("cursor"); err == nil {
		_ = exec.Command("cursor", "--command", "workbench.action.chat.open", msg).Start()
		return
	}
	if _, err := exec.LookPath("code"); err == nil {
		_ = exec.Command("code", "--command", "workbench.action.chat.open", msg).Start()
		return
	}

	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("pbcopy")
		stdin, _ := cmd.StdinPipe()
		if stdin != nil {
			go func() {
				_, _ = stdin.Write([]byte(msg))
				_ = stdin.Close()
			}()
		}
		_ = cmd.Start()
	case "linux":
		cmd := exec.Command("xclip", "-selection", "clipboard")
		stdin, _ := cmd.StdinPipe()
		if stdin != nil {
			go func() {
				_, _ = stdin.Write([]byte(msg))
				_ = stdin.Close()
			}()
		}
		_ = cmd.Start()
	case "windows":
		cmd := exec.Command("clip")
		stdin, _ := cmd.StdinPipe()
		if stdin != nil {
			go func() {
				_, _ = stdin.Write([]byte(msg))
				_ = stdin.Close()
			}()
		}
		_ = cmd.Start()
	}
}

func (r Row) isMergedLike() bool {
	return r.Tier == tierMerged
}

func popupToAgentWaiting(rows []Row) {
	var sb strings.Builder
	sb.WriteString("Review these PRs waiting for review. For each, provide review feedback or approve if appropriate.\n\n")
	for _, r := range rows {
		if r.Tier == tierWaiting && r.Review != "approved" && !r.isMergedLike() {
			sb.WriteString("- " + r.Ref + ": " + r.Title + "\n  " + r.URL + "\n\n")
		}
	}
	msg := sb.String()
	if strings.TrimSpace(msg) == "" {
		msg = "No PRs waiting for review."
	}
	if cmd := os.Getenv("GH_PR_DASH_POPUP_CMD"); cmd != "" {
		parts := strings.Fields(cmd)
		if len(parts) > 0 {
			c := exec.Command(parts[0], append(parts[1:], msg)...)
			_ = c.Start()
			return
		}
	}
	switch runtime.GOOS {
	case "darwin":
		// Try terminal-notifier style popup? Or use osascript
		escaped := strings.ReplaceAll(msg, "\"", "\\\"")
		escaped = strings.ReplaceAll(escaped, "\n", "\\n")
		script := `display dialog "` + escaped + `" with title "PR Review Request" buttons {"Copy to Agent", "Cancel"} default button "Copy to Agent"`
		_ = exec.Command("osascript", "-e", script).Start()
		sendToAgent(msg)
		return
	case "linux":
		_ = exec.Command("zenity", "--info", "--title=PR Review Request", "--text="+msg).Start()
		sendToAgent(msg)
		return
	case "windows":
		ps := `Add-Type -AssemblyName PresentationFramework; [System.Windows.MessageBox]::Show('` + strings.ReplaceAll(msg, "'", "''") + `','PR Review Request')`
		_ = exec.Command("powershell", "-Command", ps).Start()
		sendToAgent(msg)
		return
	}
	sendToAgent(msg)
}
