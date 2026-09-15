package core

import (
	"fmt"
	"strings"
)

// MatchTasks resolves a user reference. A ULID prefix (>= 4 Crockford
// chars) is tried first across all tasks; otherwise titles among tasks
// passing filter are matched: exact (case-insensitive) beats "contains all
// words". One result means resolved; more means ambiguous.
func MatchTasks(s *State, ref string, filter func(*Task) bool) []*Task {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	if up := strings.ToUpper(ref); isULIDPrefix(up) {
		if m := s.SortedTasks(func(t *Task) bool { return strings.HasPrefix(t.ID, up) }); len(m) > 0 {
			return m
		}
	}
	cands := s.SortedTasks(filter)
	return matchByName(cands, ref, func(t *Task) string { return t.Title })
}

// MatchProjects is MatchTasks for projects, by name.
func MatchProjects(s *State, ref string, includeArchived bool) []*Project {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	all := s.SortedProjects(true)
	if up := strings.ToUpper(ref); isULIDPrefix(up) {
		var m []*Project
		for _, p := range all {
			if strings.HasPrefix(p.ID, up) {
				m = append(m, p)
			}
		}
		if len(m) > 0 {
			return m
		}
	}
	return matchByName(s.SortedProjects(includeArchived), ref, func(p *Project) string { return p.Name })
}

func matchByName[T any](cands []T, ref string, name func(T) string) []T {
	low := strings.ToLower(ref)
	var exact, fuzzy []T
	words := strings.Fields(low)
	for _, c := range cands {
		n := strings.ToLower(name(c))
		if n == low {
			exact = append(exact, c)
			continue
		}
		all := true
		for _, w := range words {
			if !strings.Contains(n, w) {
				all = false
				break
			}
		}
		if all {
			fuzzy = append(fuzzy, c)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	return fuzzy
}

type AmbiguousError struct {
	Ref        string
	Candidates []string
}

func (e *AmbiguousError) Error() string {
	return fmt.Sprintf("%q is ambiguous:\n  %s", e.Ref, strings.Join(e.Candidates, "\n  "))
}

type NotFoundError struct{ Ref string }

func (e *NotFoundError) Error() string { return fmt.Sprintf("nothing matches %q", e.Ref) }

// ResolveTask returns exactly one task or a NotFound/Ambiguous error.
func ResolveTask(s *State, ref string, filter func(*Task) bool) (*Task, error) {
	m := MatchTasks(s, ref, filter)
	switch len(m) {
	case 0:
		return nil, &NotFoundError{ref}
	case 1:
		return m[0], nil
	}
	e := &AmbiguousError{Ref: ref}
	for _, t := range m {
		e.Candidates = append(e.Candidates, ShortID(t.ID)+"  "+t.Title)
	}
	return nil, e
}

// ShortID is the prefix shown in listings. ULIDs share leading time
// chars, so the random tail matters; 10 chars is usually unique.
func ShortID(id string) string {
	if len(id) > 10 {
		return id[:10]
	}
	return id
}

func isULIDPrefix(s string) bool {
	if len(s) < 4 || len(s) > 26 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", r) {
			return false
		}
	}
	return true
}
