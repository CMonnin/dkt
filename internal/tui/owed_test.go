package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/CMonnin/dkt/internal/core"
	"github.com/CMonnin/dkt/internal/plugin"
)

func owed(id, group string, hoursAgo int) plugin.Item {
	it := item(id, group, hoursAgo)
	it.Persistent = true
	return it
}

// stateWith links gh items to tasks: done for "done", hidden for "hid".
func stateWith(t *testing.T) *core.State {
	t.Helper()
	var ops []core.Op
	st := core.Replay(nil)
	add := func(more ...core.Op) { ops = append(ops, more...); st = core.Replay(ops) }
	link := func(id string) string {
		o, tid, err := core.LinkItem(st, core.LinkRequest{Plugin: "gh", Key: "o/r#" + id, Title: "task " + id}, t0)
		if err != nil {
			t.Fatal(err)
		}
		add(o...)
		return tid
	}
	done := link("done")
	add(core.SetStatus(st.Tasks[done], core.StatusDone, "", t0)...)
	hid := link("hid")
	add(core.HideTask(st.Tasks[hid], "gh", t0)...)
	link("open")
	return st
}

func TestInboxShowsOwedWorkAndTheNewUpdates(t *testing.T) {
	st := stateWith(t)
	hidNew := owed("hid", "needs-you", 0)
	hidNew.UpdatedAt = t0.Add(time.Minute) // activity after the hide
	ib := newInbox([]string{"gh"})
	ib.update("gh", ghList(
		owed("new", "needs-you", 2),
		owed("done", "needs-you", 5), // task done, still open
		hidNew,
		owed("open", "needs-you", 3), // task open: tracked, not shown
	), nil)
	rows := ib.rows(st)
	if got, want := outline(rows), "# ↻ Updates\n  gh:hid\n  gh:done\n# Needs you\n  gh:new\n"; got != want {
		t.Fatalf("rows =\n%s\nwant\n%s", got, want)
	}
	states := map[string]core.ItemState{}
	for _, r := range rows {
		if r.inbox != nil {
			states[r.inbox.item.ID] = r.inbox.state
		}
	}
	if states["hid"] != core.ItemHiddenActivity || states["done"] != core.ItemClosedOpen || states["new"] != core.ItemNew {
		t.Fatalf("states = %v", states)
	}

	// A hidden task with nothing new stays out of the inbox.
	ib.update("gh", ghList(owed("hid", "needs-you", 5)), nil)
	if got := outline(ib.rows(st)); got != "" {
		t.Fatalf("rows =\n%s\nwant none", got)
	}
}

func TestWhichInboxRowsCanBeDismissed(t *testing.T) {
	for _, c := range []struct {
		persistent bool
		state      core.ItemState
		want       bool
	}{
		{false, core.ItemNew, true},
		{false, core.ItemUpdated, true},
		{true, core.ItemNew, false},
		{true, core.ItemUpdated, true},
		{true, core.ItemHiddenActivity, true},
		{true, core.ItemClosedOpen, false},
	} {
		e := entry{item: plugin.Item{Persistent: c.persistent}, state: c.state}
		if got := e.dismissible(); got != c.want {
			t.Errorf("persistent %v, state %v: dismissible %v, want %v", c.persistent, c.state, got, c.want)
		}
	}
}

func TestInboxKnowsWhatIsStillListed(t *testing.T) {
	ib := newInbox([]string{"gh"})
	if !ib.stillListed("gh", "o/r#1") {
		t.Fatal("before any list, stillListed should give the benefit of the doubt")
	}
	ib.update("gh", ghList(owed("1", "needs-you", 1)), nil)
	if !ib.stillListed("gh", "o/r#1") || ib.stillListed("gh", "o/r#2") {
		t.Fatal("stillListed should follow the latest list")
	}
}

func TestHiddenRowsDrawNoControlCharacters(t *testing.T) {
	// A key stored before plugin output was checked could hold anything.
	evil := "o/r#1\x1b]0;pwned\x07\x1b[2J"
	ops, id, err := core.LinkItem(core.Replay(nil), core.LinkRequest{Plugin: "gh", Key: evil, Title: "title\x1b[31m"}, t0)
	if err != nil {
		t.Fatal(err)
	}
	st := core.Replay(ops)
	st = core.Replay(append(ops, core.HideTask(st.Tasks[id], "gh", t0)...))
	m := &Model{ib: newInbox([]string{"gh"}), now: func() time.Time { return t0 }}
	for _, listed := range []bool{true, false} {
		if !listed {
			m.ib.update("gh", ghList(), nil) // a list without it: "no longer open"
		}
		if got := m.renderHidden(st.Tasks[id]); strings.ContainsAny(got, "\x07\x9b") || strings.Contains(stripStyles(got), "\x1b") {
			t.Fatalf("listed %v: row %q has control characters", listed, got)
		}
	}
}

// stripStyles removes the SGR sequences lipgloss adds, so only text the
// row got from data is left to check.
func stripStyles(s string) string { return sgr.ReplaceAllString(s, "") }

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)
