package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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

const (
	animInterval = 200 * time.Millisecond
	maxOpen      = 10 // refuse to open more browser tabs than this at once
	maxStatus    = 5  // failure lines shown after a merge
	mergeTimeout = time.Minute
)

type (
	fetchMsg struct {
		gen         int
		rows        []Row
		mergedTotal int
		err         error
	}
	animMsg  struct{}
	dataMsg  struct{}
	mergeMsg struct {
		merged int
		failed []string
	}
)

// group is a set of PRs an action can target with one key. The label starts
// with the key so the hint can render it as "[k]ey". allOpen selects every
// PR that isn't merged.
type group struct {
	key, label string
	tier       int
	header     string // agent prompt for this group
}

const allOpen = -1

var groups = []group{
	{"n", "needs", tierNeedsAction, "Please review and take action on PRs that need attention (CI failures, conflicts, unresolved comments):"},
	{"r", "ready", tierReady, "Please review and merge ready PRs (approved, green, no conflicts):"},
	{"c", "ci", tierBuilding, "Please monitor PRs waiting on CI or in the merge queue:"},
	{"u", "unassigned", tierNoReviewer, "Please find reviewers for PRs with no reviewer:"},
	{"w", "waiting", tierWaiting, "Please review PRs waiting for review:"},
	{"d", "drafts", tierDrafts, "Please review draft PRs:"},
	{"t", "today", tierMerged, "Please summarize recently merged PRs:"},
	{"a", "all", allOpen, "Please review all open PRs:"},
}

const targetHeader = "Please review and take action on these PRs:"

func findGroup(key string) (group, bool) {
	for _, g := range groups {
		if g.key == key {
			return g, true
		}
	}
	return group{}, false
}

func (g group) pick(rows []Row) []Row {
	var out []Row
	for _, r := range rows {
		if g.tier == allOpen && r.Tier != tierMerged || r.Tier == g.tier {
			out = append(out, r)
		}
	}
	return out
}

func groupHint() string {
	parts := make([]string, len(groups))
	for i, g := range groups {
		parts[i] = "[" + g.key + "]" + g.label[1:]
	}
	return strings.Join(parts, " ")
}

var actionNames = map[rune]string{'o': "open", 'm': "merge", 'x': "agent"}

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

	gen      int       // bumped after a merge; fetches from an older gen are stale
	deferred *fetchMsg // result held back while the user is mid-action

	action  rune   // pending action key (o/m/x) awaiting a target
	input   string // typed PR numbers for the pending action
	confirm []Row  // PRs awaiting y/n confirmation to merge
	skipped int    // targets left out of the merge because they aren't ready
	merging bool
	status  string // result of the last action; may span lines
}

func newWatchModel(opts options) watchModel {
	return watchModel{
		opts:        opts,
		useColor:    detectTerminal().useColor,
		nextRefresh: time.Now().Add(opts.interval),
	}
}

func (m watchModel) fetchCmd() tea.Cmd {
	opts, gen := m.opts, m.gen
	return func() tea.Msg {
		rows, total, err := fetchRows(opts, time.Now())
		return fetchMsg{gen, rows, total, err}
	}
}

func animTick() tea.Cmd {
	return tea.Tick(animInterval, func(time.Time) tea.Msg { return animMsg{} })
}

func (m watchModel) dataTick() tea.Cmd {
	return tea.Tick(m.opts.interval, func(time.Time) tea.Msg { return dataMsg{} })
}

func (m watchModel) Init() tea.Cmd {
	return tea.Batch(m.fetchCmd(), animTick(), m.dataTick())
}

// busy is true while the user is typing a target, confirming, or a merge runs.
// Fetch results are held back then so row numbers don't shift under the user.
func (m watchModel) busy() bool {
	return m.action != 0 || m.confirm != nil || m.merging
}

func (m watchModel) applyFetch(msg fetchMsg) watchModel {
	if msg.err != nil {
		m.errMsg = msg.err.Error()
		return m
	}
	m.rows, m.mergedTotal, m.errMsg = msg.rows, msg.mergedTotal, ""
	m.lastFetch = time.Now()
	return m
}

func (m watchModel) settle() watchModel {
	if !m.busy() && m.deferred != nil {
		m = m.applyFetch(*m.deferred)
		m.deferred = nil
	}
	return m
}

func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case fetchMsg:
		switch {
		case msg.gen != m.gen:
		case m.busy():
			m.deferred = &msg
		default:
			m = m.applyFetch(msg)
		}
	case animMsg:
		m.tick++
		return m, animTick()
	case dataMsg:
		m.nextRefresh = time.Now().Add(m.opts.interval)
		return m, tea.Batch(m.fetchCmd(), m.dataTick())
	case mergeMsg:
		m.merging = false
		m.status = mergeStatus(msg)
		// Anything fetched before the merge finished is stale; only the fetch
		// issued now may replace the rows.
		m.gen++
		m.deferred = nil
		return m.settle(), m.fetchCmd()
	case tea.KeyMsg:
		next, cmd := m.key(msg)
		return next.settle(), cmd
	}
	return m, nil
}

func (m watchModel) key(msg tea.KeyMsg) (watchModel, tea.Cmd) {
	if msg.Paste {
		// Pasted text is only ever target numbers; it must never trigger an
		// action, a group, or a confirmation.
		text := strings.NewReplacer("\r", "", "\n", "").Replace(string(msg.Runes))
		if m.action != 0 && m.confirm == nil && !m.merging && isTargetText(text) {
			m.input += text
		}
		return m, nil
	}
	k := msg.String()
	if k == "ctrl+c" {
		return m, tea.Quit
	}
	rows := filterRows(m.rows, m.opts.repo)

	switch {
	case m.merging:
		return m, nil
	case m.confirm != nil:
		return m.keyConfirm(k)
	case m.action != 0:
		return m.keyTarget(k, rows)
	}

	switch k {
	case "q":
		return m, tea.Quit
	case "o", "m", "x":
		m.action, m.input, m.status = rune(k[0]), "", ""
	case "s":
		m.status = "slack: not implemented"
	}
	return m, nil
}

func (m watchModel) keyConfirm(k string) (watchModel, tea.Cmd) {
	switch k {
	case "y", "Y":
		pending := m.confirm
		m.confirm, m.skipped, m.merging = nil, 0, true
		m.status = fmt.Sprintf("merging %d PR(s)…", len(pending))
		return m, mergeCmd(pending)
	case "n", "N", "esc":
		m.confirm, m.skipped = nil, 0
		m.status = "merge cancelled"
	}
	return m, nil
}

// keyTarget collects the target of a pending action: PR numbers (1,3 5)
// confirmed with Enter, or a group key that applies to the whole group.
func (m watchModel) keyTarget(k string, rows []Row) (watchModel, tea.Cmd) {
	switch {
	case k == "esc":
		m.action, m.input = 0, ""
	case k == "backspace":
		if m.input != "" {
			m.input = m.input[:len(m.input)-1]
		}
	case k == "enter":
		return m.run(parseTargets(m.input, rows), group{})
	case isTargetText(k):
		m.input += k
	case m.input == "":
		if g, ok := findGroup(k); ok {
			return m.run(g.pick(rows), g)
		}
	}
	return m, nil
}

// isTargetText accepts digits, commas and spaces, including pasted runs.
func isTargetText(k string) bool {
	if k == "" {
		return false
	}
	for _, c := range k {
		if (c < '0' || c > '9') && c != ',' && c != ' ' {
			return false
		}
	}
	return true
}

// parseTargets resolves "1,3 5" against the displayed row numbers, ignoring
// anything out of range or repeated.
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

func (m watchModel) run(targets []Row, g group) (watchModel, tea.Cmd) {
	action := m.action
	m.action, m.input = 0, ""
	if len(targets) == 0 {
		m.status = "no matching PRs"
		return m, nil
	}

	switch action {
	case 'o':
		m.status = openStatus(targets)
	case 'm':
		ready := group{tier: tierReady}.pick(targets)
		if len(ready) == 0 {
			m.status = "nothing to merge: only ready PRs (approved, CI green, no conflicts) can be merged"
			return m, nil
		}
		m.confirm, m.skipped = ready, len(targets)-len(ready)
	case 'x':
		header := g.header
		if header == "" {
			header = targetHeader
		}
		how, err := deliverToAgent(agentMessage(header, targets))
		if err != nil {
			m.status = "agent failed: " + err.Error()
		} else {
			m.status = fmt.Sprintf("agent: %s (%d PR(s))", how, len(targets))
		}
	}
	return m, nil
}

func openStatus(targets []Row) string {
	if len(targets) > maxOpen {
		return fmt.Sprintf("refusing to open %d PRs at once (max %d); pick fewer", len(targets), maxOpen)
	}
	opened := 0
	var firstErr error
	for _, r := range targets {
		if err := openBrowser(r.URL); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		opened++
	}
	if firstErr != nil {
		return fmt.Sprintf("opened %d of %d PR(s); failed: %v", opened, len(targets), firstErr)
	}
	return fmt.Sprintf("opened %d PR(s)", opened)
}

func mergeCmd(rows []Row) tea.Cmd {
	flags := strings.Fields(os.Getenv("GH_PR_DASH_MERGE_FLAGS"))
	if len(flags) == 0 {
		flags = []string{"--squash"}
	}
	return func() tea.Msg {
		var res mergeMsg
		for _, r := range rows {
			ctx, cancel := context.WithTimeout(context.Background(), mergeTimeout)
			args := append([]string{"pr", "merge", r.URL}, flags...)
			out, err := exec.CommandContext(ctx, "gh", args...).CombinedOutput()
			cancel()
			if err != nil {
				res.failed = append(res.failed, r.Ref+": "+lastLine(string(out), err))
			} else {
				res.merged++
			}
		}
		return res
	}
}

// lastLine picks the most informative line of gh's output (its error is last).
func lastLine(out string, err error) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if l := strings.TrimSpace(lines[len(lines)-1]); l != "" {
		return l
	}
	return err.Error()
}

func mergeStatus(r mergeMsg) string {
	if len(r.failed) == 0 {
		return fmt.Sprintf("merged %d PR(s)", r.merged)
	}
	lines := []string{fmt.Sprintf("merged %d, %d failed:", r.merged, len(r.failed))}
	for i, f := range r.failed {
		if i == maxStatus {
			lines = append(lines, fmt.Sprintf("  … and %d more", len(r.failed)-maxStatus))
			break
		}
		lines = append(lines, "  ✗ "+f)
	}
	return strings.Join(lines, "\n")
}

// footer is the bottom help/prompt block (one or more lines) for the current
// input state.
func (m watchModel) footer() string {
	switch {
	case m.confirm != nil:
		head := fmt.Sprintf("Merge %d PR(s)?", len(m.confirm))
		if m.skipped > 0 {
			head += fmt.Sprintf(" (%d not ready, skipped)", m.skipped)
		}
		lines := []string{head}
		for i, r := range m.confirm {
			if i == maxStatus+3 {
				lines = append(lines, fmt.Sprintf("  … and %d more", len(m.confirm)-i))
				break
			}
			lines = append(lines, "  "+r.Ref+"  "+r.Title)
		}
		return strings.Join(append(lines, "[y] merge · [n]/esc cancel"), "\n")
	case m.action != 0:
		hint := "numbers (1,3) + enter, or a group now: " + groupHint() + " · esc cancel"
		if m.input != "" {
			hint = "enter run · backspace edit · esc cancel"
		}
		return actionNames[m.action] + " › " + m.input + "▏\n" + hint
	}
	help := "[o]pen  [m]erge  [x] agent — then PR numbers + enter, or a group · [q] quit"
	if m.status != "" {
		return m.status + "\n" + help
	}
	return help
}

func (m watchModel) View() string {
	if m.width == 0 {
		return ""
	}
	t := terminal{width: m.width, height: m.height, useColor: m.useColor}
	return renderWatch(m.opts, m.tick, m.rows, m.mergedTotal, m.errMsg, refreshStatus(m.lastFetch, m.nextRefresh), m.footer(), t)
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

	footLines := strings.Split(footer, "\n")
	for i, l := range footLines {
		footLines[i] = p.d + "  " + truncate(l, inner-2) + p.z
	}

	// Keep the footer visible on short terminals by clipping the body instead.
	body := strings.Split(indent(out, m), "\n")
	if keep := t.height - len(footLines); keep > 0 && len(body) > keep {
		body = body[:keep]
	}
	return strings.Join(body, "\n") + "\n" + indent(strings.Join(footLines, "\n"), m)
}
