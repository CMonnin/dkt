package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/CMonnin/dkt/internal/app"
	"github.com/CMonnin/dkt/internal/core"
	"github.com/CMonnin/dkt/internal/editor"
	"github.com/CMonnin/dkt/internal/session"
)

func openTask(t *core.Task) bool   { return t.Status.Open() }
func closedTask(t *core.Task) bool { return !t.Status.Open() }

func cmdInit(args []string) error {
	if len(args) > 1 {
		return errors.New("usage: init [git-url]")
	}
	url := ""
	if len(args) == 1 {
		url = args[0]
	}
	s, err := session.Init(url)
	if err != nil {
		return err
	}
	if err := s.Pull(); err != nil {
		fmt.Fprintln(os.Stderr, "warning: pull failed:", err)
	}
	if err := s.Push(); err != nil {
		fmt.Fprintln(os.Stderr, "warning: push failed:", err)
	}
	fmt.Printf("config: %s\ndata:   %s\nlog:    %s\nstatus: %s\n",
		s.ConfigDir, s.DataDir, s.LogPath(), s.Status())
	return nil
}

func cmdAdd(c *ctx, args []string) error {
	fs := newFlags("add")
	var proj, cat, tags string
	var backlog bool
	stringFlag(fs, &proj, "p", "project", "project name")
	stringFlag(fs, &cat, "c", "category", "work|personal (inbox tasks only)")
	stringFlag(fs, &tags, "t", "tags", "comma-separated tags")
	boolFlag(fs, &backlog, "b", "backlog", "add to backlog instead of this week")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	title, inline := core.ParseTitle(strings.Join(pos, " "))
	in := core.NewTask{Title: title, Tags: append(inline, core.SplitTags(tags)...), Backlog: backlog}
	if proj != "" {
		p, err := c.resolveProject(proj, false)
		if err != nil {
			return err
		}
		in.Project = p.ID
	}
	if cat != "" {
		if in.Category, err = core.ParseCategory(cat); err != nil {
			return err
		}
	} else if proj == "" {
		in.Category = c.s.DefaultCategory()
	}
	op, err := core.CreateTask(c.st, in, c.now)
	if err != nil {
		return err
	}
	if err := c.commit([]core.Op{op}, "add: "+title); err != nil {
		return err
	}
	where := "week"
	if backlog {
		where = "backlog"
	}
	fmt.Fprintf(c.out, "added %s  %s  (%s)\n", core.ShortID(op.Target), title, where)
	return nil
}

func cmdLs(c *ctx, args []string) error {
	fs := newFlags("ls")
	var backlog, history bool
	var proj, tag string
	boolFlag(fs, &backlog, "backlog", "", "show backlog")
	boolFlag(fs, &history, "history", "", "show done and dropped")
	stringFlag(fs, &proj, "p", "project", "filter by project")
	fs.StringVar(&tag, "tag", "", "filter by tag")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	if backlog && history {
		return errors.New("--backlog and --history are exclusive")
	}
	projectID := ""
	if proj != "" {
		p, err := c.resolveProject(proj, history)
		if err != nil {
			return err
		}
		projectID = p.ID
	}
	tag = core.NormalizeTag(tag)
	keep := func(ts []*core.Task) []*core.Task {
		var out []*core.Task
		for _, t := range ts {
			if (projectID == "" || t.Project == projectID) && (tag == "" || t.HasTag(tag)) {
				out = append(out, t)
			}
		}
		return out
	}

	switch {
	case history:
		for _, t := range keep(c.st.HistoryTasks()) {
			when := t.CreatedAt
			if t.CompletedAt != nil {
				when = *t.CompletedAt
			}
			fmt.Fprintf(c.out, "%s %s\n", when.In(c.s.Loc).Format("2006-01-02"), c.line(t, true))
		}
	case backlog:
		tasks := keep(c.st.BacklogTasks())
		c.group("Inbox", tasks, func(t *core.Task) bool { return t.Project == "" }, false)
		for _, p := range c.st.SortedProjects(false) {
			c.group(p.Name, tasks, func(t *core.Task) bool { return t.Project == p.ID }, false)
		}
	default:
		tasks := keep(c.st.WeekTasks(c.now, c.s.Loc))
		for _, cat := range []core.Category{core.Work, core.Personal} {
			c.group(strings.ToUpper(string(cat[:1]))+string(cat[1:]), tasks,
				func(t *core.Task) bool { return c.st.EffectiveCategory(t) == cat }, true)
		}
		if n := len(c.st.Leftovers(c.now, c.s.Loc)); n > 0 && projectID == "" && tag == "" {
			fmt.Fprintf(c.out, "\n%d leftover(s) from last week — see `%s plan`\n", n, app.Name)
		}
	}
	return nil
}

func (c *ctx) group(title string, tasks []*core.Task, in func(*core.Task) bool, showProject bool) {
	first := true
	for _, t := range tasks {
		if !in(t) {
			continue
		}
		if first {
			fmt.Fprintf(c.out, "%s\n", title)
			first = false
		}
		fmt.Fprintln(c.out, c.line(t, showProject))
	}
}

func box(s core.Status) string {
	return map[core.Status]string{
		core.StatusTodo: "[ ]", core.StatusDoing: "[~]", core.StatusBlocked: "[!]",
		core.StatusDone: "[x]", core.StatusDropped: "[-]",
	}[s]
}

// styleBlocked renders plain when stdout is not a terminal or NO_COLOR is set.
var styleBlocked = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))

func (c *ctx) line(t *core.Task, showProject bool) string {
	var b strings.Builder
	head := box(t.Status) + " " + t.Title
	if t.Status == core.StatusBlocked {
		head = styleBlocked.Render(head)
	}
	fmt.Fprintf(&b, "  %s %s", core.ShortID(t.ID), head)
	if showProject && t.Project != "" {
		fmt.Fprintf(&b, "  (%s)", c.st.ProjectName(t.Project))
	}
	for _, tag := range t.Tags {
		b.WriteString(" #" + tag)
	}
	if t.Status == core.StatusBlocked && t.BlockedReason != "" {
		b.WriteString(styleBlocked.Render(" — " + t.BlockedReason))
	}
	if core.IsLeftover(t, c.now, c.s.Loc) {
		b.WriteString("  [leftover]")
	}
	return b.String()
}

func statusCmd(st core.Status) handler {
	return func(c *ctx, args []string) error {
		fs := newFlags(string(st))
		var reason string
		if st == core.StatusBlocked {
			stringFlag(fs, &reason, "r", "reason", "why it is blocked")
		}
		pos, err := parseFlags(fs, args)
		if err != nil {
			return err
		}
		filter, verb := openTask, string(st)
		if st == core.StatusTodo {
			filter, verb = closedTask, "reopen"
		}
		t, err := c.resolveTask(strings.Join(pos, " "), filter)
		if err != nil {
			return err
		}
		if err := c.commit(core.SetStatus(t, st, reason, c.now), verb+": "+t.Title); err != nil {
			return err
		}
		fmt.Fprintf(c.out, "%s %s  %s\n", box(st), core.ShortID(t.ID), t.Title)
		return nil
	}
}

func cmdEdit(c *ctx, args []string) error {
	fs := newFlags("edit")
	var title, note, proj, cat string
	fs.StringVar(&title, "title", "", "new title")
	stringFlag(fs, &note, "n", "note", "one-line note")
	stringFlag(fs, &proj, "p", "project", `project name ("" = inbox)`)
	stringFlag(fs, &cat, "c", "category", "work|personal")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	t, err := c.resolveTask(strings.Join(pos, " "), nil)
	if err != nil {
		return err
	}

	var ops []core.Op
	if !isSet(fs, "title", "n", "note", "p", "project", "c", "category") {
		text, err := editor.Edit(core.TaskForm(c.st, t))
		if err != nil {
			return err
		}
		if ops, err = core.ApplyTaskForm(c.st, t, text, c.now); err != nil {
			return err
		}
	} else {
		add := func(o []core.Op, err error) error {
			ops = append(ops, o...)
			return err
		}
		if isSet(fs, "title") {
			if err := add(core.SetTitle(t, title, c.now)); err != nil {
				return err
			}
		}
		if isSet(fs, "n", "note") {
			ops = append(ops, core.SetNote(t, note, c.now)...)
		}
		if isSet(fs, "p", "project") {
			id := ""
			if proj != "" {
				p, err := c.resolveProject(proj, false)
				if err != nil {
					return err
				}
				id = p.ID
			}
			if err := add(core.SetProject(c.st, t, id, c.now)); err != nil {
				return err
			}
			t.Project = id // so a -c in the same call is validated against the new project
		}
		if isSet(fs, "c", "category") {
			cc, err := core.ParseCategory(cat)
			if err != nil {
				return err
			}
			if err := add(core.SetCategory(t, cc, c.now)); err != nil {
				return err
			}
		}
	}
	if len(ops) == 0 {
		fmt.Fprintln(c.out, "no changes")
		return nil
	}
	if err := c.commit(ops, "edit: "+t.Title); err != nil {
		return err
	}
	fmt.Fprintf(c.out, "edited %s\n", core.ShortID(t.ID))
	return nil
}

func cmdTag(c *ctx, args []string) error {
	var add, remove, ref []string
	for _, a := range args {
		switch {
		case len(a) > 1 && (a[0] == '+' || a[0] == '#'):
			add = append(add, a[1:])
		case len(a) > 1 && a[0] == '-':
			remove = append(remove, a[1:])
		default:
			ref = append(ref, a)
		}
	}
	if len(add)+len(remove) == 0 {
		return errors.New("usage: tag <ref> +add -remove")
	}
	t, err := c.resolveTask(strings.Join(ref, " "), nil)
	if err != nil {
		return err
	}
	ops := core.EditTags(t, add, remove, c.now)
	if err := c.commit(ops, "tag: "+t.Title); err != nil {
		return err
	}
	st, _ := c.s.Load()
	fmt.Fprintf(c.out, "%s  %s  #%s\n", core.ShortID(t.ID), t.Title, strings.Join(st.Tasks[t.ID].Tags, " #"))
	return nil
}

func cmdWeek(c *ctx, args []string) error {
	t, err := c.resolveTask(strings.Join(args, " "), openTask)
	if err != nil {
		return err
	}
	if err := c.commit(core.MoveToWeek(t, c.now), "week: "+t.Title); err != nil {
		return err
	}
	fmt.Fprintf(c.out, "on week: %s  %s\n", core.ShortID(t.ID), t.Title)
	return nil
}

func cmdDefer(c *ctx, args []string) error {
	t, err := c.resolveTask(strings.Join(args, " "), openTask)
	if err != nil {
		return err
	}
	if err := c.commit(core.Defer(t, c.now), "defer: "+t.Title); err != nil {
		return err
	}
	fmt.Fprintf(c.out, "backlog: %s  %s\n", core.ShortID(t.ID), t.Title)
	return nil
}

func cmdPlan(c *ctx, args []string) error {
	left := c.st.Leftovers(c.now, c.s.Loc)
	if len(left) == 0 {
		fmt.Fprintln(c.out, "no leftovers from last week")
		return nil
	}
	fmt.Fprintf(c.out, "%d leftover(s) from last week:\n", len(left))
	for _, t := range left {
		fmt.Fprintln(c.out, c.line(t, true))
	}
	fmt.Fprintf(c.out, "\nkeep: %[1]s week <ref> · defer: %[1]s defer <ref> · drop: %[1]s drop <ref>\n", app.Name)
	return nil
}

func cmdExport(c *ctx, args []string) error {
	fs := newFlags("export")
	var since, until, tag, out string
	fs.StringVar(&since, "since", "", "window start (YYYY-MM-DD or YYYY-MM-DDTHH:MM)")
	fs.StringVar(&until, "until", "", "window end (a bare date means end of that day)")
	fs.StringVar(&tag, "tag", "", "only tasks with this tag")
	fs.StringVar(&out, "out", "", "write to file instead of stdout")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	opts := core.ExportOptions{Tag: tag, Loc: c.s.Loc}
	opts.Since, opts.Until = core.MeetingWindow(c.now, c.s.Loc, c.s.Meeting)
	var err error
	if since != "" {
		if opts.Since, err = parseWhen(since, c.s.Loc, false); err != nil {
			return err
		}
	}
	if until != "" {
		if opts.Until, err = parseWhen(until, c.s.Loc, true); err != nil {
			return err
		}
	}
	md := core.Export(c.st, opts)
	if out == "" {
		_, err := fmt.Fprint(c.out, md)
		return err
	}
	if err := os.WriteFile(out, []byte(md), 0o644); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "wrote", out)
	return nil
}

func parseWhen(s string, loc *time.Location, endOfDay bool) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02 15:04"} {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, nil
		}
	}
	t, err := time.ParseInLocation("2006-01-02", s, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("bad time %q (want YYYY-MM-DD[THH:MM])", s)
	}
	if endOfDay {
		t = t.AddDate(0, 0, 1)
	}
	return t, nil
}

func cmdSync(c *ctx, args []string) error {
	pullErr := c.s.Pull()
	pushErr := c.s.Push()
	fmt.Fprintln(c.out, c.s.Status())
	return errors.Join(pullErr, pushErr)
}

func cmdProject(c *ctx, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: project add|ls|rename|describe|archive")
	}
	sub, args := args[0], args[1:]
	switch sub {
	case "add":
		fs := newFlags("project add")
		var cat, desc string
		stringFlag(fs, &cat, "c", "category", "work|personal")
		stringFlag(fs, &desc, "d", "description", "description")
		pos, err := parseFlags(fs, args)
		if err != nil {
			return err
		}
		cc := c.s.DefaultCategory()
		if cat != "" {
			if cc, err = core.ParseCategory(cat); err != nil {
				return err
			}
		}
		op, err := core.CreateProject(c.st, strings.Join(pos, " "), cc, desc, c.now)
		if err != nil {
			return err
		}
		if err := c.commit([]core.Op{op}, "project add: "+op.Project.Name); err != nil {
			return err
		}
		fmt.Fprintf(c.out, "project %s  %s (%s)\n", core.ShortID(op.Target), op.Project.Name, cc)
	case "ls":
		fs := newFlags("project ls")
		var all bool
		boolFlag(fs, &all, "all", "a", "include archived")
		if _, err := parseFlags(fs, args); err != nil {
			return err
		}
		for _, p := range c.st.SortedProjects(all) {
			open := len(c.st.SortedTasks(func(t *core.Task) bool { return t.Project == p.ID && t.Status.Open() }))
			line := fmt.Sprintf("  %s  %s  (%s, %d open)", core.ShortID(p.ID), p.Name, p.Category, open)
			if p.Archived {
				line += " [archived]"
			}
			if p.Description != "" {
				line += " — " + p.Description
			}
			fmt.Fprintln(c.out, line)
		}
	case "rename":
		if len(args) != 2 {
			return errors.New(`usage: project rename <ref> "new name"`)
		}
		p, err := c.resolveProject(args[0], false)
		if err != nil {
			return err
		}
		ops, err := core.RenameProject(c.st, p, args[1], c.now)
		if err != nil {
			return err
		}
		return c.commit(ops, "project rename: "+p.Name+" → "+args[1])
	case "describe":
		if len(args) < 1 {
			return errors.New("usage: project describe <ref> <text>")
		}
		p, err := c.resolveProject(args[0], false)
		if err != nil {
			return err
		}
		return c.commit(core.DescribeProject(p, strings.Join(args[1:], " "), c.now), "project describe: "+p.Name)
	case "archive":
		p, err := c.resolveProject(strings.Join(args, " "), false)
		if err != nil {
			return err
		}
		ops := core.ArchiveProject(c.st, p, c.now)
		if err := c.commit(ops, "project archive: "+p.Name); err != nil {
			return err
		}
		fmt.Fprintf(c.out, "archived %s\n", p.Name)
	default:
		return fmt.Errorf("unknown project subcommand %q", sub)
	}
	return nil
}
