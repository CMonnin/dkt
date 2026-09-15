// Package core is the pure domain: ops, replay into state, and the
// derived views (week, leftovers, meeting window, export, refs).
// Nothing here touches the filesystem, git, or the wall clock.
package core

import (
	"fmt"
	"slices"
	"strings"
	"time"
)

type Status string

const (
	StatusTodo    Status = "todo"
	StatusDoing   Status = "doing"
	StatusBlocked Status = "blocked"
	StatusDone    Status = "done"
	StatusDropped Status = "dropped"
)

// Open reports whether the task still needs doing.
func (s Status) Open() bool {
	return s == StatusTodo || s == StatusDoing || s == StatusBlocked
}

func ParseStatus(s string) (Status, error) {
	st := Status(strings.ToLower(strings.TrimSpace(s)))
	switch st {
	case StatusTodo, StatusDoing, StatusBlocked, StatusDone, StatusDropped:
		return st, nil
	}
	return "", fmt.Errorf("unknown status %q (todo|doing|blocked|done|dropped)", s)
}

type Category string

const (
	Work     Category = "work"
	Personal Category = "personal"
)

func ParseCategory(s string) (Category, error) {
	c := Category(strings.ToLower(strings.TrimSpace(s)))
	if c == Work || c == Personal {
		return c, nil
	}
	return "", fmt.Errorf("unknown category %q (work|personal)", s)
}

type Task struct {
	ID            string
	Title         string
	Category      Category // only meaningful for inbox tasks; see State.EffectiveCategory
	Project       string   // project ID, "" = inbox
	Status        Status
	Note          string
	BlockedReason string
	Tags          []string // sorted, unique
	OnWeek        bool
	WeekAddedAt   *time.Time
	CreatedAt     time.Time
	CompletedAt   *time.Time // set when done or dropped
}

func (t *Task) HasTag(tag string) bool {
	_, ok := slices.BinarySearch(t.Tags, tag)
	return ok
}

func (t *Task) addTag(tag string) {
	if i, ok := slices.BinarySearch(t.Tags, tag); !ok {
		t.Tags = slices.Insert(t.Tags, i, tag)
	}
}

func (t *Task) removeTag(tag string) {
	if i, ok := slices.BinarySearch(t.Tags, tag); ok {
		t.Tags = slices.Delete(t.Tags, i, i+1)
	}
}

type Project struct {
	ID          string
	Name        string
	Category    Category
	Description string
	Archived    bool
	CreatedAt   time.Time
}
