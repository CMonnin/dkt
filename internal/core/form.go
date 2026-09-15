package core

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

const formSeparator = "--- note below this line ---"

// TaskForm renders a task as an editable text form for $EDITOR.
func TaskForm(s *State, t *Task) string {
	return fmt.Sprintf(`title: %s
project: %s
category: %s
status: %s
tags: %s
blocked_reason: %s
# project: empty = inbox. category is ignored for project tasks.
# status: todo|doing|blocked|done|dropped. Lines starting with # are ignored.
%s
%s
`, t.Title, s.ProjectName(t.Project), t.Category, t.Status,
		strings.Join(t.Tags, ", "), t.BlockedReason, formSeparator, t.Note)
}

// ApplyTaskForm diffs an edited form against the task and returns the ops.
// Keys removed from the form leave their field unchanged.
func ApplyTaskForm(s *State, t *Task, text string, now time.Time) ([]Op, error) {
	fields := map[string]string{}
	var note []string
	inNote := false
	for _, line := range strings.Split(text, "\n") {
		switch {
		case inNote:
			note = append(note, line)
		case strings.HasPrefix(line, "---"):
			inNote = true
		case strings.HasPrefix(strings.TrimSpace(line), "#"), strings.TrimSpace(line) == "":
		default:
			k, v, ok := strings.Cut(line, ":")
			if !ok {
				return nil, fmt.Errorf("bad line %q (want key: value)", line)
			}
			fields[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}

	var ops []Op
	if v, ok := fields["title"]; ok && v != t.Title {
		o, err := SetTitle(t, v, now)
		if err != nil {
			return nil, err
		}
		ops = append(ops, o...)
	}
	project := t.Project
	if v, ok := fields["project"]; ok && !strings.EqualFold(v, s.ProjectName(t.Project)) {
		project = ""
		if v != "" {
			m := MatchProjects(s, v, false)
			if len(m) != 1 {
				return nil, fmt.Errorf("project %q: %d matches", v, len(m))
			}
			project = m[0].ID
		}
		o, err := SetProject(s, t, project, now)
		if err != nil {
			return nil, err
		}
		ops = append(ops, o...)
	}
	if v, ok := fields["category"]; ok && project == "" && v != string(t.Category) {
		c, err := ParseCategory(v)
		if err != nil {
			return nil, err
		}
		ops = append(ops, setTask(now, t.ID, "category", c))
	}
	status := t.Status
	reason, reasonGiven := fields["blocked_reason"]
	if v, ok := fields["status"]; ok && v != string(t.Status) {
		st, err := ParseStatus(v)
		if err != nil {
			return nil, err
		}
		status = st
		ops = append(ops, SetStatus(t, st, reason, now)...)
	} else if reasonGiven && status == StatusBlocked && reason != t.BlockedReason {
		ops = append(ops, SetBlockedReason(t, reason, now)...)
	}
	if v, ok := fields["tags"]; ok {
		want := SplitTags(v)
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
		ops = append(ops, EditTags(t, add, remove, now)...)
	}
	if inNote {
		if n := strings.TrimSpace(strings.Join(note, "\n")); n != t.Note {
			ops = append(ops, SetNote(t, n, now)...)
		}
	}
	return ops, nil
}
