// Package tui is the Bubble Tea interface. State is always re-read from
// the op log after a write, so the TUI never diverges from the CLI.
package tui

import (
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/CMonnin/dwkt/internal/core"
	"github.com/CMonnin/dwkt/internal/editor"
	"github.com/CMonnin/dwkt/internal/session"
)

type tab int

const (
	tabWeek tab = iota
	tabBacklog
	tabProjects
	tabHistory
)

var tabNames = []string{"Week", "Backlog", "Projects", "History"}

type row struct {
	header  string
	task    *core.Task
	project *core.Project
}

func (r row) selectable() bool { return r.header == "" }

func (r row) id() string {
	switch {
	case r.task != nil:
		return r.task.ID
	case r.project != nil:
		return r.project.ID
	}
	return ""
}

type mode int

const (
	modeNormal mode = iota
	modeAdd
	modeSearch
	modeTags
	modeBlock
	modeNote
	modeProjectAdd
	modeRename
	modeDescribe
	modeConfirmArchive
)

type (
	pushDoneMsg   struct{}
	syncDoneMsg   struct{ err error }
	editorDoneMsg struct {
		id, path string
		err      error
	}
)

type Model struct {
	s   *session.Session
	st  *core.State
	now func() time.Time

	tab            tab
	rows           []row
	cursor, offset int
	width, height  int
	selectID       string // row to select on next rebuild

	mode   mode
	input  textinput.Model
	target string // task or project ID the input applies to

	search        string
	projectFilter string

	status             session.SyncStatus
	pushing, pushAgain bool
	syncing            bool
	flash              string

	keys keyMap
	help help.Model
}

func Run(s *session.Session) error {
	m, err := New(s)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func New(s *session.Session) (*Model, error) {
	m := &Model{s: s, now: time.Now, keys: newKeys(), help: help.New(), input: textinput.New()}
	m.input.CharLimit = 500
	if err := m.reload(); err != nil {
		return nil, err
	}
	m.status = s.Status()
	m.jumpToLeftover()
	return m, nil
}

// Init pushes anything the startup pull committed (e.g. recovered ops).
func (m *Model) Init() tea.Cmd {
	if m.status.Unpushed > 0 {
		return m.push()
	}
	return nil
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.Width = msg.Width
		m.input.Width = msg.Width - 20
		return m, nil
	case pushDoneMsg:
		m.pushing = false
		m.status = m.s.Status()
		if m.pushAgain {
			m.pushAgain = false
			return m, m.push()
		}
		return m, nil
	case syncDoneMsg:
		m.syncing = false
		m.flash = "synced"
		if msg.err != nil {
			m.flash = "sync failed (offline?): " + firstLine(msg.err.Error())
		}
		m.reloadOrFlash()
		m.status = m.s.Status()
		return m, nil
	case editorDoneMsg:
		return m, m.finishEditor(msg)
	case tea.KeyMsg:
		if m.mode != modeNormal {
			return m, m.updateInput(msg)
		}
		return m, m.updateNormal(msg)
	}
	if m.mode != modeNormal {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

// --- state

func (m *Model) reload() error {
	st, err := m.s.Load()
	if err != nil {
		return err
	}
	m.st = st
	if len(m.s.Warnings) > 0 {
		m.flash = "warning: " + m.s.Warnings[0]
	}
	m.buildRows()
	return nil
}

func (m *Model) reloadOrFlash() {
	if err := m.reload(); err != nil {
		m.flash = "error: " + err.Error()
	}
}

// apply persists ops, reloads, and schedules a background push.
func (m *Model) apply(ops []core.Op, msg string) tea.Cmd {
	if len(ops) == 0 {
		return nil
	}
	gitErr, err := m.s.Commit(ops, msg)
	if err != nil {
		m.flash = "error: " + err.Error()
		return nil
	}
	if gitErr != nil {
		m.flash = "saved, but git commit failed: " + firstLine(gitErr.Error())
	}
	m.reloadOrFlash()
	m.status = m.s.Status()
	return m.push()
}

func (m *Model) push() tea.Cmd {
	if m.pushing {
		m.pushAgain = true
		return nil
	}
	m.pushing = true
	s := m.s
	return func() tea.Msg {
		_ = s.Push() // failure just leaves commits unpushed
		return pushDoneMsg{}
	}
}

func (m *Model) sync() tea.Cmd {
	if m.syncing {
		return nil
	}
	m.syncing = true
	s := m.s
	return func() tea.Msg { return syncDoneMsg{errors.Join(s.Pull(), s.Push())} }
}

func (m *Model) leftovers() []*core.Task { return m.st.Leftovers(m.now(), m.s.Loc) }

func (m *Model) isLeftover(t *core.Task) bool { return core.IsLeftover(t, m.now(), m.s.Loc) }

// --- rows and cursor

func (m *Model) buildRows() {
	want := m.selectID
	m.selectID = ""
	if want == "" {
		if r := m.selected(); r != nil {
			want = r.id()
		}
	}
	oldCursor := m.cursor
	now := m.now()
	var rows []row
	group := func(title string, tasks []*core.Task, in func(*core.Task) bool) {
		first := true
		for _, t := range tasks {
			if !in(t) {
				continue
			}
			if first {
				rows = append(rows, row{header: title})
				first = false
			}
			rows = append(rows, row{task: t})
		}
	}

	switch m.tab {
	case tabWeek:
		tasks := m.filter(m.st.WeekTasks(now, m.s.Loc))
		group("Work", tasks, func(t *core.Task) bool { return m.st.EffectiveCategory(t) == core.Work })
		group("Personal", tasks, func(t *core.Task) bool { return m.st.EffectiveCategory(t) == core.Personal })
	case tabBacklog:
		tasks := m.filter(m.st.BacklogTasks())
		if m.projectFilter == "" {
			group("Inbox", tasks, func(t *core.Task) bool { return t.Project == "" })
		}
		for _, p := range m.st.SortedProjects(false) {
			if m.projectFilter == "" || m.projectFilter == p.ID {
				group(p.Name, tasks, func(t *core.Task) bool { return t.Project == p.ID })
			}
		}
	case tabProjects:
		for _, p := range m.st.SortedProjects(false) {
			rows = append(rows, row{project: p})
		}
	case tabHistory:
		for _, t := range m.filter(m.st.HistoryTasks()) {
			rows = append(rows, row{task: t})
		}
	}
	m.rows = rows

	m.cursor = -1
	for i, r := range rows {
		if r.selectable() && want != "" && r.id() == want {
			m.cursor = i
			return
		}
	}
	// Selection vanished: stay near the old position.
	oldCursor = min(max(oldCursor, 0), len(rows)-1)
	for i := oldCursor; i >= 0 && i < len(rows); i++ {
		if rows[i].selectable() {
			m.cursor = i
			return
		}
	}
	for i := oldCursor; i >= 0; i-- {
		if rows[i].selectable() {
			m.cursor = i
			return
		}
	}
}

// filter applies the search: "#tag" words must be tags, others must appear in the title.
func (m *Model) filter(tasks []*core.Task) []*core.Task {
	if strings.TrimSpace(m.search) == "" {
		return tasks
	}
	var out []*core.Task
	for _, t := range tasks {
		if matchSearch(t, m.search) {
			out = append(out, t)
		}
	}
	return out
}

func matchSearch(t *core.Task, q string) bool {
	title := strings.ToLower(t.Title)
	for _, w := range strings.Fields(strings.ToLower(q)) {
		if strings.HasPrefix(w, "#") {
			if tag := core.NormalizeTag(w); tag != "" && !t.HasTag(tag) {
				return false
			}
		} else if !strings.Contains(title, w) {
			return false
		}
	}
	return true
}

func (m *Model) selected() *row {
	if m.cursor >= 0 && m.cursor < len(m.rows) && m.rows[m.cursor].selectable() {
		return &m.rows[m.cursor]
	}
	return nil
}

func (m *Model) selectedTask() *core.Task {
	if r := m.selected(); r != nil {
		return r.task
	}
	return nil
}

func (m *Model) selectedProject() *core.Project {
	if r := m.selected(); r != nil {
		return r.project
	}
	return nil
}

func (m *Model) move(delta int) {
	for i := m.cursor + delta; i >= 0 && i < len(m.rows); i += delta {
		if m.rows[i].selectable() {
			m.cursor = i
			return
		}
	}
}

func (m *Model) setTab(t tab) {
	if t != m.tab {
		m.search = ""
		if t != tabBacklog {
			m.projectFilter = ""
		}
	}
	m.tab, m.cursor, m.offset = t, 0, 0
	m.buildRows()
	if m.selected() == nil {
		m.move(1)
	}
	m.jumpToLeftover()
}

func (m *Model) jumpToLeftover() {
	if m.tab != tabWeek {
		return
	}
	for i, r := range m.rows {
		if r.task != nil && m.isLeftover(r.task) {
			m.cursor = i
			return
		}
	}
}

// --- keys

func (m *Model) updateNormal(msg tea.KeyMsg) tea.Cmd {
	m.flash = ""
	k, now := m.keys, m.now()
	t, p := m.selectedTask(), m.selectedProject()

	// Leftover triage on the Week tab takes precedence over navigation.
	if m.tab == tabWeek {
		if t != nil && m.isLeftover(t) {
			var cmd tea.Cmd
			switch msg.String() {
			case "k":
				cmd = m.apply(core.MoveToWeek(t, now), "keep: "+t.Title)
			case "d":
				cmd = m.apply(core.Defer(t, now), "defer: "+t.Title)
			case "D":
				cmd = m.apply(core.SetStatus(t, core.StatusDropped, "", now), "drop: "+t.Title)
			}
			if cmd != nil {
				m.jumpToLeftover()
				return cmd
			}
		}
		if left := m.leftovers(); len(left) > 0 && msg.String() == "esc" {
			var ops []core.Op
			for _, lt := range left {
				ops = append(ops, core.MoveToWeek(lt, now)...)
			}
			m.flash = "kept all leftovers"
			return m.apply(ops, "keep all leftovers")
		}
	}

	switch {
	case key.Matches(msg, k.Quit):
		return tea.Quit
	case key.Matches(msg, k.Help):
		m.help.ShowAll = !m.help.ShowAll
	case key.Matches(msg, k.Tabs):
		m.setTab(tab(msg.String()[0] - '1'))
	case key.Matches(msg, k.NextTab):
		m.setTab((m.tab + 1) % 4)
	case key.Matches(msg, k.PrevTab):
		m.setTab((m.tab + 3) % 4)
	case key.Matches(msg, k.Down):
		m.move(1)
	case key.Matches(msg, k.Up):
		m.move(-1)
	case key.Matches(msg, k.Top):
		m.cursor = -1
		m.move(1)
	case key.Matches(msg, k.Bottom):
		m.cursor = len(m.rows)
		m.move(-1)
	case key.Matches(msg, k.Sync):
		return m.sync()
	case key.Matches(msg, k.Esc):
		m.search, m.projectFilter = "", ""
		m.buildRows()

	case key.Matches(msg, k.Add):
		switch m.tab {
		case tabWeek:
			return m.startInput(modeAdd, "add to week: ", "", "")
		case tabBacklog:
			where := "backlog"
			if m.projectFilter != "" {
				where = m.st.ProjectName(m.projectFilter)
			}
			return m.startInput(modeAdd, "add to "+where+": ", "", "")
		case tabProjects:
			return m.startInput(modeProjectAdd, "new project: ", "", "")
		default:
			m.flash = "switch to Week or Backlog to add tasks"
		}
	case key.Matches(msg, k.Search) && m.tab != tabProjects:
		return m.startInput(modeSearch, "/", m.search, "")

	case key.Matches(msg, k.Open) && p != nil:
		m.setTab(tabBacklog)
		m.projectFilter = p.ID
		m.buildRows()
		m.cursor = -1
		m.move(1)
	case key.Matches(msg, k.Rename) && p != nil:
		return m.startInput(modeRename, "rename: ", p.Name, p.ID)
	case key.Matches(msg, k.Describe) && p != nil:
		return m.startInput(modeDescribe, "description: ", p.Description, p.ID)
	case key.Matches(msg, k.Archive) && p != nil:
		m.mode, m.target = modeConfirmArchive, p.ID
	case key.Matches(msg, k.Category) && p != nil:
		return m.apply(core.SetProjectCategory(p, otherCategory(p.Category), now), "project category: "+p.Name)
	}

	if t == nil {
		return nil
	}
	switch {
	case key.Matches(msg, k.Done):
		if m.tab == tabHistory {
			return m.apply(core.Reopen(t, now), "reopen: "+t.Title)
		}
		return m.apply(core.ToggleDone(t, now), "toggle done: "+t.Title)
	case key.Matches(msg, k.Doing):
		st := core.StatusDoing
		if t.Status == core.StatusDoing {
			st = core.StatusTodo
		}
		return m.apply(core.SetStatus(t, st, "", now), string(st)+": "+t.Title)
	case key.Matches(msg, k.Block):
		return m.startInput(modeBlock, "blocked because (enter to skip): ", t.BlockedReason, t.ID)
	case key.Matches(msg, k.Drop) && t.Status.Open():
		return m.apply(core.SetStatus(t, core.StatusDropped, "", now), "drop: "+t.Title)
	case key.Matches(msg, k.Week) && m.tab == tabBacklog:
		m.flash = "moved to week: " + t.Title
		return m.apply(core.MoveToWeek(t, now), "week: "+t.Title)
	case (key.Matches(msg, k.Week) || key.Matches(msg, k.Defer)) && m.tab == tabWeek && t.Status.Open():
		m.flash = "moved to backlog: " + t.Title
		return m.apply(core.Defer(t, now), "defer: "+t.Title)
	case key.Matches(msg, k.Tags):
		prefill := strings.Join(t.Tags, " ")
		if prefill != "" {
			prefill += " "
		}
		cmd := m.startInput(modeTags, "tags: ", prefill, t.ID)
		m.updateTagSuggestions()
		return cmd
	case key.Matches(msg, k.Note):
		if strings.Contains(t.Note, "\n") {
			m.flash = "note has several lines; edit it with E"
			return nil
		}
		return m.startInput(modeNote, "note: ", t.Note, t.ID)
	case key.Matches(msg, k.Editor):
		return m.startEditor(t)
	case key.Matches(msg, k.Category):
		ops, err := core.SetCategory(t, otherCategory(t.Category), now)
		if err != nil {
			m.flash = err.Error()
			return nil
		}
		return m.apply(ops, "category: "+t.Title)
	}
	return nil
}

func otherCategory(c core.Category) core.Category {
	if c == core.Work {
		return core.Personal
	}
	return core.Work
}

// --- inputs

func (m *Model) startInput(md mode, prompt, value, target string) tea.Cmd {
	m.mode, m.target = md, target
	m.input.Prompt = prompt
	m.input.ShowSuggestions = md == modeTags
	m.input.SetSuggestions(nil)
	m.input.SetValue(value)
	m.input.CursorEnd()
	return m.input.Focus()
}

func (m *Model) updateInput(msg tea.KeyMsg) tea.Cmd {
	if m.mode == modeConfirmArchive {
		m.mode = modeNormal
		p := m.st.Projects[m.target]
		if msg.String() == "y" && p != nil {
			m.flash = "archived " + p.Name
			return m.apply(core.ArchiveProject(m.st, p, m.now()), "project archive: "+p.Name)
		}
		return nil
	}
	switch msg.Type {
	case tea.KeyEsc:
		if m.mode == modeSearch {
			m.search = ""
			m.buildRows()
		}
		m.mode = modeNormal
		m.input.Blur()
		return nil
	case tea.KeyEnter:
		md, val := m.mode, m.input.Value()
		m.mode = modeNormal
		m.input.Blur()
		return m.submit(md, val)
	case tea.KeyCtrlC:
		return tea.Quit
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	switch m.mode {
	case modeTags:
		m.updateTagSuggestions()
	case modeSearch:
		m.search = m.input.Value()
		m.buildRows()
	}
	return cmd
}

// updateTagSuggestions offers completions for the word being typed.
func (m *Model) updateTagSuggestions() {
	v := m.input.Value()
	i := strings.LastIndexAny(v, " ,") + 1
	prefix, last := v[:i], strings.ToLower(strings.TrimLeft(v[i:], "#"))
	present := core.SplitTags(prefix)
	var sugg []string
	for _, tag := range m.st.AllTags() {
		if !slices.Contains(present, tag) && strings.HasPrefix(tag, last) {
			sugg = append(sugg, prefix+tag)
		}
	}
	m.input.SetSuggestions(sugg)
}

func (m *Model) submit(md mode, val string) tea.Cmd {
	now := m.now()
	t, p := m.st.Tasks[m.target], m.st.Projects[m.target]
	fail := func(err error) tea.Cmd { m.flash = err.Error(); return nil }

	switch md {
	case modeSearch:
		m.search = val
		m.buildRows()
	case modeAdd:
		title, tags := core.ParseTitle(val)
		if title == "" {
			return nil
		}
		in := core.NewTask{Title: title, Tags: tags, Category: m.s.DefaultCategory()}
		if m.tab == tabBacklog {
			in.Backlog = true
			if m.projectFilter != "" {
				in.Project, in.Category = m.projectFilter, ""
			}
		}
		op, err := core.CreateTask(m.st, in, now)
		if err != nil {
			return fail(err)
		}
		m.selectID = op.Target
		return m.apply([]core.Op{op}, "add: "+title)
	case modeProjectAdd:
		op, err := core.CreateProject(m.st, val, m.s.DefaultCategory(), "", now)
		if err != nil {
			return fail(err)
		}
		m.selectID = op.Target
		return m.apply([]core.Op{op}, "project add: "+op.Project.Name)
	case modeTags:
		if t == nil {
			return nil
		}
		want := core.SplitTags(val)
		var add, remove []string
		for _, tag := range want {
			if !t.HasTag(tag) {
				add = append(add, tag)
			}
		}
		for _, tag := range t.Tags {
			if !slices.Contains(want, tag) {
				remove = append(remove, tag)
			}
		}
		return m.apply(core.EditTags(t, add, remove, now), "tag: "+t.Title)
	case modeBlock:
		if t != nil {
			return m.apply(core.SetStatus(t, core.StatusBlocked, val, now), "block: "+t.Title)
		}
	case modeNote:
		if t != nil && strings.TrimSpace(val) != t.Note {
			return m.apply(core.SetNote(t, val, now), "note: "+t.Title)
		}
	case modeRename:
		if p != nil && val != p.Name {
			ops, err := core.RenameProject(m.st, p, val, now)
			if err != nil {
				return fail(err)
			}
			return m.apply(ops, "project rename: "+val)
		}
	case modeDescribe:
		if p != nil && val != p.Description {
			return m.apply(core.DescribeProject(p, val, now), "project describe: "+p.Name)
		}
	}
	return nil
}

// --- $EDITOR

func (m *Model) startEditor(t *core.Task) tea.Cmd {
	cmd, path, err := editor.Prepare(core.TaskForm(m.st, t))
	if err != nil {
		m.flash = err.Error()
		return nil
	}
	id := t.ID
	return tea.ExecProcess(cmd, func(err error) tea.Msg { return editorDoneMsg{id: id, path: path, err: err} })
}

func (m *Model) finishEditor(msg editorDoneMsg) tea.Cmd {
	text, err := editor.Finish(msg.path)
	if msg.err != nil {
		err = msg.err
	}
	if err != nil {
		m.flash = "editor: " + err.Error()
		return nil
	}
	m.reloadOrFlash()
	t := m.st.Tasks[msg.id]
	if t == nil {
		return nil
	}
	ops, err := core.ApplyTaskForm(m.st, t, text, m.now())
	if err != nil {
		m.flash = "not saved: " + err.Error()
		return nil
	}
	if len(ops) == 0 {
		m.flash = "no changes"
		return nil
	}
	return m.apply(ops, "edit: "+t.Title)
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	return s
}
