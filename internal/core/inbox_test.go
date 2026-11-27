package core

import "testing"

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
