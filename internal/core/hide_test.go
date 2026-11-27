package core

import (
	"testing"
	"time"
)

func ids(ts []*Task) []string {
	var out []string
	for _, t := range ts {
		out = append(out, t.ID)
	}
	return out
}

func TestHiddenTasksLeaveHistory(t *testing.T) {
	s := Replay([]Op{
		mkCreate("01A1", "T1", "2026-11-27T09:00:00Z", "dropped"),
		mkSet("01A2", "T1", "2026-11-27T09:01:00Z", "status", "dropped"),
		mkCreate("01B1", "T2", "2026-11-27T09:00:00Z", "hidden"),
		mkSet("01B2", "T2", "2026-11-27T09:01:00Z", "status", "dropped"),
		mkSet("01B3", "T2", "2026-11-27T09:01:00Z", "x.gh.hidden_at", "2026-11-27T09:01:00Z"),
		mkCreate("01C1", "T3", "2026-11-27T09:00:00Z", "hidden later"),
		mkSet("01C2", "T3", "2026-11-27T09:02:00Z", "status", "dropped"),
		mkSet("01C3", "T3", "2026-11-27T09:02:00Z", "x.wl.hidden_at", "2026-11-27T09:02:00Z"),
	})
	if !s.Tasks["T2"].Hidden() || !s.Tasks["T3"].Hidden() || s.Tasks["T1"].Hidden() {
		t.Fatalf("Hidden() = %v %v %v, want false true true", s.Tasks["T1"].Hidden(), s.Tasks["T2"].Hidden(), s.Tasks["T3"].Hidden())
	}
	if got := ids(s.HistoryTasks()); len(got) != 1 || got[0] != "T1" {
		t.Fatalf("HistoryTasks = %v, want [T1]", got)
	}
	if got := ids(s.HiddenTasks()); len(got) != 2 || got[0] != "T3" || got[1] != "T2" {
		t.Fatalf("HiddenTasks = %v, want [T3 T2], newest hide first", got)
	}
}

func TestHideItemMakesHiddenLinkedTask(t *testing.T) {
	l := &log{t: t}
	now := at("2026-11-27T11:00:00Z")
	ops, id, err := HideItem(Replay(nil), withProject(gh533()), now)
	if err != nil {
		t.Fatal(err)
	}
	s := l.add(ops...)
	task := s.Tasks[id]
	if task == nil || task.Status != StatusDropped || task.OnWeek || !task.Hidden() ||
		task.Ext("gh", "item") != "CoBrALab/RABIES#533" || task.Ext("gh", "hidden_at") != "2026-11-27T11:00:00Z" ||
		s.ProjectName(task.Project) != "RABIES" {
		t.Fatalf("task = %+v", task)
	}
	if got := ids(s.HiddenTasks()); len(got) != 1 || got[0] != id {
		t.Fatalf("HiddenTasks = %v", got)
	}
}

func TestHideTask(t *testing.T) {
	l := &log{t: t}
	now := at("2026-11-27T11:00:00Z")
	ops, id, _ := LinkItem(Replay(nil), gh533(), now)
	s := l.add(ops...)

	s = l.add(HideTask(s.Tasks[id], "gh", now.Add(time.Hour))...)
	if task := s.Tasks[id]; task.Status != StatusDropped || task.Ext("gh", "hidden_at") != "2026-11-27T12:00:00Z" {
		t.Fatalf("open task, hidden: status %s, hidden_at %q", task.Status, task.Ext("gh", "hidden_at"))
	}
	s = l.add(HideTask(s.Tasks[id], "gh", now.Add(2*time.Hour))...)
	if got := s.Tasks[id].Ext("gh", "hidden_at"); got != "2026-11-27T13:00:00Z" {
		t.Fatalf("hidden again: hidden_at %q, want the new time", got)
	}

	ops, done, _ := LinkItem(s, gh533(), now)
	s = l.add(ops...)
	s = l.add(SetStatus(s.Tasks[done], StatusDone, "", now)...)
	s = l.add(HideTask(s.Tasks[done], "gh", now.Add(time.Hour))...)
	if task := s.Tasks[done]; task.Status != StatusDone || !task.Hidden() {
		t.Fatalf("done task, hidden: status %s, hidden %v; want done, hidden", task.Status, task.Hidden())
	}
}

func TestUnhideAndTaskFromHidden(t *testing.T) {
	l := &log{t: t}
	now := at("2026-11-27T11:00:00Z")
	ops, a, _ := HideItem(Replay(nil), gh533(), now)
	s := l.add(ops...)
	r2 := gh533()
	r2.Key = "CoBrALab/RABIES#540"
	ops, b, _ := HideItem(s, r2, now)
	s = l.add(ops...)

	s = l.add(Unhide(s.Tasks[a], "gh", now.Add(time.Hour))...)
	if st, _ := s.ClassifyItem("gh", "CoBrALab/RABIES#533", now, true); st != ItemNew {
		t.Fatalf("after Unhide, item is %v, want ItemNew", st)
	}
	if task := s.Tasks[a]; task.Hidden() || task.Status != StatusDropped {
		t.Fatalf("after Unhide, task hidden %v status %s; want a plain dropped task", task.Hidden(), task.Status)
	}
	if got := ids(s.HistoryTasks()); len(got) != 1 || got[0] != a {
		t.Fatalf("History = %v, want the unhidden task", got)
	}

	s = l.add(TaskFromHidden(s.Tasks[b], "gh", now.Add(time.Hour))...)
	task := s.Tasks[b]
	if task.Hidden() || task.Status != StatusTodo || task.OnWeek || task.CompletedAt != nil || task.Ext("gh", "item") != "CoBrALab/RABIES#540" {
		t.Fatalf("after TaskFromHidden: %+v; want todo, backlog, not hidden, still linked", task)
	}
	if got := ids(s.HiddenTasks()); len(got) != 0 {
		t.Fatalf("HiddenTasks = %v, want none", got)
	}
}

func TestClassifyClosedAndHidden(t *testing.T) {
	l := &log{t: t}
	linkedAt := at("2026-11-27T10:00:00Z")
	key := "CoBrALab/RABIES#533"
	ops, id, _ := LinkItem(Replay(nil), gh533(), linkedAt)
	s := l.add(ops...)
	check := func(name string, updated time.Time, persistent bool, want ItemState) {
		t.Helper()
		if st, task := s.ClassifyItem("gh", key, updated, persistent); st != want || task == nil || task.ID != id {
			t.Errorf("%s: %v, %v; want %v, the task", name, st, task, want)
		}
	}
	before := linkedAt.Add(-time.Hour)

	check("open task, persistent", before, true, ItemLinked)

	s = l.add(SetStatus(s.Tasks[id], StatusDone, "", linkedAt.Add(time.Minute))...)
	check("done task, persistent", before, true, ItemClosedOpen)
	check("done task, not persistent", before, false, ItemLinked)
	check("done task, new activity", linkedAt.Add(time.Hour), true, ItemUpdated)

	hiddenAt := linkedAt.Add(2 * time.Hour)
	s = l.add(HideTask(s.Tasks[id], "gh", hiddenAt)...)
	check("hidden, activity before the hide", linkedAt.Add(time.Hour), true, ItemLinked)
	check("hidden, activity after the hide", hiddenAt.Add(time.Minute), true, ItemHiddenActivity)
	check("hidden, activity after the hide, not persistent", hiddenAt.Add(time.Minute), false, ItemHiddenActivity)
}
