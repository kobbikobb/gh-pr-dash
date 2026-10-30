package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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

const targetHeader = "Please review and take action on these PRs:"

// inTier picks the rows of one category.
func inTier(rows []Row, tier int) []Row {
	var out []Row
	for _, r := range rows {
		if r.Tier == tier {
			out = append(out, r)
		}
	}
	return out
}

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

	cursor    string       // URL of the selected PR, so it survives refreshes
	lastIdx   int          // cursor position before the last refresh, for re-seating it
	collapsed map[int]bool // categories folded to just their border
	help      bool

	confirm []Row // PRs awaiting y/n confirmation to merge
	merging bool
	status  string // result of the last action; may span lines
}

func newWatchModel(opts options) watchModel {
	return watchModel{
		opts:        opts,
		collapsed:   map[int]bool{},
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

// busy is true while the user is confirming or a merge runs. Fetch results are
// held back then so the list being confirmed doesn't change under the user.
func (m watchModel) busy() bool {
	return m.confirm != nil || m.merging
}

func (m watchModel) applyFetch(msg fetchMsg) watchModel {
	if msg.err != nil {
		m.errMsg = msg.err.Error()
		return m
	}
	m.rows, m.mergedTotal, m.errMsg = msg.rows, msg.mergedTotal, ""
	m.lastFetch = time.Now()
	return m.fixCursor()
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
		return m, nil // pasted text must never trigger an action or confirmation
	}
	k := msg.String()
	if k == "ctrl+c" {
		return m, tea.Quit
	}

	switch {
	case m.merging:
		return m, nil
	case m.confirm != nil:
		return m.keyConfirm(k)
	}

	switch k {
	case "q":
		return m, tea.Quit
	case "esc":
		m.help, m.status = false, ""
	case "?":
		m.help = !m.help
	case "up", "k":
		return m.move(-1), nil
	case "down", "j":
		return m.move(1), nil
	case "enter":
		return m.toggleCollapse(), nil
	case "1", "2", "3", "4", "5", "6", "7":
		return m.jump(int(k[0] - '1')), nil
	case "o", "m", "a", "O", "M", "A":
		return m.act(k)
	}
	return m, nil
}

func (m watchModel) keyConfirm(k string) (watchModel, tea.Cmd) {
	switch k {
	case "y", "Y":
		pending := m.confirm
		m.confirm, m.merging = nil, true
		m.status = fmt.Sprintf("merging %d PR(s)…", len(pending))
		return m, mergeCmd(pending)
	case "n", "N", "esc":
		m.confirm = nil
		m.status = "merge cancelled"
	}
	return m, nil
}

// shown is the selectable rows in display order: every PR, except that a folded
// category is reduced to a single stop.
func (m watchModel) shown() []Row {
	var out []Row
	last := -1
	for _, r := range filterRows(m.rows, m.opts.repo) {
		// A folded category keeps one stop (its first PR) so enter can unfold it.
		if !m.collapsed[r.Tier] || r.Tier != last {
			out = append(out, r)
		}
		last = r.Tier
	}
	return out
}

func indexOfURL(rows []Row, url string) int {
	for i, r := range rows {
		if r.URL == url {
			return i
		}
	}
	return -1
}

func (m watchModel) selected() (Row, bool) {
	shown := m.shown()
	if i := indexOfURL(shown, m.cursor); i >= 0 {
		return shown[i], true
	}
	return Row{}, false
}

// fixCursor keeps the cursor on the same PR after rows change. If that PR is
// gone, it lands on whatever now sits at the same position.
func (m watchModel) fixCursor() watchModel {
	shown := m.shown()
	if len(shown) == 0 {
		m.cursor = ""
		return m
	}
	if i := indexOfURL(shown, m.cursor); i >= 0 {
		m.lastIdx = i
		return m
	}
	m.lastIdx = min(m.lastIdx, len(shown)-1)
	m.cursor = shown[m.lastIdx].URL
	return m
}

func (m watchModel) move(delta int) watchModel {
	shown := m.shown()
	if len(shown) == 0 {
		return m
	}
	i := indexOfURL(shown, m.cursor)
	if i < 0 {
		i = 0
	} else {
		i = min(max(i+delta, 0), len(shown)-1)
	}
	m.cursor, m.lastIdx = shown[i].URL, i
	return m
}

// jump moves to the first PR of a category, unfolding it if needed.
func (m watchModel) jump(tier int) watchModel {
	rows := inTier(filterRows(m.rows, m.opts.repo), tier)
	if len(rows) == 0 {
		m.status = "no PRs in " + categories[tier].name
		return m
	}
	m.status = ""
	m.collapsed[tier] = false
	m.cursor = rows[0].URL
	return m.fixCursor()
}

// toggleCollapse folds or unfolds the cursor's category. A folded category is
// a single stop, so enter on it unfolds it again.
func (m watchModel) toggleCollapse() watchModel {
	sel, ok := m.selected()
	if !ok {
		return m
	}
	m.collapsed[sel.Tier] = !m.collapsed[sel.Tier]
	m.cursor = sel.URL
	return m.fixCursor()
}

// act runs an action on the selected PR (lowercase) or on every PR in its
// category (uppercase).
func (m watchModel) act(k string) (watchModel, tea.Cmd) {
	sel, ok := m.selected()
	if !ok {
		m.status = "no PR selected"
		return m, nil
	}
	if m.collapsed[sel.Tier] && k[0] >= 'a' && k[0] <= 'z' {
		m.status = "category is folded — press enter to unfold"
		return m, nil
	}
	m.status = ""
	targets, header := []Row{sel}, targetHeader
	if k == strings.ToUpper(k) {
		targets, header = inTier(filterRows(m.rows, m.opts.repo), sel.Tier), categories[sel.Tier].agent
	}

	switch strings.ToLower(k) {
	case "o":
		m.status = openStatus(targets)
	case "m":
		if sel.Tier != tierReady {
			m.status = "can't merge: only ready PRs (approved, CI green, no conflicts) can be merged"
			return m, nil
		}
		m.confirm = targets
	case "a":
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
	if m.confirm != nil {
		lines := []string{fmt.Sprintf("Merge %d PR(s)?", len(m.confirm))}
		for i, r := range m.confirm {
			if i == maxStatus+3 {
				lines = append(lines, fmt.Sprintf("  … and %d more", len(m.confirm)-i))
				break
			}
			lines = append(lines, "  "+r.Ref+"  "+r.Title)
		}
		return strings.Join(append(lines, "[y] merge · [n]/esc cancel"), "\n")
	}
	help := "[o] open  [m] merge  [a] agent  [↑↓] move  [1-7] go to category  [?] help  [q] quit"
	if m.help {
		help = strings.Join([]string{
			"↑/↓ or j/k   move the cursor",
			"1-7          jump to a category (unfolds it)",
			"enter        fold / unfold the cursor's category",
			"o  m  a      open / merge / send to agent: the selected PR",
			"O  M  A      the same for every PR in the selected category",
			"esc          clear this message · q quit",
		}, "\n")
	}
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
	return renderWatch(m.opts, m.tick, m.rows, m.mergedTotal, m.errMsg, refreshStatus(m.lastFetch, m.nextRefresh), m.footer(), t,
		view{cursor: m.cursor, collapsed: m.collapsed, interactive: true})
}

func refreshStatus(lastFetch, nextRefresh time.Time) string {
	if lastFetch.IsZero() {
		return ""
	}
	secs := int(time.Until(nextRefresh).Seconds())
	if secs < 0 {
		secs = 0
	}
	return fmt.Sprintf("next refresh in %ds", secs)
}

// scopeLabel names what the dashboard is showing: the org (or all orgs) and,
// when set, the repo filter.
func scopeLabel(opts options) string {
	switch {
	case opts.org != "" && opts.repo != "":
		return opts.org + "/" + opts.repo
	case opts.org != "":
		return opts.org
	case opts.repo != "":
		return opts.repo
	}
	return "all orgs"
}

func renderWatch(opts options, tick int, rows []Row, mergedTotal int, errMsg, status, footer string, t terminal, v view) string {
	p := colors(t.useColor)
	m := gutter(t.width)
	inner := t.width - 2*m
	right := time.Now().Format("15:04:05")
	if status != "" {
		right += " · " + status
	}
	out := renderHeader(tick, t.useColor, inner, t.height, headerInfo{left: scopeLabel(opts), right: right})
	if errMsg != "" {
		out += p.r + "  ⚠ " + errMsg + p.z + "\n\n"
	}
	switch {
	case opts.asJSON:
	case len(rows) == 0 && errMsg == "" && status == "":
		out += p.d + "  loading…" + p.z + "\n"
	case len(rows) == 0 && errMsg != "":
		out += p.d + "  will retry on the next refresh" + p.z + "\n"
	default:
		filtered := filterRows(rows, opts.repo)
		if opts.repo != "" && len(filtered) == 0 {
			out += p.y + "  ⚠ --repo \"" + opts.repo + "\" matched no PRs" + p.z + "\n"
		}
		total := mergedTotal
		if opts.repo != "" {
			total = 0
		}
		out += renderPanels(filtered, inner, t.useColor, total, v)
	}

	footLines := strings.Split(footer, "\n")
	for i, l := range footLines {
		footLines[i] = p.d + "  " + truncate(l, inner-2) + p.z
	}

	// Keep the footer visible on short terminals by clipping the body instead.
	body := strings.Split(indent(out, m), "\n")
	if keep := t.height - len(footLines); keep > 0 && len(body) > keep {
		// Scroll just far enough that the cursor row stays on screen.
		start := 0
		for i, ln := range body {
			if strings.Contains(ln, "❯") {
				start = max(i-keep+1, 0)
				break
			}
		}
		body = body[start : start+keep]
	}
	return strings.Join(body, "\n") + "\n" + indent(strings.Join(footLines, "\n"), m)
}
