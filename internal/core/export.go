package core

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

type ExportOptions struct {
	Since, Until time.Time
	Tag          string // "" = no filter
	Loc          *time.Location
}

type exportGroup struct {
	name                       string
	done, doing, blocked, next []*Task
}

// Export renders the work-category meeting report as Markdown.
func Export(s *State, o ExportOptions) string {
	loc := o.Loc
	if loc == nil {
		loc = time.Local
	}
	tag := NormalizeTag(o.Tag)
	groups := map[string]*exportGroup{}
	get := func(projectID string) *exportGroup {
		g, ok := groups[projectID]
		if !ok {
			g = &exportGroup{name: "Inbox"}
			if projectID != "" {
				g.name = s.ProjectName(projectID)
			}
			groups[projectID] = g
		}
		return g
	}

	for _, t := range s.SortedTasks(nil) {
		if s.EffectiveCategory(t) != Work || (tag != "" && !t.HasTag(tag)) {
			continue
		}
		switch t.Status {
		case StatusDone:
			if c := t.CompletedAt; c != nil && c.After(o.Since) && !c.After(o.Until) {
				get(t.Project).done = append(get(t.Project).done, t)
			}
		case StatusDoing:
			get(t.Project).doing = append(get(t.Project).doing, t)
		case StatusBlocked:
			get(t.Project).blocked = append(get(t.Project).blocked, t)
		case StatusTodo:
			if t.OnWeek {
				get(t.Project).next = append(get(t.Project).next, t)
			}
		}
	}

	ids := make([]string, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b string) int {
		if (a == "") != (b == "") { // Inbox first
			if a == "" {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(groups[a].name), strings.ToLower(groups[b].name))
	})

	var b strings.Builder
	const layout = "2006-01-02 15:04"
	fmt.Fprintf(&b, "# Work update: %s → %s\n", o.Since.In(loc).Format(layout), o.Until.In(loc).Format(layout))
	if tag != "" {
		fmt.Fprintf(&b, "\n_Tag: #%s_\n", tag)
	}
	if len(ids) == 0 {
		b.WriteString("\n_Nothing to report._\n")
		return b.String()
	}
	for _, id := range ids {
		g := groups[id]
		slices.SortStableFunc(g.done, func(x, y *Task) int { return x.CompletedAt.Compare(*y.CompletedAt) })
		fmt.Fprintf(&b, "\n## %s\n", g.name)
		section(&b, "Done", g.done, nil)
		section(&b, "In progress", g.doing, nil)
		section(&b, "Blocked", g.blocked, func(t *Task) string { return firstLine(t.BlockedReason) })
		section(&b, "Next", g.next, nil)
	}
	return b.String()
}

func section(b *strings.Builder, title string, tasks []*Task, extra func(*Task) string) {
	if len(tasks) == 0 {
		return
	}
	fmt.Fprintf(b, "\n**%s**\n", title)
	for _, t := range tasks {
		line := "- " + t.Title
		if extra != nil {
			if e := extra(t); e != "" {
				line += " — " + e
			}
		}
		b.WriteString(line + "\n")
	}
}
