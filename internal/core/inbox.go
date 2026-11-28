package core

import (
	"errors"
	"strings"
	"time"
)

// ItemState says what the inbox does with a plugin item.
type ItemState int

const (
	ItemNew            ItemState = iota // not linked to a task: show it in its plugin group
	ItemUpdated                         // linked, with activity since the link: show it in ↻ Updates
	ItemLinked                          // linked, nothing new: hide it, and tell the plugin again
	ItemClosedOpen                      // persistent, task closed but not hidden: still open at the source
	ItemHiddenActivity                  // task hidden, with activity since the hide
)

// ClassifyItem finds the task linked to a plugin item by its key, and says
// whether the item is new, updated since the link (or since the hide, for
// a hidden task), still open at the source although its task is closed
// (persistent items only), or already handled. If
// several tasks are linked to the item (two machines linked it before
// syncing), the most recently linked one counts.
func (s *State) ClassifyItem(plugin, key string, updatedAt time.Time, persistent bool) (ItemState, *Task) {
	var best *Task
	var bestAt time.Time
	for _, t := range s.Tasks {
		if t.Ext(plugin, "item") != key {
			continue
		}
		at, _ := time.Parse(time.RFC3339, t.Ext(plugin, "linked_at"))
		if best == nil || at.After(bestAt) || at.Equal(bestAt) && t.ID > best.ID {
			best, bestAt = t, at
		}
	}
	switch {
	case best == nil:
		return ItemNew, nil
	case best.Hidden():
		hiddenAt, _ := time.Parse(time.RFC3339, best.Ext(plugin, "hidden_at"))
		if updatedAt.After(hiddenAt) {
			return ItemHiddenActivity, best
		}
	case updatedAt.After(bestAt):
		return ItemUpdated, best
	case persistent && !best.Status.Open():
		return ItemClosedOpen, best
	}
	return ItemLinked, best
}

// LinkRequest is a plugin item to turn into a linked task, with the task
// fields the plugin suggested.
type LinkRequest struct {
	Plugin, Key string // the item's plugin and stable key

	Title, Note string
	Category    Category
	Tags        []string
	Backlog     bool // false: on this week

	ProjectKey, ProjectName string // suggested project; empty for none
}

// LinkItem returns the ops that make a task from a plugin item and link it,
// and the new task's ID. Commit them together.
func LinkItem(s *State, r LinkRequest, now time.Time) ([]Op, string, error) {
	if r.Key == "" {
		return nil, "", errors.New("item key is empty")
	}
	if err := checkExt(r.Plugin, "item", r.Key); err != nil {
		return nil, "", err
	}
	in := NewTask{Title: r.Title, Category: r.Category, Tags: r.Tags, Backlog: r.Backlog}
	var ops []Op
	if r.ProjectKey != "" {
		p := s.linkedProject(r.Plugin, r.ProjectKey)
		if p == nil {
			if p = s.projectNamed(r.ProjectName); p != nil {
				link, err := SetProjectExt(p, r.Plugin, "project."+r.ProjectKey, "1", now)
				if err != nil {
					return nil, "", err
				}
				ops = append(ops, link...)
			}
		}
		if p != nil {
			in.Project, in.Category = p.ID, "" // project tasks take the project's category
		}
	}
	newProject := ""
	if r.ProjectKey != "" && in.Project == "" {
		op, err := CreateProject(s, r.ProjectName, r.Category, "", now)
		if err != nil {
			return nil, "", err
		}
		link, err := SetProjectExt(&Project{ID: op.Target}, r.Plugin, "project."+r.ProjectKey, "1", now)
		if err != nil {
			return nil, "", err
		}
		ops = append(append(ops, op), link...)
		newProject = op.Target
	}
	create, err := CreateTask(s, in, now)
	if err != nil {
		return nil, "", err
	}
	if newProject != "" { // CreateTask can't see a project made in this same batch
		create.Task.Project = newProject
	}
	create.Task.Note = strings.TrimSpace(r.Note)
	id := create.Target
	return append(ops,
		create,
		setTask(now, id, "x."+r.Plugin+".item", r.Key),
		setTask(now, id, "x."+r.Plugin+".linked_at", now.UTC().Format(time.RFC3339)),
	), id, nil
}

// linkedProject is the non-archived project linked to a plugin's project key.
func (s *State) linkedProject(plugin, key string) *Project {
	for _, p := range s.SortedProjects(false) {
		if p.Ext(plugin, "project."+key) != "" {
			return p
		}
	}
	return nil
}

// projectNamed is the non-archived project with this name, ignoring case.
func (s *State) projectNamed(name string) *Project {
	for _, p := range s.SortedProjects(false) {
		if name != "" && strings.EqualFold(p.Name, strings.TrimSpace(name)) {
			return p
		}
	}
	return nil
}

// HideItem returns the ops that hide a plugin item with no task yet: a
// linked task in the backlog, dropped at once, with x.<plugin>.hidden_at.
func HideItem(s *State, r LinkRequest, now time.Time) ([]Op, string, error) {
	r.Backlog = true
	ops, id, err := LinkItem(s, r, now)
	if err != nil {
		return nil, "", err
	}
	return append(ops,
		setTask(now, id, "status", StatusDropped),
		setTask(now, id, "completed_at", now.UTC()),
		setTask(now, id, "x."+r.Plugin+".hidden_at", now.UTC().Format(time.RFC3339)),
	), id, nil
}

// HideTask hides the plugin item behind an existing task: it drops the
// task if it's open, and sets x.<plugin>.hidden_at to now. Hiding again
// moves hidden_at, so only activity after it shows.
func HideTask(t *Task, plugin string, now time.Time) []Op {
	var ops []Op
	if t.Status.Open() {
		ops = SetStatus(t, StatusDropped, "", now)
	}
	return append(ops, setTask(now, t.ID, "x."+plugin+".hidden_at", now.UTC().Format(time.RFC3339)))
}

// Unhide puts a hidden item back in the inbox, as new: it clears the link
// and hidden_at. The task stays dropped, and so moves to History.
func Unhide(t *Task, plugin string, now time.Time) []Op {
	return []Op{
		setTask(now, t.ID, "x."+plugin+".item", ""),
		setTask(now, t.ID, "x."+plugin+".hidden_at", ""),
	}
}

// TaskFromHidden turns a hidden item's task into a real one: todo, in the
// backlog, no longer hidden, still linked.
func TaskFromHidden(t *Task, plugin string, now time.Time) []Op {
	ops := SetStatus(t, StatusTodo, "", now)
	if t.OnWeek {
		ops = append(ops, Defer(t, now)...)
	}
	return append(ops, setTask(now, t.ID, "x."+plugin+".hidden_at", ""))
}
