package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"github.com/CMonnin/dkt/internal/core"
)

var (
	faintColor     = lipgloss.AdaptiveColor{Light: "245", Dark: "243"}
	styleTab       = lipgloss.NewStyle().Padding(0, 1).Foreground(faintColor)
	styleActiveTab = lipgloss.NewStyle().Padding(0, 1).Bold(true).Reverse(true)
	styleHeader    = lipgloss.NewStyle().Bold(true).Underline(true)
	styleCursor    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("4"))
	styleFaint     = lipgloss.NewStyle().Foreground(faintColor)
	styleDone      = lipgloss.NewStyle().Foreground(faintColor).Strikethrough(true)
	styleBlocked   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleDoing     = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	styleTag       = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	styleLeftover  = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	styleBanner    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("3")).Padding(0, 1)
	styleWarn      = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

var boxes = map[core.Status]string{
	core.StatusTodo: "[ ]", core.StatusDoing: "[~]", core.StatusBlocked: "[!]",
	core.StatusDone: "[x]", core.StatusDropped: "[-]",
}

func (m *Model) View() string {
	if m.width == 0 {
		return ""
	}
	top := []string{m.viewTabs()}
	var filters []string
	if m.projectFilter != "" {
		filters = append(filters, "project: "+m.st.ProjectName(m.projectFilter))
	}
	if m.search != "" && m.mode != modeSearch {
		filters = append(filters, "search: "+m.search)
	}
	if len(filters) > 0 {
		top = append(top, styleFaint.Render(strings.Join(filters, " · ")+"  (esc clears)"))
	}
	if m.tab == tabWeek {
		if n := len(m.leftovers()); n > 0 {
			top = append(top, styleBanner.Render(fmt.Sprintf(
				"%d leftover%s from last week — k keep · d defer · D drop · esc keep all", n, plural(n))))
		}
	}

	var bottom []string
	switch m.mode {
	case modeNormal:
	case modeConfirmArchive:
		if p := m.st.Projects[m.target]; p != nil {
			bottom = append(bottom, styleWarn.Render(fmt.Sprintf("archive %q and drop its open tasks? [y/N]", p.Name)))
		}
	default:
		bottom = append(bottom, m.input.View())
	}
	if m.flash != "" {
		bottom = append(bottom, styleWarn.Render(m.flash))
	}
	if m.help.ShowAll {
		bottom = append(bottom, m.help.FullHelpView(m.fullHelp()))
	} else {
		bottom = append(bottom, m.help.ShortHelpView(m.shortHelp()))
	}

	topStr, bottomStr := strings.Join(top, "\n"), strings.Join(bottom, "\n")
	bodyH := max(m.height-lipgloss.Height(topStr)-lipgloss.Height(bottomStr)-1, 1)
	m.scrollTo(bodyH)

	clip := lipgloss.NewStyle().MaxWidth(m.width)
	lines := make([]string, 0, bodyH)
	for i := m.offset; i < len(m.rows) && len(lines) < bodyH; i++ {
		lines = append(lines, clip.Render(m.renderRow(i)))
	}
	if len(m.rows) == 0 {
		lines = append(lines, styleFaint.Render("  nothing here"))
	}
	for len(lines) < bodyH {
		lines = append(lines, "")
	}
	return topStr + "\n\n" + strings.Join(lines, "\n") + "\n" + bottomStr
}

func (m *Model) scrollTo(h int) {
	if m.cursor < 0 {
		m.offset = 0
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
		if m.offset > 0 && !m.rows[m.offset-1].selectable() {
			m.offset-- // keep the group header in view
		}
	}
	if m.cursor >= m.offset+h {
		m.offset = m.cursor - h + 1
	}
	m.offset = max(m.offset, 0)
}

func (m *Model) viewTabs() string {
	var tabs []string
	for i, name := range tabNames {
		label := fmt.Sprintf("%d %s", i+1, name)
		if tab(i) == m.tab {
			tabs = append(tabs, styleActiveTab.Render(label))
		} else {
			tabs = append(tabs, styleTab.Render(label))
		}
	}
	left := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	status := m.status.String()
	if m.pushing || m.syncing {
		status += " · syncing…"
	}
	right := styleFaint.Render(status)
	if m.status.Unpushed > 0 {
		right = styleWarn.Render(status)
	}
	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	return left + strings.Repeat(" ", gap) + right
}

func (m *Model) renderRow(i int) string {
	r := m.rows[i]
	if r.header != "" {
		return styleHeader.Render(r.header)
	}
	cursor := "  "
	if i == m.cursor {
		cursor = styleCursor.Render("▸ ")
	}
	if r.project != nil {
		return cursor + m.renderProject(r.project)
	}
	return cursor + m.renderTask(r.task)
}

func (m *Model) renderProject(p *core.Project) string {
	open := len(m.st.SortedTasks(func(t *core.Task) bool { return t.Project == p.ID && t.Status.Open() }))
	line := p.Name + styleFaint.Render(fmt.Sprintf("  %s · %d open", p.Category, open))
	if p.Description != "" {
		line += styleFaint.Render("  — " + p.Description)
	}
	return line
}

func (m *Model) renderTask(t *core.Task) string {
	box, title := boxes[t.Status], t.Title
	switch {
	case t.Status == core.StatusBlocked:
		box = styleBlocked.Render(box)
	case t.Status == core.StatusDoing:
		box = styleDoing.Render(box)
	case !t.Status.Open():
		box = styleFaint.Render(box)
	}
	switch {
	case !t.Status.Open():
		title = styleDone.Render(title)
	case m.tab == tabWeek && m.isLeftover(t):
		title = styleLeftover.Render(title + "  ⟲ leftover")
	}

	var b strings.Builder
	if m.tab == tabHistory {
		when := t.CreatedAt
		if t.CompletedAt != nil {
			when = *t.CompletedAt
		}
		b.WriteString(styleFaint.Render(when.In(m.s.Loc).Format("2006-01-02")) + " ")
	}
	b.WriteString(box + " " + title)
	if m.tab != tabBacklog && t.Project != "" {
		b.WriteString(styleFaint.Render("  " + m.st.ProjectName(t.Project)))
	}
	for _, tag := range t.Tags {
		b.WriteString(styleTag.Render(" #" + tag))
	}
	if t.Status == core.StatusBlocked && t.BlockedReason != "" {
		b.WriteString(styleBlocked.Render(" — " + t.BlockedReason))
	}
	if t.Note != "" {
		b.WriteString(styleFaint.Render("  ✎ " + firstLine(t.Note)))
	}
	return b.String()
}

func (m *Model) shortHelp() []key.Binding {
	k := m.keys
	switch m.tab {
	case tabWeek:
		if t := m.selectedTask(); t != nil && m.isLeftover(t) {
			return []key.Binding{k.Keep, k.Defer, k.Drop, k.KeepAll, k.Help}
		}
		return []key.Binding{k.Add, k.Done, k.Doing, k.Block, k.Defer, k.Tags, k.Note, k.Editor, k.Search, k.Help, k.Quit}
	case tabBacklog:
		return []key.Binding{k.Add, k.Week, k.Done, k.Tags, k.Editor, k.Search, k.Esc, k.Help, k.Quit}
	case tabProjects:
		return []key.Binding{k.Add, k.Open, k.Rename, k.Describe, k.Category, k.Archive, k.Help, k.Quit}
	}
	return []key.Binding{k.Reopen, k.Search, k.Editor, k.Help, k.Quit}
}

func (m *Model) fullHelp() [][]key.Binding {
	k := m.keys
	return [][]key.Binding{
		{k.Up, k.Down, k.Top, k.Bottom, k.NextTab, k.PrevTab, k.Tabs},
		{k.Add, k.Done, k.Doing, k.Block, k.Drop, k.Week, k.Defer},
		{k.Tags, k.Note, k.Editor, k.Search, k.Category, k.Esc},
		{k.Open, k.Rename, k.Describe, k.Archive},
		{k.Keep, k.KeepAll, k.Sync, k.Help, k.Quit},
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
