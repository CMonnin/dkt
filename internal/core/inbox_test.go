package core

import (
	"testing"
	"time"
)

func TestClassifyUnlinkedItemIsNew(t *testing.T) {
	s := Replay([]Op{
		mkCreate("01A1", "T1", "2026-11-27T09:00:00Z", "unrelated"),
		mkSet("01A2", "T1", "2026-11-27T09:01:00Z", "x.gh.item", "CoBrALab/RABIES#520"),
	})
	st, task := s.ClassifyItem("gh", "CoBrALab/RABIES#533", at("2026-11-27T11:00:00Z"))
	if st != ItemNew || task != nil {
		t.Fatalf("ClassifyItem = %v, %v; want ItemNew, nil", st, task)
	}
}

// linked is a task linked to plugin item key at linkedAt.
func linked(id, plugin, key, linkedAt string) []Op {
	return []Op{
		mkCreate(id+"c", id, "2026-11-27T09:00:00Z", "task "+id),
		mkSet(id+"i", id, linkedAt, "x."+plugin+".item", key),
		mkSet(id+"l", id, linkedAt, "x."+plugin+".linked_at", linkedAt),
	}
}

func TestClassifyLinkedItemWithNoNewActivity(t *testing.T) {
	s := Replay(linked("T1", "gh", "CoBrALab/RABIES#533", "2026-11-27T10:00:00Z"))
	for _, updated := range []string{"2026-11-27T09:30:00Z", "2026-11-27T10:00:00Z"} {
		st, task := s.ClassifyItem("gh", "CoBrALab/RABIES#533", at(updated))
		if st != ItemLinked || task == nil || task.ID != "T1" {
			t.Errorf("updated %s: ClassifyItem = %v, %v; want ItemLinked, T1", updated, st, task)
		}
	}
}

func TestClassifyLinkedItemWithNewActivity(t *testing.T) {
	s := Replay(linked("T1", "gh", "CoBrALab/RABIES#533", "2026-11-27T10:00:00Z"))
	st, task := s.ClassifyItem("gh", "CoBrALab/RABIES#533", at("2026-11-27T10:00:01Z"))
	if st != ItemUpdated || task == nil || task.ID != "T1" {
		t.Fatalf("ClassifyItem = %v, %v; want ItemUpdated, T1", st, task)
	}
}

func TestClassifyPicksMostRecentlyLinkedTask(t *testing.T) {
	ops := linked("T1", "gh", "CoBrALab/RABIES#533", "2026-11-27T10:00:00Z")
	ops = append(ops, linked("T2", "gh", "CoBrALab/RABIES#533", "2026-11-27T10:05:00Z")...)
	ops = append(ops, linked("T3", "gh", "CoBrALab/RABIES#533", "2026-11-27T09:55:00Z")...)
	s := Replay(ops)
	for range 50 { // map order is random; the answer must not be
		if _, task := s.ClassifyItem("gh", "CoBrALab/RABIES#533", at("2026-11-27T11:00:00Z")); task == nil || task.ID != "T2" {
			t.Fatalf("ClassifyItem picked %v, want T2, the most recently linked", task)
		}
	}
}

func gh533() LinkRequest {
	return LinkRequest{
		Plugin: "gh", Key: "CoBrALab/RABIES#533",
		Title: "Review: fix motion correction (RABIES#533)", Category: Work, Tags: []string{"gh"},
		Note: "https://github.com/CoBrALab/RABIES/pull/533",
	}
}

func TestLinkItemMakesLinkedTaskOnWeek(t *testing.T) {
	l := &log{t: t}
	now := at("2026-11-27T11:00:00Z")
	ops, id, err := LinkItem(Replay(nil), gh533(), now)
	if err != nil {
		t.Fatal(err)
	}
	task := l.add(ops...).Tasks[id]
	if task == nil {
		t.Fatalf("no task %q after replaying %d ops", id, len(ops))
	}
	if task.Title != "Review: fix motion correction (RABIES#533)" || task.Category != Work ||
		task.Note != "https://github.com/CoBrALab/RABIES/pull/533" || !task.HasTag("gh") ||
		!task.OnWeek || task.Project != "" || task.Status != StatusTodo {
		t.Fatalf("task = %+v", task)
	}
	if got := task.Ext("gh", "item"); got != "CoBrALab/RABIES#533" {
		t.Errorf("x.gh.item = %q", got)
	}
	if got := task.Ext("gh", "linked_at"); got != "2026-11-27T11:00:00Z" {
		t.Errorf("x.gh.linked_at = %q, want 2026-11-27T11:00:00Z", got)
	}
}

func withProject(r LinkRequest) LinkRequest {
	r.ProjectKey, r.ProjectName = "CoBrALab/RABIES", "RABIES"
	return r
}

func TestLinkItemUsesProjectLinkedByKey(t *testing.T) {
	l := &log{t: t}
	now := at("2026-11-27T11:00:00Z")
	s, pid := l.must(CreateProject(Replay(nil), "Preprocessing pipeline", Personal, "", now))
	ops, err := SetProjectExt(s.Projects[pid], "gh", "project.CoBrALab/RABIES", "1", now)
	if err != nil {
		t.Fatal(err)
	}
	s = l.add(ops...)

	ops, id, err := LinkItem(s, withProject(gh533()), now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	s = l.add(ops...)
	if got := s.Tasks[id].Project; got != pid {
		t.Fatalf("task project = %q (%s), want the linked project %q", got, s.ProjectName(got), pid)
	}
	if n := len(s.Projects); n != 1 {
		t.Fatalf("%d projects, want 1: no new project", n)
	}
	if got := s.Tasks[id].Category; got != Personal {
		t.Fatalf("task category = %q, want the project's, personal", got)
	}
}

func TestLinkItemLinksProjectWithSameName(t *testing.T) {
	l := &log{t: t}
	now := at("2026-11-27T11:00:00Z")
	s, pid := l.must(CreateProject(Replay(nil), "rabies", Work, "", now))

	ops, id, err := LinkItem(s, withProject(gh533()), now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	s = l.add(ops...)
	if got := s.Tasks[id].Project; got != pid {
		t.Fatalf("task project = %q, want %q, the project named like the suggestion", got, pid)
	}
	if got := s.Projects[pid].Ext("gh", "project.CoBrALab/RABIES"); got != "1" {
		t.Fatalf("x.gh.project.CoBrALab/RABIES = %q, want \"1\": the project should now be linked", got)
	}
	if n := len(s.Projects); n != 1 {
		t.Fatalf("%d projects, want 1", n)
	}
}

func TestLinkItemCreatesLinkedProject(t *testing.T) {
	l := &log{t: t}
	now := at("2026-11-27T11:00:00Z")
	r := withProject(gh533())
	r.Category = Personal

	ops, id, err := LinkItem(Replay(nil), r, now)
	if err != nil {
		t.Fatal(err)
	}
	s := l.add(ops...)
	p := s.Projects[s.Tasks[id].Project]
	if p == nil {
		t.Fatalf("task has no project; want a new one")
	}
	if p.Name != "RABIES" || p.Category != Personal {
		t.Fatalf("project = %q (%s), want RABIES (personal)", p.Name, p.Category)
	}
	if got := p.Ext("gh", "project.CoBrALab/RABIES"); got != "1" {
		t.Fatalf("new project not linked: x.gh.project.CoBrALab/RABIES = %q", got)
	}

	// A second item from the same repo goes into the same project.
	r2 := withProject(gh533())
	r2.Key, r2.Title = "CoBrALab/RABIES#540", "Reply: atlas question (RABIES#540)"
	ops, id2, err := LinkItem(s, r2, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	s = l.add(ops...)
	if got := s.Tasks[id2].Project; got != p.ID || len(s.Projects) != 1 {
		t.Fatalf("second item went to project %q with %d projects; want %q and 1", got, len(s.Projects), p.ID)
	}
}

func TestLinkItemSkipsArchivedProjects(t *testing.T) {
	l := &log{t: t}
	now := at("2026-11-27T11:00:00Z")
	s, byKey := l.must(CreateProject(Replay(nil), "Old pipeline", Work, "", now))
	ops, _ := SetProjectExt(s.Projects[byKey], "gh", "project.CoBrALab/RABIES", "1", now)
	s = l.add(ops...)
	s, byName := l.must(CreateProject(s, "RABIES", Work, "", now))
	s = l.add(ArchiveProject(s, s.Projects[byKey], now)...)
	s = l.add(ArchiveProject(s, s.Projects[byName], now)...)

	ops, id, err := LinkItem(s, withProject(gh533()), now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	s = l.add(ops...)
	if got := s.Tasks[id].Project; got == byKey || got == byName || got == "" {
		t.Fatalf("task project = %q; want a new project, not an archived one", got)
	}
}

func TestLinkItemRejectsBadPluginOrKey(t *testing.T) {
	now := at("2026-11-27T11:00:00Z")
	for _, c := range []struct{ plugin, key string }{{"GH", "a/b#1"}, {"gh", ""}, {"gh", "a/b#1\nx"}} {
		r := gh533()
		r.Plugin, r.Key = c.plugin, c.key
		if ops, _, err := LinkItem(Replay(nil), r, now); err == nil {
			t.Errorf("LinkItem(plugin %q, key %q) = %d ops, want an error", c.plugin, c.key, len(ops))
		}
	}
}
