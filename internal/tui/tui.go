// Package tui provides ctrlz's interactive snapshot browser.
package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Aasthik17/ctrlz/internal/store"
	"github.com/Aasthik17/ctrlz/internal/watch"
)

const (
	pageSize     = 200
	refreshEvery = 5 * time.Second
	maxDiffBytes = 512 * 1024
)

var (
	accent = lipgloss.NewStyle().Foreground(lipgloss.Color("69")).Bold(true)
	muted  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	good   = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	bad    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	warn   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	panel  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("62")).Padding(0, 1)
)

// Run starts the TUI after synchronously validating and loading its initial
// history. Keeping this initial load outside Bubble Tea means callers get a
// normal CLI error instead of a blank alternate screen for bad projects.
func Run(project *store.Project, storePath string) error {
	data, err := load(project, storePath, pageSize)
	if err != nil {
		return err
	}
	m := newModel(project, storePath, data)
	_, err = tea.NewProgram(m).Run()
	return err
}

type historyData struct {
	snapshots []store.Snapshot
	total     int
	size      int64
	watching  bool
}

func load(project *store.Project, storePath string, limit int) (historyData, error) {
	snapshots, err := store.ListSnapshots(storePath, limit)
	if err != nil {
		return historyData{}, err
	}
	total, err := store.SnapshotCount(storePath)
	if err != nil {
		return historyData{}, err
	}
	size, err := store.StoreSize(storePath)
	if err != nil {
		return historyData{}, err
	}
	lockPath, err := store.WatchLockPath(project.ID)
	if err != nil {
		return historyData{}, err
	}
	watching, _, err := watch.Status(lockPath)
	if err != nil {
		return historyData{}, err
	}
	return historyData{snapshots: snapshots, total: total, size: size, watching: watching}, nil
}

type historyMsg struct {
	data historyData
	err  error
}
type diffMsg struct {
	request int
	diff    string
	err     error
}
type prepareMsg struct {
	plan    *store.UndoPlan
	release func()
	err     error
}
type restoreMsg struct{ err error }
type tickMsg time.Time

type model struct {
	project   *store.Project
	storePath string

	snapshots []store.Snapshot
	total     int
	size      int64
	watching  bool
	selected  int
	listTop   int
	diffTop   int
	diff      string
	diffReq   int

	width, height int
	busy          bool
	message       string
	err           error
	showHelp      bool
	confirm       bool
	plan          *store.UndoPlan
	release       func()
	input         textinput.Model
}

func newModel(project *store.Project, storePath string, data historyData) model {
	in := textinput.New()
	in.Prompt = "Type the short hash to confirm: "
	in.CharLimit = 12
	in.SetWidth(28)
	m := model{project: project, storePath: storePath, snapshots: data.snapshots, total: data.total, size: data.size, watching: data.watching, selected: 0, input: in}
	return m
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{tick()}
	if len(m.snapshots) > 0 {
		cmds = append(cmds, m.diffCmd())
	}
	return tea.Batch(cmds...)
}

func tick() tea.Cmd {
	return tea.Tick(refreshEvery, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) historyCmd(limit int) tea.Cmd {
	return func() tea.Msg {
		data, err := load(m.project, m.storePath, limit)
		return historyMsg{data: data, err: err}
	}
}

func (m model) diffCmd() tea.Cmd {
	if len(m.snapshots) == 0 || m.selected < 0 || m.selected >= len(m.snapshots) {
		return nil
	}
	hash, request := m.snapshots[m.selected].Hash, m.diffReq
	return func() tea.Msg {
		out, err := store.Diff(m.storePath, m.project.Path, hash, "")
		if err == nil && len(out) > maxDiffBytes {
			out = out[:maxDiffBytes] + "\n\n… diff truncated at 512 KiB; run ctrlz diff " + hash[:12] + " for full output.\n"
		}
		return diffMsg{request: request, diff: out, err: err}
	}
}

func (m model) prepareCmd() tea.Cmd {
	hash := m.snapshots[m.selected].Hash
	return func() tea.Msg {
		release, err := store.AcquireSnapshotLock(m.storePath, true)
		if err != nil {
			return prepareMsg{err: err}
		}
		plan, err := store.PrepareUndo(m.storePath, m.project.Path, hash)
		if err != nil {
			release()
			return prepareMsg{err: err}
		}
		return prepareMsg{plan: plan, release: release}
	}
}

func (m model) restoreCmd() tea.Cmd {
	plan, release := m.plan, m.release
	return func() tea.Msg {
		err := store.ApplyUndo(m.storePath, m.project.Path, plan.Target)
		release()
		return restoreMsg{err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		if m.confirm || m.busy {
			return m, tick()
		}
		m.busy, m.message = true, "Refreshing history…"
		return m, tea.Batch(m.historyCmd(len(m.snapshots)), tick())
	case historyMsg:
		m.busy = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		selectedHash := ""
		if len(m.snapshots) > 0 && m.selected < len(m.snapshots) {
			selectedHash = m.snapshots[m.selected].Hash
		}
		m.snapshots, m.total, m.size, m.watching = msg.data.snapshots, msg.data.total, msg.data.size, msg.data.watching
		m.selected = 0
		for i, s := range m.snapshots {
			if s.Hash == selectedHash {
				m.selected = i
				break
			}
		}
		m.message, m.err = fmt.Sprintf("Showing %d of %d snapshots", len(m.snapshots), m.total), nil
		m.diffTop, m.diffReq = 0, m.diffReq+1
		return m, m.diffCmd()
	case diffMsg:
		if msg.request != m.diffReq {
			return m, nil
		}
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.diff, m.err = msg.diff, nil
		return m, nil
	case prepareMsg:
		m.busy = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.plan, m.release, m.confirm, m.err = msg.plan, msg.release, true, nil
		m.input.SetValue("")
		return m, m.input.Focus()
	case restoreMsg:
		m.busy, m.confirm, m.plan, m.release = false, false, nil, nil
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.message, m.err = "Restore complete. History reloaded.", nil
		return m, m.historyCmd(len(m.snapshots))
	case tea.MouseClickMsg:
		mouse := msg.Mouse()
		// Header occupies three lines; rows begin immediately below it.
		if mouse.X < m.timelineWidth() && mouse.Y >= 3 {
			index := m.listTop + mouse.Y - 3
			if index < len(m.snapshots) {
				m.selectSnapshot(index)
				return m, m.diffCmd()
			}
			if index == len(m.snapshots) && len(m.snapshots) < m.total && !m.busy {
				m.busy, m.message = true, "Loading older snapshots…"
				return m, m.historyCmd(min(len(m.snapshots)+pageSize, m.total))
			}
		}
		// The restore affordance is the final line of the Details pane.
		if m.width >= 120 && mouse.X >= m.timelineWidth() && mouse.X < m.timelineWidth()+m.width*23/100 && mouse.Y >= 10 && !m.busy && len(m.snapshots) > 0 {
			m.busy, m.message = true, "Saving safety snapshot and preparing restore…"
			return m, m.prepareCmd()
		}
	case tea.MouseWheelMsg:
		mouse := msg.Mouse()
		if mouse.Button == tea.MouseWheelDown {
			m.move(3)
		} else {
			m.move(-3)
		}
		return m, m.diffCmd()
	case tea.KeyPressMsg:
		key := msg.String()
		if m.confirm {
			return m.updateConfirm(msg, key)
		}
		if m.showHelp {
			if key == "esc" || key == "?" || key == "q" {
				m.showHelp = false
			}
			return m, nil
		}
		switch key {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		case "?":
			m.showHelp = true
		case "j", "down":
			m.move(1)
			return m, m.diffCmd()
		case "k", "up":
			m.move(-1)
			return m, m.diffCmd()
		case "pgdown", "ctrl+d":
			m.move(m.timelineRows())
			return m, m.diffCmd()
		case "pgup", "ctrl+u":
			m.move(-m.timelineRows())
			return m, m.diffCmd()
		case "r":
			if !m.busy {
				m.busy, m.message = true, "Refreshing history…"
				return m, m.historyCmd(len(m.snapshots))
			}
		case "l":
			if !m.busy && len(m.snapshots) < m.total {
				m.busy, m.message = true, "Loading older snapshots…"
				return m, m.historyCmd(min(len(m.snapshots)+pageSize, m.total))
			}
		case "u":
			if !m.busy && len(m.snapshots) > 0 {
				m.busy, m.message = true, "Saving safety snapshot and preparing restore…"
				return m, m.prepareCmd()
			}
		case "right", "ctrl+f":
			m.diffTop += max(1, m.diffRows()/2)
		case "left", "ctrl+b":
			m.diffTop = max(0, m.diffTop-max(1, m.diffRows()/2))
		}
	}
	return m, nil
}

func (m model) updateConfirm(msg tea.Msg, key string) (tea.Model, tea.Cmd) {
	if key == "esc" {
		m.confirm, m.plan = false, nil
		if m.release != nil {
			m.release()
			m.release = nil
		}
		m.message = "Restore cancelled; the safety snapshot was kept."
		return m, nil
	}
	if key == "enter" && m.plan != nil && m.input.Value() == short(m.plan.Target) {
		m.busy = true
		return m, m.restoreCmd()
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *model) move(delta int) {
	if len(m.snapshots) == 0 {
		return
	}
	m.selectSnapshot(m.selected + delta)
}

func (m *model) selectSnapshot(index int) {
	if index < 0 || index >= len(m.snapshots) {
		return
	}
	m.selected, m.diffTop, m.diffReq = index, 0, m.diffReq+1
	rows := m.timelineRows()
	if m.selected < m.listTop {
		m.listTop = m.selected
	}
	if m.selected >= m.listTop+rows {
		m.listTop = m.selected - rows + 1
	}
}

func (m model) View() tea.View {
	content := m.render()
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "ctrlz — snapshot recovery"
	return v
}

func (m model) render() string {
	if m.width < 80 || m.height < 18 {
		return accent.Render("ctrlz interactive recovery") + "\n\nResize this terminal to at least 80×18 to use the snapshot browser.\n\nPress q to quit."
	}
	header := accent.Render("ctrlz") + "  " + muted.Render("interactive recovery") + "\n" +
		m.project.Path + "\n" + muted.Render(fmt.Sprintf("%d snapshots • %s • watch: %s", m.total, humanSize(m.size), yesNo(m.watching)))
	var body string
	if m.width >= 120 {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.timelinePane(m.width*31/100, m.height-7), m.detailsPane(m.width*23/100, m.height-7), m.diffPane(m.width*46/100, m.height-7))
	} else {
		right := lipgloss.JoinVertical(lipgloss.Left, m.detailsPane(m.width*62/100, 7), m.diffPane(m.width*62/100, m.height-14))
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.timelinePane(m.width*38/100, m.height-7), right)
	}
	footer := muted.Render("↑↓/j k select  •  u restore  •  l older  •  r refresh  •  ? help  •  q quit")
	if m.busy {
		footer = warn.Render("● " + m.message)
	}
	if m.err != nil {
		footer = bad.Render("Error: " + m.err.Error())
	}
	if !m.busy && m.err == nil && m.message != "" {
		footer = good.Render(m.message) + "  " + footer
	}
	out := header + "\n" + body + "\n" + footer
	if m.showHelp {
		return out + "\n\n" + m.helpPane()
	}
	if m.confirm {
		return out + "\n\n" + m.confirmPane()
	}
	return out
}

func (m model) timelineWidth() int {
	if m.width >= 120 {
		return m.width*31/100 + 2
	}
	return m.width*38/100 + 2
}
func (m model) timelineRows() int { return max(3, m.height-11) }
func (m model) diffRows() int     { return max(3, m.height-12) }

func (m model) timelinePane(width, height int) string {
	rows := []string{accent.Render("SNAPSHOT TIMELINE")}
	end := min(len(m.snapshots), m.listTop+max(1, height-3))
	for i := m.listTop; i < end; i++ {
		s := m.snapshots[i]
		line := fmt.Sprintf("%s  %s  %s", short(s.Hash), s.Timestamp.Local().Format("15:04:05"), s.Reason)
		if i == m.selected {
			line = lipgloss.NewStyle().Background(lipgloss.Color("62")).Bold(true).Render(trim(line, width-4))
		} else {
			line = trim(line, width-4)
		}
		rows = append(rows, line)
	}
	if len(m.snapshots) == 0 {
		rows = append(rows, muted.Render("No snapshots yet."))
	}
	if len(m.snapshots) < m.total {
		rows = append(rows, accent.Render(fmt.Sprintf("[l] Load older (%d remaining)", m.total-len(m.snapshots))))
	}
	return panel.Width(max(20, width-2)).Height(max(3, height-2)).Render(strings.Join(rows, "\n"))
}

func (m model) detailsPane(width, height int) string {
	rows := []string{accent.Render("SELECTED SNAPSHOT")}
	if len(m.snapshots) == 0 {
		rows = append(rows, muted.Render("Nothing to inspect."))
	} else {
		s := m.snapshots[m.selected]
		rows = append(rows, "Hash: "+short(s.Hash), "Time: "+s.Timestamp.Local().Format(time.RFC1123), "Reason: "+s.Reason, "Stat: "+s.Stat, muted.Render(fmt.Sprintf("%d of %d", m.selected+1, m.total)), "", accent.Render("[u] Restore this snapshot"))
	}
	return panel.Width(max(20, width-2)).Height(max(3, height-2)).Render(strings.Join(rows, "\n"))
}

func (m model) diffPane(width, height int) string {
	rows := []string{accent.Render("DIFF — SELECTED SNAPSHOT → CURRENT WORKTREE")}
	if len(m.snapshots) == 0 {
		rows = append(rows, muted.Render("Select a snapshot when history exists."))
	} else if m.diff == "" {
		rows = append(rows, muted.Render("No differences from the current working tree."))
	} else {
		lines := strings.Split(m.diff, "\n")
		mTop := min(m.diffTop, max(0, len(lines)-1))
		for _, line := range lines[mTop:min(len(lines), mTop+max(1, height-3))] {
			rows = append(rows, colorDiff(trim(line, width-4)))
		}
	}
	return panel.Width(max(24, width-2)).Height(max(3, height-2)).Render(strings.Join(rows, "\n"))
}

func (m model) helpPane() string {
	return panel.Width(min(76, m.width-4)).Render(accent.Render("KEYBOARD & MOUSE HELP") + "\n\n" +
		"↑/↓ or j/k  select a snapshot\nPgUp/PgDn, Ctrl+u/Ctrl+d  move through history\nMouse click / wheel  select and scroll\nu  prepare a safe restore    l  load 200 older snapshots\nr  refresh now                q/Esc  close this screen\n\nThe diff always compares the selected snapshot to files on disk now.")
}

func (m model) confirmPane() string {
	if m.plan == nil {
		return ""
	}
	p := m.plan
	text := accent.Render("CONFIRM RESTORE") + "\n\n" +
		"A safety snapshot of the current state has been saved.\n" +
		fmt.Sprintf("Restore %s: restore %d, remove %d, modify %d files.\n\n", short(p.Target), p.Summary.ToRestore, p.Summary.ToRemove, p.Summary.ToModify) +
		m.input.View() + "\n" + muted.Render("Press Enter only after the hash matches. Esc cancels.")
	return panel.BorderForeground(lipgloss.Color("214")).Width(min(78, m.width-4)).Render(text)
}

func colorDiff(line string) string {
	switch {
	case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
		return accent.Render(line)
	case strings.HasPrefix(line, "+"):
		return good.Render(line)
	case strings.HasPrefix(line, "-"):
		return bad.Render(line)
	case strings.HasPrefix(line, "@@"):
		return warn.Render(line)
	default:
		return line
	}
}
func short(hash string) string {
	if len(hash) > 12 {
		return hash[:12]
	}
	return hash
}
func trim(s string, n int) string {
	if n <= 1 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
func humanSize(n int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d %s", n, units[i])
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}
func yesNo(v bool) string {
	if v {
		return "active"
	}
	return "not active"
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
