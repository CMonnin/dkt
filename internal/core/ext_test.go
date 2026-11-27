package core

import (
	"testing"
	"time"
)

func TestReplayTaskExtField(t *testing.T) {
	s := Replay([]Op{
		mkCreate("01A1", "T1", "2026-11-27T09:00:00Z", "review PR"),
		mkSet("01A2", "T1", "2026-11-27T09:01:00Z", "x.gh.item", "CoBrALab/RABIES#533"),
	})
	if got := s.Tasks["T1"].Ext("gh", "item"); got != "CoBrALab/RABIES#533" {
		t.Fatalf("Ext(gh, item) = %q, want %q", got, "CoBrALab/RABIES#533")
	}
}

func TestReplayTaskExtFieldEmptyRemovesKey(t *testing.T) {
	s := Replay([]Op{
		mkCreate("01A1", "T1", "2026-11-27T09:00:00Z", "review PR"),
		mkSet("01A2", "T1", "2026-11-27T09:01:00Z", "x.gh.item", "CoBrALab/RABIES#533"),
		mkSet("01A3", "T1", "2026-11-27T09:02:00Z", "x.gh.item", ""),
	})
	if got := s.Tasks["T1"].Ext("gh", "item"); got != "" {
		t.Fatalf("Ext(gh, item) = %q after clearing, want \"\"", got)
	}
}

func TestReplayTaskExtFieldLaterWinsAcrossMachines(t *testing.T) {
	create := mkCreate("01A1", "T1", "2026-11-27T09:00:00Z", "review PR")
	machineA := mkSet("01A2", "T1", "2026-11-27T09:05:00Z", "x.gh.item", "CoBrALab/RABIES#600")
	machineB := mkSet("01B1", "T1", "2026-11-27T09:01:00Z", "x.gh.item", "CoBrALab/RABIES#533")
	for name, ops := range map[string][]Op{
		"A then B": {create, machineA, machineB},
		"B then A": {create, machineB, machineA},
	} {
		if got := Replay(ops).Tasks["T1"].Ext("gh", "item"); got != "CoBrALab/RABIES#600" {
			t.Errorf("%s: Ext(gh, item) = %q, want the later value %q", name, got, "CoBrALab/RABIES#600")
		}
	}
}

func mkProjCreate(id, proj, ts, name string) Op {
	return Op{ID: id, TS: at(ts), Type: OpProjectCreate, Target: proj, Project: &ProjectCreate{Name: name, Category: Work}}
}

func mkProjSet(id, proj, ts, field string, v any) Op {
	op := mkSet(id, proj, ts, field, v)
	op.Type = OpProjectSet
	return op
}

func TestReplayProjectExtField(t *testing.T) {
	s := Replay([]Op{
		mkProjCreate("01A1", "P1", "2026-11-27T09:00:00Z", "RABIES"),
		mkProjSet("01A2", "P1", "2026-11-27T09:01:00Z", "x.gh.project.CoBrALab/RABIES", "1"),
	})
	if got := s.Projects["P1"].Ext("gh", "project.CoBrALab/RABIES"); got != "1" {
		t.Fatalf("Ext(gh, project.CoBrALab/RABIES) = %q, want %q", got, "1")
	}
}

func TestReplayProjectExtFieldEmptyRemovesKey(t *testing.T) {
	s := Replay([]Op{
		mkProjCreate("01A1", "P1", "2026-11-27T09:00:00Z", "RABIES"),
		mkProjSet("01A2", "P1", "2026-11-27T09:01:00Z", "x.gh.project.CoBrALab/RABIES", "1"),
		mkProjSet("01A3", "P1", "2026-11-27T09:02:00Z", "x.gh.project.CoBrALab/RABIES", ""),
	})
	if got := s.Projects["P1"].Ext("gh", "project.CoBrALab/RABIES"); got != "" {
		t.Fatalf("Ext after clearing = %q, want \"\"", got)
	}
}

func TestReplayProjectExtFieldLaterWinsAcrossMachines(t *testing.T) {
	create := mkProjCreate("01A1", "P1", "2026-11-27T09:00:00Z", "RABIES")
	machineA := mkProjSet("01A2", "P1", "2026-11-27T09:05:00Z", "x.gh.project.CoBrALab/RABIES", "")
	machineB := mkProjSet("01B1", "P1", "2026-11-27T09:01:00Z", "x.gh.project.CoBrALab/RABIES", "1")
	for name, ops := range map[string][]Op{
		"A then B": {create, machineA, machineB},
		"B then A": {create, machineB, machineA},
	} {
		if got := Replay(ops).Projects["P1"].Ext("gh", "project.CoBrALab/RABIES"); got != "" {
			t.Errorf("%s: Ext = %q, want the later (cleared) value", name, got)
		}
	}
}

func TestSetTaskExtReplaysToValue(t *testing.T) {
	l := &log{t: t}
	now := at("2026-11-27T09:00:00Z")
	s, id := l.must(CreateTask(Replay(nil), NewTask{Title: "review PR"}, now))
	ops, err := SetTaskExt(s.Tasks[id], "gh", "item", "CoBrALab/RABIES#533", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	s = l.add(ops...)
	if got := s.Tasks[id].Ext("gh", "item"); got != "CoBrALab/RABIES#533" {
		t.Fatalf("Ext(gh, item) = %q, want %q", got, "CoBrALab/RABIES#533")
	}
}

func TestSetProjectExtReplaysToValue(t *testing.T) {
	l := &log{t: t}
	now := at("2026-11-27T09:00:00Z")
	s, pid := l.must(CreateProject(Replay(nil), "RABIES", Work, "", now))
	ops, err := SetProjectExt(s.Projects[pid], "gh", "project.CoBrALab/RABIES", "1", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	s = l.add(ops...)
	if got := s.Projects[pid].Ext("gh", "project.CoBrALab/RABIES"); got != "1" {
		t.Fatalf("Ext = %q, want %q", got, "1")
	}
}

func TestSetTaskExtRejectsBadPluginName(t *testing.T) {
	task := &Task{ID: "T1"}
	for _, plugin := range []string{"", "GH", "g.h", "g h", "gh/x", "gh_x"} {
		if _, err := SetTaskExt(task, plugin, "item", "v", at("2026-11-27T09:00:00Z")); err == nil {
			t.Errorf("SetTaskExt(plugin %q) succeeded, want error", plugin)
		}
	}
	for _, plugin := range []string{"gh", "my-plugin", "x2"} {
		if _, err := SetTaskExt(task, plugin, "item", "v", at("2026-11-27T09:00:00Z")); err != nil {
			t.Errorf("SetTaskExt(plugin %q) = %v, want ok", plugin, err)
		}
	}
}

func TestSetTaskExtKeyRules(t *testing.T) {
	task := &Task{ID: "T1"}
	now := at("2026-11-27T09:00:00Z")
	if _, err := SetTaskExt(task, "gh", "", "v", now); err == nil {
		t.Errorf("SetTaskExt with empty key succeeded, want error")
	}
	if _, err := SetTaskExt(task, "gh", "project.CoBrALab/RABIES", "1", now); err != nil {
		t.Errorf("SetTaskExt with key containing . and / = %v, want ok", err)
	}
}

func TestSetTaskExtRejectsNewlineInValue(t *testing.T) {
	task := &Task{ID: "T1"}
	for _, v := range []string{"a\nb", "a\r\nb", "a\n"} {
		if _, err := SetTaskExt(task, "gh", "item", v, at("2026-11-27T09:00:00Z")); err == nil {
			t.Errorf("SetTaskExt(value %q) succeeded, want error", v)
		}
	}
}

func TestSetProjectExtValidates(t *testing.T) {
	p := &Project{ID: "P1"}
	now := at("2026-11-27T09:00:00Z")
	for _, c := range []struct{ plugin, key, value string }{
		{"GH", "item", "v"},
		{"gh", "", "v"},
		{"gh", "item", "a\nb"},
	} {
		if _, err := SetProjectExt(p, c.plugin, c.key, c.value, now); err == nil {
			t.Errorf("SetProjectExt(%q, %q, %q) succeeded, want error", c.plugin, c.key, c.value)
		}
	}
}
