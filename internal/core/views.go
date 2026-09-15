package core

import (
	"slices"
	"strings"
	"time"
)

// EffectiveCategory: project tasks inherit their project's category.
func (s *State) EffectiveCategory(t *Task) Category {
	if p, ok := s.Projects[t.Project]; ok {
		return p.Category
	}
	return t.Category
}

func (s *State) ProjectName(id string) string {
	if p, ok := s.Projects[id]; ok {
		return p.Name
	}
	return ""
}

// SortedTasks returns tasks matching pred (nil = all), oldest first.
func (s *State) SortedTasks(pred func(*Task) bool) []*Task {
	var out []*Task
	for _, t := range s.Tasks {
		if pred == nil || pred(t) {
			out = append(out, t)
		}
	}
	slices.SortFunc(out, func(a, b *Task) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

// SortedProjects returns projects by name.
func (s *State) SortedProjects(includeArchived bool) []*Project {
	var out []*Project
	for _, p := range s.Projects {
		if includeArchived || !p.Archived {
			out = append(out, p)
		}
	}
	slices.SortFunc(out, func(a, b *Project) int {
		if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

// AllTags returns every tag in use, sorted.
func (s *State) AllTags() []string {
	set := map[string]bool{}
	for _, t := range s.Tasks {
		for _, tag := range t.Tags {
			set[tag] = true
		}
	}
	out := make([]string, 0, len(set))
	for tag := range set {
		out = append(out, tag)
	}
	slices.Sort(out)
	return out
}

// WeekTasks: open tasks on the week, plus ones completed this week so
// ticking a box doesn't make the row vanish.
func (s *State) WeekTasks(now time.Time, loc *time.Location) []*Task {
	ws := WeekStart(now, loc)
	return s.SortedTasks(func(t *Task) bool {
		if !t.OnWeek {
			return false
		}
		return t.Status.Open() ||
			(t.Status == StatusDone && t.CompletedAt != nil && !t.CompletedAt.Before(ws))
	})
}

func (s *State) BacklogTasks() []*Task {
	return s.SortedTasks(func(t *Task) bool { return !t.OnWeek && t.Status.Open() })
}

// HistoryTasks: done and dropped, newest first.
func (s *State) HistoryTasks() []*Task {
	out := s.SortedTasks(func(t *Task) bool { return !t.Status.Open() })
	slices.SortStableFunc(out, func(a, b *Task) int {
		return completedAt(b).Compare(completedAt(a))
	})
	return out
}

func completedAt(t *Task) time.Time {
	if t.CompletedAt != nil {
		return *t.CompletedAt
	}
	return t.CreatedAt
}

func IsLeftover(t *Task, now time.Time, loc *time.Location) bool {
	return t.OnWeek && t.Status.Open() && t.WeekAddedAt != nil &&
		t.WeekAddedAt.Before(WeekStart(now, loc))
}

func (s *State) Leftovers(now time.Time, loc *time.Location) []*Task {
	return s.SortedTasks(func(t *Task) bool { return IsLeftover(t, now, loc) })
}
