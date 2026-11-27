package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/CMonnin/dkt/internal/core"
	"github.com/CMonnin/dkt/internal/plugin"
)

var t0 = time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)

func item(id, group string, hoursAgo int) plugin.Item {
	return plugin.Item{
		ID: id, Key: "o/r#" + id, Group: group, Title: "item " + id,
		UpdatedAt: t0.Add(-time.Duration(hoursAgo) * time.Hour),
	}
}

func ghList(items ...plugin.Item) plugin.List {
	return plugin.List{
		Groups: []plugin.Group{{ID: "needs-you", Name: "Needs you"}, {ID: "fyi", Name: "FYI"}},
		Items:  items, Cursor: "c1", PollAfter: time.Minute,
	}
}

// outline is the inbox rows as text: "# header" or "  plugin:id".
func outline(rows []row) string {
	var b strings.Builder
	for _, r := range rows {
		if r.header != "" {
			fmt.Fprintf(&b, "# %s\n", r.header)
		} else {
			fmt.Fprintf(&b, "  %s:%s\n", r.inbox.plugin, r.inbox.item.ID)
		}
	}
	return b.String()
}

func TestInboxGroupsNewItemsNewestFirst(t *testing.T) {
	ib := newInbox([]string{"gh"})
	ib.update("gh", ghList(item("1", "fyi", 1), item("2", "needs-you", 5), item("3", "needs-you", 2)), nil)
	want := "# Needs you\n  gh:3\n  gh:2\n# FYI\n  gh:1\n"
	if got := outline(ib.rows(core.Replay(nil))); got != want {
		t.Fatalf("rows =\n%s\nwant\n%s", got, want)
	}
}

// linkedState has a task linked to gh item o/r#<id> at t0.
func linkedState(t *testing.T, id string) (*core.State, string) {
	t.Helper()
	ops, taskID, err := core.LinkItem(core.Replay(nil), core.LinkRequest{Plugin: "gh", Key: "o/r#" + id, Title: "task for " + id}, t0)
	if err != nil {
		t.Fatal(err)
	}
	return core.Replay(ops), taskID
}

func TestInboxHidesHandledItemsAndRelinksThem(t *testing.T) {
	st, taskID := linkedState(t, "2")
	ib := newInbox([]string{"gh"})
	ib.update("gh", ghList(item("1", "fyi", 1), item("2", "needs-you", 3)), nil) // 2: activity before the link
	if got, want := outline(ib.rows(st)), "# FYI\n  gh:1\n"; got != want {
		t.Fatalf("rows =\n%s\nwant\n%s", got, want)
	}
	re := ib.relinks(st)
	if len(re) != 1 || re[0].plugin != "gh" || re[0].item.ID != "2" || re[0].task.ID != taskID {
		t.Fatalf("relinks = %+v, want gh item 2 with its task", re)
	}
}

func TestInboxShowsUpdatesFirst(t *testing.T) {
	st, taskID := linkedState(t, "2")
	updated := item("2", "needs-you", 0)
	updated.UpdatedAt = t0.Add(time.Minute) // activity after the link
	ib := newInbox([]string{"gh"})
	ib.update("gh", ghList(item("1", "needs-you", 1), updated), nil)
	rows := ib.rows(st)
	if got, want := outline(rows), "# ↻ Updates\n  gh:2\n# Needs you\n  gh:1\n"; got != want {
		t.Fatalf("rows =\n%s\nwant\n%s", got, want)
	}
	if rows[1].inbox.task == nil || rows[1].inbox.task.ID != taskID {
		t.Fatalf("update row has task %v, want %s", rows[1].inbox.task, taskID)
	}
	if re := ib.relinks(st); len(re) != 0 {
		t.Fatalf("relinks = %+v, want none: an update is shown, not relinked", re)
	}
}

func TestInboxShowsUndeclaredGroupsAsOther(t *testing.T) {
	ib := newInbox([]string{"gh"})
	ib.update("gh", ghList(item("1", "fyi", 1), item("2", "mystery", 2), item("3", "", 3)), nil)
	want := "# FYI\n  gh:1\n# Other\n  gh:2\n  gh:3\n"
	if got := outline(ib.rows(core.Replay(nil))); got != want {
		t.Fatalf("rows =\n%s\nwant\n%s", got, want)
	}
}

func TestInboxKeepsLastListWhenUnchangedOrFailed(t *testing.T) {
	st := core.Replay(nil)
	ib := newInbox([]string{"gh"})
	ib.update("gh", ghList(item("1", "fyi", 1)), nil)
	want := outline(ib.rows(st))

	ib.update("gh", plugin.List{Unchanged: true, PollAfter: time.Minute}, nil)
	if got := outline(ib.rows(st)); got != want || ib.cursor("gh") != "c1" || ib.stale() {
		t.Fatalf("after unchanged: rows %q cursor %q stale %v; want the same rows, c1, not stale", got, ib.cursor("gh"), ib.stale())
	}

	ib.update("gh", plugin.List{}, fmt.Errorf("dkt-gh: offline"))
	if got := outline(ib.rows(st)); got != want || ib.cursor("gh") != "c1" || !ib.stale() {
		t.Fatalf("after failure: rows %q cursor %q stale %v; want the same rows, c1, stale", got, ib.cursor("gh"), ib.stale())
	}
	if ib.stopped("gh") {
		t.Fatal("an ordinary failure stopped polling")
	}

	ib.update("gh", ghList(item("2", "fyi", 1)), nil)
	if ib.stale() {
		t.Fatal("still stale after a successful poll")
	}

	ib.update("gh", plugin.List{}, fmt.Errorf("dkt-gh: api 2, want 1: %w", plugin.ErrAPI))
	if !ib.stopped("gh") {
		t.Fatal("ErrAPI did not stop polling the plugin")
	}
}

func TestInboxPostponeAndRemoveHideItems(t *testing.T) {
	st := core.Replay(nil)
	ib := newInbox([]string{"gh"})
	ib.update("gh", ghList(item("1", "fyi", 1), item("2", "fyi", 2), item("3", "fyi", 3)), nil)
	ib.postpone("gh", "1")
	ib.remove("gh", "2")
	if got, want := outline(ib.rows(st)), "# FYI\n  gh:3\n"; got != want {
		t.Fatalf("rows =\n%s\nwant\n%s", got, want)
	}

	// The same full list again: postponed stays hidden for the session,
	// while a removed item that the plugin still lists comes back.
	ib.update("gh", ghList(item("1", "fyi", 1), item("2", "fyi", 2), item("3", "fyi", 3)), nil)
	if got, want := outline(ib.rows(st)), "# FYI\n  gh:2\n  gh:3\n"; got != want {
		t.Fatalf("after next poll, rows =\n%s\nwant\n%s", got, want)
	}
}

func TestInboxNamesPluginsWhenThereAreSeveral(t *testing.T) {
	ib := newInbox([]string{"gh", "wl"})
	ib.update("gh", ghList(item("1", "fyi", 1)), nil)
	ib.update("wl", plugin.List{Groups: []plugin.Group{{ID: "mine", Name: "Assigned"}}, Items: []plugin.Item{item("9", "mine", 1)}}, nil)
	want := "# gh · FYI\n  gh:1\n# wl · Assigned\n  wl:9\n"
	if got := outline(ib.rows(core.Replay(nil))); got != want {
		t.Fatalf("rows =\n%s\nwant\n%s", got, want)
	}
}

func TestInboxDropsPluginKeysThatClashWithDkts(t *testing.T) {
	ib := newInbox([]string{"gh"})
	l := ghList()
	l.Verbs = []plugin.Verb{{Key: "d", Label: "dismiss"}, {Key: "t", Label: "x"}, {Key: "j", Label: "x"}, {Key: "u", Label: "unsubscribe"}, {Key: "esc", Label: "x"}}
	l.GroupVerbs = []plugin.GroupVerb{{Key: "D", Verb: "d", Label: "dismiss group"}, {Key: "z", Verb: "d", Label: "x"}, {Key: "q", Verb: "d", Label: "x"}}
	ib.update("gh", l, nil)
	var got []string
	for _, v := range ib.verbs("gh") {
		got = append(got, v.Key)
	}
	for _, v := range ib.groupVerbs("gh") {
		got = append(got, v.Key)
	}
	if strings.Join(got, " ") != "d u D" {
		t.Fatalf("plugin keys kept = %v, want [d u D]", got)
	}
}
