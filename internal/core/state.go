package core

import (
	"encoding/json"
	"slices"
	"strings"
	"time"
)

type State struct {
	Tasks    map[string]*Task
	Projects map[string]*Project
}

// Replay sorts ops by (ts, id) and applies them in order. Applying in a
// total order makes every scalar field last-write-wins, and tag add/remove
// behave as a set, independent of how the logs were read.
func Replay(ops []Op) *State {
	sorted := slices.Clone(ops)
	slices.SortStableFunc(sorted, func(a, b Op) int {
		switch {
		case a.Less(b):
			return -1
		case b.Less(a):
			return 1
		}
		return 0
	})

	s := &State{Tasks: map[string]*Task{}, Projects: map[string]*Project{}}
	createdTasks := map[string]bool{}
	createdProjects := map[string]bool{}
	seen := map[string]bool{}
	for _, op := range sorted {
		if seen[op.ID] {
			continue
		}
		seen[op.ID] = true
		switch op.Type {
		case OpTaskCreate:
			if op.Task != nil {
				s.applyTaskCreate(op)
				createdTasks[op.Target] = true
			}
		case OpTaskSet:
			s.applyTaskSet(s.task(op.Target), op)
		case OpTagAdd:
			s.task(op.Target).addTag(NormalizeTag(op.Tag))
		case OpTagRemove:
			s.task(op.Target).removeTag(NormalizeTag(op.Tag))
		case OpProjectCreate:
			if op.Project != nil {
				p := s.project(op.Target)
				p.Name, p.Category, p.Description = op.Project.Name, op.Project.Category, op.Project.Description
				p.CreatedAt = op.TS
				createdProjects[op.Target] = true
			}
		case OpProjectSet:
			s.applyProjectSet(s.project(op.Target), op)
		}
	}
	// Ops whose create never arrived (e.g. a log not yet pulled) are ignored.
	for id := range s.Tasks {
		if !createdTasks[id] {
			delete(s.Tasks, id)
		}
	}
	for id := range s.Projects {
		if !createdProjects[id] {
			delete(s.Projects, id)
		}
	}
	return s
}

func (s *State) task(id string) *Task {
	t, ok := s.Tasks[id]
	if !ok {
		t = &Task{ID: id, Status: StatusTodo}
		s.Tasks[id] = t
	}
	return t
}

func (s *State) project(id string) *Project {
	p, ok := s.Projects[id]
	if !ok {
		p = &Project{ID: id}
		s.Projects[id] = p
	}
	return p
}

func (s *State) applyTaskCreate(op Op) {
	t, c := s.task(op.Target), op.Task
	t.Title, t.Category, t.Project, t.Note = c.Title, c.Category, c.Project, c.Note
	t.Status = c.Status
	if t.Status == "" {
		t.Status = StatusTodo
	}
	t.OnWeek, t.WeekAddedAt = c.OnWeek, utcPtr(c.WeekAddedAt)
	t.CreatedAt = op.TS
	for _, tag := range c.Tags {
		t.addTag(NormalizeTag(tag))
	}
}

func (s *State) applyTaskSet(t *Task, op Op) {
	switch op.Field {
	case "title":
		t.Title = decStr(op.Value)
	case "category":
		t.Category = Category(decStr(op.Value))
	case "project":
		t.Project = decStr(op.Value)
	case "status":
		t.Status = Status(decStr(op.Value))
	case "note":
		t.Note = decStr(op.Value)
	case "blocked_reason":
		t.BlockedReason = decStr(op.Value)
	case "on_week":
		t.OnWeek = decBool(op.Value)
	case "week_added_at":
		t.WeekAddedAt = decTime(op.Value)
	case "completed_at":
		t.CompletedAt = decTime(op.Value)
	}
}

func (s *State) applyProjectSet(p *Project, op Op) {
	switch op.Field {
	case "name":
		p.Name = decStr(op.Value)
	case "category":
		p.Category = Category(decStr(op.Value))
	case "description":
		p.Description = decStr(op.Value)
	case "archived":
		p.Archived = decBool(op.Value)
	}
}

func decStr(v json.RawMessage) string {
	var s string
	_ = json.Unmarshal(v, &s)
	return s
}

func decBool(v json.RawMessage) bool {
	var b bool
	_ = json.Unmarshal(v, &b)
	return b
}

func decTime(v json.RawMessage) *time.Time {
	if len(v) == 0 || strings.TrimSpace(string(v)) == "null" {
		return nil
	}
	var t time.Time
	if err := json.Unmarshal(v, &t); err != nil {
		return nil
	}
	return utcPtr(&t)
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}
