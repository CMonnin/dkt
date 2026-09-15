package core

import (
	"slices"
	"strings"
	"testing"
	"time"
)

// log accumulates ops and replays, standing in for the store.
type log struct {
	t   *testing.T
	ops []Op
}

func (l *log) add(ops ...Op) *State {
	l.ops = append(l.ops, ops...)
	return Replay(l.ops)
}

func (l *log) must(op Op, err error) (*State, string) {
	l.t.Helper()
	if err != nil {
		l.t.Fatal(err)
	}
	return l.add(op), op.Target
}

func TestArchivingProjectDropsOpenTasks(t *testing.T) {
	l := &log{t: t}
	now := at("2026-09-14T09:00:00Z")
	s, pid := l.must(CreateProject(Replay(nil), "ADNI QC", Work, "", now))
	s, open := l.must(CreateTask(s, NewTask{Title: "open", Project: pid}, now.Add(1*time.Minute)))
	s, done := l.must(CreateTask(s, NewTask{Title: "done", Project: pid}, now.Add(2*time.Minute)))
	s, other := l.must(CreateTask(s, NewTask{Title: "other"}, now.Add(3*time.Minute)))
	s = l.add(SetStatus(s.Tasks[done], StatusDone, "", now.Add(4*time.Minute))...)
	doneAt := *s.Tasks[done].CompletedAt

	s = l.add(ArchiveProject(s, s.Projects[pid], now.Add(time.Hour))...)

	if !s.Projects[pid].Archived {
		t.Error("project not archived")
	}
	if got := s.Tasks[open]; got.Status != StatusDropped || got.CompletedAt == nil {
		t.Errorf("open task = %s completed=%v, want dropped with completed_at", got.Status, got.CompletedAt)
	}
	if got := s.Tasks[done]; got.Status != StatusDone || !got.CompletedAt.Equal(doneAt) {
		t.Errorf("done task changed: %s %v", got.Status, got.CompletedAt)
	}
	if got := s.Tasks[other]; got.Status != StatusTodo {
		t.Errorf("unrelated task = %s, want todo", got.Status)
	}
}

func TestReopenClearsCompletedAt(t *testing.T) {
	l := &log{t: t}
	now := at("2026-09-14T09:00:00Z")
	s, id := l.must(CreateTask(Replay(nil), NewTask{Title: "x"}, now))
	s = l.add(ToggleDone(s.Tasks[id], now.Add(time.Minute))...)
	if s.Tasks[id].Status != StatusDone || s.Tasks[id].CompletedAt == nil {
		t.Fatalf("after done: %s %v", s.Tasks[id].Status, s.Tasks[id].CompletedAt)
	}
	s = l.add(Reopen(s.Tasks[id], now.Add(2*time.Minute))...)
	if s.Tasks[id].Status != StatusTodo || s.Tasks[id].CompletedAt != nil {
		t.Fatalf("after reopen: %s %v", s.Tasks[id].Status, s.Tasks[id].CompletedAt)
	}
}

func TestBlockSetsReasonAndUnblockClearsIt(t *testing.T) {
	l := &log{t: t}
	now := at("2026-09-14T09:00:00Z")
	s, id := l.must(CreateTask(Replay(nil), NewTask{Title: "x"}, now))
	s = l.add(SetStatus(s.Tasks[id], StatusBlocked, "waiting on IT\nmore detail", now.Add(time.Minute))...)
	if got := s.Tasks[id].BlockedReason; got != "waiting on IT" {
		t.Fatalf("reason = %q", got)
	}
	s = l.add(SetStatus(s.Tasks[id], StatusDoing, "", now.Add(2*time.Minute))...)
	if got := s.Tasks[id].BlockedReason; got != "" {
		t.Fatalf("reason after unblock = %q", got)
	}
}

func TestProjectTasksInheritCategory(t *testing.T) {
	l := &log{t: t}
	now := at("2026-09-14T09:00:00Z")
	s, pid := l.must(CreateProject(Replay(nil), "Garden", Personal, "", now))
	if _, err := CreateTask(s, NewTask{Title: "x", Project: pid, Category: Work}, now); err == nil {
		t.Error("work category on personal project accepted")
	}
	s, id := l.must(CreateTask(s, NewTask{Title: "x", Project: pid}, now.Add(time.Minute)))
	if got := s.EffectiveCategory(s.Tasks[id]); got != Personal {
		t.Errorf("category = %s, want personal", got)
	}
	if _, err := SetCategory(s.Tasks[id], Work, now); err == nil {
		t.Error("SetCategory on project task accepted")
	}
}

func TestCreateTaskDefaultsToWeek(t *testing.T) {
	title, tags := ParseTitle("rerun QC #adni")
	now := at("2026-09-14T09:00:00Z")
	op, err := CreateTask(Replay(nil), NewTask{Title: title, Tags: tags}, now)
	if err != nil {
		t.Fatal(err)
	}
	got := Replay([]Op{op}).Tasks[op.Target]
	if got.Title != "rerun QC" || !slices.Equal(got.Tags, []string{"adni"}) || !got.OnWeek ||
		got.Category != Work || got.WeekAddedAt == nil || !got.WeekAddedAt.Equal(now) {
		t.Fatalf("got %+v", got)
	}
}

func TestApplyTaskForm(t *testing.T) {
	l := &log{t: t}
	now := at("2026-09-14T09:00:00Z")
	s, pid := l.must(CreateProject(Replay(nil), "ADNI QC", Work, "", now))
	s, id := l.must(CreateTask(s, NewTask{Title: "old", Tags: []string{"a", "b"}}, now.Add(time.Minute)))

	form := TaskForm(s, s.Tasks[id])
	form = strings.Replace(form, "title: old", "title: new", 1)
	form = strings.Replace(form, "project: ", "project: adni qc", 1)
	form = strings.Replace(form, "tags: a, b", "tags: b, c", 1)
	form = strings.Replace(form, "status: todo", "status: blocked", 1)
	form = strings.Replace(form, "blocked_reason: ", "blocked_reason: IT", 1)
	form += "line one\nline two\n"

	ops, err := ApplyTaskForm(s, s.Tasks[id], form, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	got := l.add(ops...).Tasks[id]
	if got.Title != "new" || got.Project != pid || !slices.Equal(got.Tags, []string{"b", "c"}) ||
		got.Status != StatusBlocked || got.BlockedReason != "IT" || got.Note != "line one\nline two" {
		t.Fatalf("got %+v", got)
	}

	// Unchanged form yields no ops.
	s = Replay(l.ops)
	if ops, err := ApplyTaskForm(s, s.Tasks[id], TaskForm(s, s.Tasks[id]), now.Add(2*time.Hour)); err != nil || len(ops) != 0 {
		t.Fatalf("unchanged form: %d ops, err %v", len(ops), err)
	}
}
