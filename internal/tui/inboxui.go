package tui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/CMonnin/dkt/internal/core"
	"github.com/CMonnin/dkt/internal/plugin"
)

// The inbox screen and plugin polling. The inbox's contents live in the
// inbox type (inbox.go); this file connects it to the Model.

type (
	pollMsg     struct{ name string } // time to poll this plugin
	pollDoneMsg struct {
		name string
		list plugin.List
		err  error
	}
	actDoneMsg struct {
		name, id, verb string
		err            error
	}
	linkedDoneMsg struct{}
)

const retryAfter = 60 * time.Second // after a failed poll

func (m *Model) poll(name string) tea.Cmd {
	cursor := m.ib.cursor(name)
	return func() tea.Msg {
		l, err := plugin.Inbox(context.Background(), name, cursor)
		return pollDoneMsg{name, l, err}
	}
}

func (m *Model) pollDone(msg pollDoneMsg) tea.Cmd {
	wasFailing := m.ib.failed[msg.name]
	m.ib.update(msg.name, msg.list, msg.err)
	var cmds []tea.Cmd
	switch {
	case msg.err != nil:
		if !wasFailing { // say why once, then retry quietly
			m.flash = "inbox: " + firstLine(msg.err.Error())
		}
	case !msg.list.Unchanged:
		for _, e := range m.ib.relinks(m.st) {
			cmds = append(cmds, linked(e))
		}
	}
	if !m.ib.stopped(msg.name) {
		wait := msg.list.PollAfter
		if msg.err != nil || wait == 0 {
			wait = retryAfter
		}
		name := msg.name
		cmds = append(cmds, tea.Tick(wait, func(time.Time) tea.Msg { return pollMsg{name} }))
	}
	if !m.touched && !m.inInbox && len(m.leftovers()) == 0 && m.inboxCount() > 0 {
		m.openInbox()
	} else if m.inInbox {
		m.buildRows()
	}
	return tea.Batch(cmds...)
}

// linked tells the plugin, in the background, that an item now has a task.
// If it fails, the next poll brings the item back as handled and it runs
// again.
func linked(e entry) tea.Cmd {
	return func() tea.Msg {
		_ = plugin.Linked(context.Background(), e.plugin, e.item.ID, e.task.ID)
		return linkedDoneMsg{}
	}
}

func act(name, id, verb string) tea.Cmd {
	return func() tea.Msg {
		return actDoneMsg{name, id, verb, plugin.Act(context.Background(), name, id, verb)}
	}
}

func (m *Model) actDone(msg actDoneMsg) {
	if msg.err != nil {
		m.flash = firstLine(msg.err.Error())
		return
	}
	m.ib.remove(msg.name, msg.id)
	if m.inInbox {
		m.buildRows()
	}
}

func (m *Model) inboxCount() int {
	n := 0
	for _, r := range m.ib.rows(m.st) {
		if r.inbox != nil {
			n++
		}
	}
	return n
}

func (m *Model) openInbox() {
	m.inInbox, m.search, m.projectFilter = true, "", ""
	m.cursor, m.offset = 0, 0
	m.buildRows()
	if m.selected() == nil {
		m.move(1)
	}
}

func (m *Model) closeInbox() {
	m.inInbox = false
	m.setTab(tabWeek)
}

// updateInbox handles a key on the inbox screen. It reports false for keys
// it leaves to the normal handler (navigation, help, quit).
func (m *Model) updateInbox(msg tea.KeyMsg) (tea.Cmd, bool) {
	key := msg.String()
	if key == "esc" {
		m.closeInbox()
		return nil, true
	}
	r := m.selected()
	if r == nil || r.inbox == nil {
		return nil, false
	}
	e := *r.inbox
	update := e.task != nil
	switch key {
	case "t", "b":
		if update {
			m.flash = "already a task: " + e.task.Title
			return nil, true
		}
		return m.linkItem(e, e.item.Task.Title, key == "b"), true
	case "T", "B":
		if update {
			m.flash = "already a task: " + e.task.Title
			return nil, true
		}
		m.pending, m.pendingBacklog = &e, key == "B"
		return m.startInput(modeLinkTitle, "task title: ", e.item.Task.Title, ""), true
	case "z":
		m.ib.postpone(e.plugin, e.item.ID)
		m.buildRows()
		return nil, true
	case "o":
		if e.item.URL == "" {
			m.flash = "no link for this item"
			return nil, true
		}
		return openURL(e.item.URL), true
	case "r":
		switch {
		case e.state == core.ItemHiddenActivity:
			m.flash = "now a task, in the backlog: " + e.task.Title
			return m.apply(core.TaskFromHidden(e.task, e.plugin, m.now()), "unhide as task: "+e.task.Title), true
		case update && !e.task.Status.Open():
			m.flash = "reopened: " + e.task.Title
			return m.apply(core.Reopen(e.task, m.now()), "reopen: "+e.task.Title), true
		}
		return nil, true
	case "h":
		return m.hideItem(e), true
	}
	if !e.dismissible() {
		return nil, false
	}
	for _, v := range m.ib.verbs(e.plugin) {
		if v.Key == key {
			return act(e.plugin, e.item.ID, v.Key), true
		}
	}
	for _, v := range m.ib.groupVerbs(e.plugin) {
		if v.Key == key {
			group := m.cursorGroup()
			if v.Confirm {
				m.mode, m.pendingGroup, m.pendingVerb = modeConfirmGroup, group, v
				return nil, true
			}
			return m.actGroup(group, v.Verb), true
		}
	}
	return nil, false
}

// linkRequest is what LinkItem and HideItem need for an entry.
func (m *Model) linkRequest(e entry, title string, backlog bool) core.LinkRequest {
	sug := e.item.Task
	cat, err := core.ParseCategory(sug.Category)
	if err != nil {
		cat = m.s.DefaultCategory()
	}
	r := core.LinkRequest{
		Plugin: e.plugin, Key: e.item.Key,
		Title: title, Note: sug.Note, Category: cat, Tags: sug.Tags, Backlog: backlog,
	}
	if sug.Project != nil {
		r.ProjectKey, r.ProjectName = sug.Project.Key, sug.Project.Name
	}
	return r
}

// linkItem makes a linked task from an item, then tells the plugin.
func (m *Model) linkItem(e entry, title string, backlog bool) tea.Cmd {
	ops, id, err := core.LinkItem(m.st, m.linkRequest(e, title, backlog), m.now())
	if err != nil {
		m.flash = err.Error()
		return nil
	}
	where := "week"
	if backlog {
		where = "backlog"
	}
	m.flash = "added to " + where + ": " + strings.TrimSpace(title)
	cmd := m.apply(ops, "inbox "+e.plugin+": "+strings.TrimSpace(title))
	if t := m.st.Tasks[id]; t != nil {
		e.task = t
		cmd = tea.Batch(cmd, linked(e))
	}
	return cmd
}

// cursorGroup is the items in the group the cursor is in that a group
// verb can act on: owed work is left out.
func (m *Model) cursorGroup() []entry {
	start := m.cursor
	for start > 0 && m.rows[start].header == "" {
		start--
	}
	var out []entry
	for i := start + 1; i < len(m.rows) && m.rows[i].header == ""; i++ {
		if m.rows[i].inbox != nil && m.rows[i].inbox.dismissible() {
			out = append(out, *m.rows[i].inbox)
		}
	}
	return out
}

// hideItem hides owed work: a hidden task if it has none, else its task
// hidden, or hidden again from now.
func (m *Model) hideItem(e entry) tea.Cmd {
	if !e.item.Persistent {
		m.flash = "only open work is hidden; dismiss this one instead"
		return nil
	}
	if e.task != nil {
		m.flash = "hidden: " + e.task.Title
		return m.apply(core.HideTask(e.task, e.plugin, m.now()), "hide: "+e.task.Title)
	}
	ops, _, err := core.HideItem(m.st, m.linkRequest(e, e.item.Task.Title, true), m.now())
	if err != nil {
		m.flash = err.Error()
		return nil
	}
	m.flash = "hidden: " + e.item.Title
	return m.apply(ops, "hide: "+e.item.Title)
}

func (m *Model) actGroup(group []entry, verb string) tea.Cmd {
	var cmds []tea.Cmd
	for _, e := range group {
		cmds = append(cmds, act(e.plugin, e.item.ID, verb))
	}
	return tea.Batch(cmds...)
}

func openURL(url string) tea.Cmd {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	return func() tea.Msg {
		_ = exec.Command(name, url).Start()
		return nil
	}
}

// renderItem draws one inbox row.
func renderItem(e *entry, now time.Time) string {
	it := e.item
	var b strings.Builder
	switch {
	case e.state == core.ItemClosedOpen:
		b.WriteString(it.Ref + "  " + styleWarn.Render("task closed, still open") + styleFaint.Render(" · task: ") + e.task.Title)
	case e.state == core.ItemHiddenActivity:
		b.WriteString(it.Ref + "  " + styleFaint.Render("hidden · new activity · task: ") + e.task.Title)
	case e.task != nil:
		b.WriteString(it.Ref + "  " + styleFaint.Render("new activity · task: ") + e.task.Title)
	default:
		if it.Label != "" {
			b.WriteString(styleTag.Render("["+it.Label+"]") + " ")
		}
		if it.Ref != "" {
			b.WriteString(styleFaint.Render(it.Ref) + "  ")
		}
		b.WriteString(it.Title)
	}
	if !it.UpdatedAt.IsZero() {
		b.WriteString(styleFaint.Render("  " + age(now.Sub(it.UpdatedAt))))
	}
	return b.String()
}

func age(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", max(int(d.Minutes()), 0))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// updateHidden handles a key on the Hidden tab: r, u and o.
func (m *Model) updateHidden(msg tea.KeyMsg) (tea.Cmd, bool) {
	t := m.selectedTask()
	if t == nil {
		return nil, false
	}
	name, key := m.taskPlugin(t)
	if name == "" && (msg.String() == "r" || msg.String() == "u") {
		m.flash = "its plugin isn't in inbox = [...] in config.toml"
		return nil, true
	}
	switch msg.String() {
	case "r":
		m.flash = "now a task, in the backlog: " + t.Title
		return m.apply(core.TaskFromHidden(t, name, m.now()), "unhide as task: "+t.Title), true
	case "u":
		m.flash = "back in the inbox: " + t.Title
		return m.apply(core.Unhide(t, name, m.now()), "unhide: "+t.Title), true
	case "o":
		if u := m.itemURL(name, key, t); u != "" {
			return openURL(u), true
		}
		m.flash = "no link for this item"
		return nil, true
	}
	return nil, false
}

// taskPlugin is the inbox plugin a hidden task's item came from, and its key.
func (m *Model) taskPlugin(t *core.Task) (name, key string) {
	for _, p := range m.ib.plugins {
		if t.Ext(p, "hidden_at") != "" {
			return p, t.Ext(p, "item")
		}
	}
	return "", ""
}

// itemURL is the web page of a plugin item: from the latest list, or else
// the task's note if it is a web URL (dkt-gh puts the URL there).
func (m *Model) itemURL(name, key string, t *core.Task) string {
	for _, it := range m.ib.lists[name].Items {
		if it.Key == key && it.URL != "" {
			return it.URL
		}
	}
	return plugin.WebURL(strings.TrimSpace(t.Note))
}
