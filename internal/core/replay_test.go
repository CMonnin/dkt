package core

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"slices"
	"testing"
	"time"
)

func at(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func mkCreate(id, task, ts, title string, tags ...string) Op {
	w := at(ts)
	return Op{ID: id, TS: w, Type: OpTaskCreate, Target: task, Task: &TaskCreate{
		Title: title, Category: Work, Status: StatusTodo, OnWeek: true, WeekAddedAt: &w, Tags: tags,
	}}
}

func mkSet(id, task, ts, field string, v any) Op {
	b, _ := json.Marshal(v)
	return Op{ID: id, TS: at(ts), Type: OpTaskSet, Target: task, Field: field, Value: b}
}

func mkTag(id string, typ OpType, task, ts, tag string) Op {
	return Op{ID: id, TS: at(ts), Type: typ, Target: task, Tag: tag}
}

func TestReplayIndependentOfReadOrder(t *testing.T) {
	machineA := []Op{
		mkCreate("01A1", "T1", "2026-09-14T09:00:00Z", "one"),
		mkSet("01A2", "T1", "2026-09-14T10:00:00Z", "title", "one from a"),
		mkSet("01A3", "T1", "2026-09-14T12:00:00Z", "status", "doing"),
		mkTag("01A4", OpTagAdd, "T2", "2026-09-14T12:30:00Z", "x"),
	}
	machineB := []Op{
		mkCreate("01B1", "T2", "2026-09-14T09:30:00Z", "two"),
		mkSet("01B2", "T1", "2026-09-14T11:00:00Z", "title", "one from b"),
		mkSet("01B3", "T2", "2026-09-14T11:30:00Z", "on_week", false),
		mkTag("01B4", OpTagRemove, "T2", "2026-09-14T13:00:00Z", "x"),
	}
	want := Replay(append(slices.Clone(machineA), machineB...))
	if got := Replay(append(slices.Clone(machineB), machineA...)); !reflect.DeepEqual(got, want) {
		t.Fatalf("B then A differs from A then B")
	}
	r := rand.New(rand.NewSource(1))
	for i := range 20 {
		ops := append(slices.Clone(machineA), machineB...)
		r.Shuffle(len(ops), func(i, j int) { ops[i], ops[j] = ops[j], ops[i] })
		if got := Replay(ops); !reflect.DeepEqual(got, want) {
			t.Fatalf("shuffle %d differs", i)
		}
	}
	if got := want.Tasks["T1"]; got.Title != "one from b" || got.Status != StatusDoing {
		t.Errorf("T1 = %q/%s, want 'one from b'/doing", got.Title, got.Status)
	}
	if got := want.Tasks["T2"]; got.OnWeek || len(got.Tags) != 0 {
		t.Errorf("T2 on_week=%v tags=%v, want false/none", got.OnWeek, got.Tags)
	}
}

func TestPerFieldLWWKeepsConcurrentEditsToDifferentFields(t *testing.T) {
	s := Replay([]Op{
		mkCreate("01A1", "T1", "2026-09-14T09:00:00Z", "old"),
		mkSet("01A2", "T1", "2026-09-14T10:00:00Z", "title", "new title"), // machine A
		mkSet("01B1", "T1", "2026-09-14T10:00:05Z", "status", "done"),     // machine B
	})
	if got := s.Tasks["T1"]; got.Title != "new title" || got.Status != StatusDone {
		t.Fatalf("got %q/%s, want both edits", got.Title, got.Status)
	}
}

func TestSameFieldLaterTimestampWins(t *testing.T) {
	s := Replay([]Op{
		mkCreate("01A1", "T1", "2026-09-14T09:00:00Z", "old"),
		mkSet("01ZZ", "T1", "2026-09-14T10:00:00Z", "title", "earlier, higher id"),
		mkSet("01AA", "T1", "2026-09-14T10:00:01Z", "title", "later"),
	})
	if got := s.Tasks["T1"].Title; got != "later" {
		t.Fatalf("title = %q, want later", got)
	}
}

func TestSameFieldEqualTimestampHigherULIDWins(t *testing.T) {
	s := Replay([]Op{
		mkCreate("01A1", "T1", "2026-09-14T09:00:00Z", "old"),
		mkSet("01B9", "T1", "2026-09-14T10:00:00Z", "title", "higher"),
		mkSet("01B1", "T1", "2026-09-14T10:00:00Z", "title", "lower"),
	})
	if got := s.Tasks["T1"].Title; got != "higher" {
		t.Fatalf("title = %q, want higher", got)
	}
}

func TestTagsAreSetsNotLWW(t *testing.T) {
	ops := []Op{
		mkCreate("01A1", "T1", "2026-09-14T09:00:00Z", "t", "base"),
		mkTag("01A2", OpTagAdd, "T1", "2026-09-14T10:00:00Z", "x"),
		mkTag("01B2", OpTagAdd, "T1", "2026-09-14T10:00:00Z", "#Y"),
	}
	if got := Replay(ops).Tasks["T1"].Tags; !slices.Equal(got, []string{"base", "x", "y"}) {
		t.Fatalf("tags = %v, want [base x y]", got)
	}
	ops = append(ops, mkTag("01A3", OpTagRemove, "T1", "2026-09-14T11:00:00Z", "x"))
	if got := Replay(ops).Tasks["T1"].Tags; !slices.Equal(got, []string{"base", "y"}) {
		t.Fatalf("after remove tags = %v, want [base y]", got)
	}
}

func TestOpsForUncreatedTaskAreIgnored(t *testing.T) {
	s := Replay([]Op{mkSet("01A1", "GHOST", "2026-09-14T10:00:00Z", "title", "boo")})
	if len(s.Tasks) != 0 {
		t.Fatalf("got %d tasks, want 0", len(s.Tasks))
	}
}

func TestOpsSurviveJSONRoundTrip(t *testing.T) {
	now := at("2026-09-14T10:00:00Z")
	create, err := CreateTask(Replay(nil), NewTask{Title: "t", Tags: []string{"a"}}, now)
	if err != nil {
		t.Fatal(err)
	}
	s := Replay([]Op{create})
	ops := append([]Op{create}, SetStatus(s.Tasks[create.Target], StatusDone, "", now.Add(time.Hour))...)

	var decoded []Op
	for _, op := range ops {
		b, err := json.Marshal(op)
		if err != nil {
			t.Fatal(err)
		}
		var d Op
		if err := json.Unmarshal(b, &d); err != nil {
			t.Fatal(err)
		}
		decoded = append(decoded, d)
	}
	if want, got := Replay(ops), Replay(decoded); !reflect.DeepEqual(want, got) {
		t.Fatalf("round trip changed state:\nwant %+v\ngot  %+v", want.Tasks[create.Target], got.Tasks[create.Target])
	}
}
