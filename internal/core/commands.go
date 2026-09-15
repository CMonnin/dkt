package core

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Builders turn user intents into ops. They validate against the current
// state but never mutate it; callers persist the ops and replay.

type NewTask struct {
	Title    string
	Category Category
	Project  string // project ID
	Tags     []string
	Backlog  bool
}

func CreateTask(s *State, in NewTask, now time.Time) (Op, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return Op{}, errors.New("title is empty")
	}
	cat := in.Category
	if in.Project != "" {
		p, ok := s.Projects[in.Project]
		if !ok || p.Archived {
			return Op{}, errors.New("project not found or archived")
		}
		if cat != "" && cat != p.Category {
			return Op{}, fmt.Errorf("project %q is %s; project tasks inherit its category", p.Name, p.Category)
		}
		cat = p.Category
	}
	if cat == "" {
		cat = Work
	}
	op := newOp(now, OpTaskCreate, NewID())
	c := &TaskCreate{Title: title, Category: cat, Project: in.Project, Status: StatusTodo, OnWeek: !in.Backlog}
	for _, tag := range in.Tags {
		if tag = NormalizeTag(tag); tag != "" {
			c.Tags = append(c.Tags, tag)
		}
	}
	if c.OnWeek {
		u := now.UTC()
		c.WeekAddedAt = &u
	}
	op.Task = c
	return op, nil
}

// SetStatus keeps completed_at and blocked_reason consistent with status.
func SetStatus(t *Task, st Status, reason string, now time.Time) []Op {
	ops := []Op{setTask(now, t.ID, "status", st)}
	if st.Open() {
		if t.CompletedAt != nil {
			ops = append(ops, setTask(now, t.ID, "completed_at", nil))
		}
	} else {
		ops = append(ops, setTask(now, t.ID, "completed_at", now.UTC()))
	}
	if st == StatusBlocked {
		ops = append(ops, setTask(now, t.ID, "blocked_reason", firstLine(reason)))
	} else if t.BlockedReason != "" {
		ops = append(ops, setTask(now, t.ID, "blocked_reason", ""))
	}
	return ops
}

func Reopen(t *Task, now time.Time) []Op { return SetStatus(t, StatusTodo, "", now) }

func ToggleDone(t *Task, now time.Time) []Op {
	if t.Status == StatusDone {
		return Reopen(t, now)
	}
	return SetStatus(t, StatusDone, "", now)
}

// MoveToWeek also serves as "keep" for a leftover: it restamps week_added_at.
func MoveToWeek(t *Task, now time.Time) []Op {
	return []Op{
		setTask(now, t.ID, "on_week", true),
		setTask(now, t.ID, "week_added_at", now.UTC()),
	}
}

func Defer(t *Task, now time.Time) []Op {
	return []Op{setTask(now, t.ID, "on_week", false)}
}

func SetTitle(t *Task, title string, now time.Time) ([]Op, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, errors.New("title is empty")
	}
	return []Op{setTask(now, t.ID, "title", title)}, nil
}

func SetNote(t *Task, note string, now time.Time) []Op {
	return []Op{setTask(now, t.ID, "note", strings.TrimSpace(note))}
}

func SetBlockedReason(t *Task, reason string, now time.Time) []Op {
	return []Op{setTask(now, t.ID, "blocked_reason", firstLine(reason))}
}

func SetProject(s *State, t *Task, projectID string, now time.Time) ([]Op, error) {
	if projectID != "" {
		if p, ok := s.Projects[projectID]; !ok || p.Archived {
			return nil, errors.New("project not found or archived")
		}
	}
	return []Op{setTask(now, t.ID, "project", projectID)}, nil
}

func SetCategory(t *Task, c Category, now time.Time) ([]Op, error) {
	if t.Project != "" {
		return nil, errors.New("project tasks inherit the project's category")
	}
	return []Op{setTask(now, t.ID, "category", c)}, nil
}

// EditTags emits only the changes that actually alter the tag set.
func EditTags(t *Task, add, remove []string, now time.Time) []Op {
	var ops []Op
	for _, tag := range remove {
		if tag = NormalizeTag(tag); tag != "" && t.HasTag(tag) {
			ops = append(ops, tagOp(now, OpTagRemove, t.ID, tag))
		}
	}
	for _, tag := range add {
		if tag = NormalizeTag(tag); tag != "" && !t.HasTag(tag) {
			ops = append(ops, tagOp(now, OpTagAdd, t.ID, tag))
		}
	}
	return ops
}

func CreateProject(s *State, name string, c Category, desc string, now time.Time) (Op, error) {
	name = strings.TrimSpace(name)
	if err := checkProjectName(s, name, ""); err != nil {
		return Op{}, err
	}
	if c == "" {
		c = Work
	}
	op := newOp(now, OpProjectCreate, NewID())
	op.Project = &ProjectCreate{Name: name, Category: c, Description: strings.TrimSpace(desc)}
	return op, nil
}

func RenameProject(s *State, p *Project, name string, now time.Time) ([]Op, error) {
	name = strings.TrimSpace(name)
	if err := checkProjectName(s, name, p.ID); err != nil {
		return nil, err
	}
	return []Op{setProject(now, p.ID, "name", name)}, nil
}

func DescribeProject(p *Project, desc string, now time.Time) []Op {
	return []Op{setProject(now, p.ID, "description", strings.TrimSpace(desc))}
}

func SetProjectCategory(p *Project, c Category, now time.Time) []Op {
	return []Op{setProject(now, p.ID, "category", c)}
}

// ArchiveProject archives the project and drops its open tasks.
func ArchiveProject(s *State, p *Project, now time.Time) []Op {
	ops := []Op{setProject(now, p.ID, "archived", true)}
	for _, t := range s.SortedTasks(func(t *Task) bool { return t.Project == p.ID && t.Status.Open() }) {
		ops = append(ops, SetStatus(t, StatusDropped, "", now)...)
	}
	return ops
}

func checkProjectName(s *State, name, selfID string) error {
	if name == "" {
		return errors.New("project name is empty")
	}
	for _, p := range s.Projects {
		if p.ID != selfID && !p.Archived && strings.EqualFold(p.Name, name) {
			return fmt.Errorf("project %q already exists", p.Name)
		}
	}
	return nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}
