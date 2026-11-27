package tui

import (
	"errors"
	"slices"

	"github.com/CMonnin/dkt/internal/core"
	"github.com/CMonnin/dkt/internal/plugin"
)

// entry is one inbox item as shown: its plugin, the item, and for an
// update, the task it is linked to.
type entry struct {
	plugin string
	item   plugin.Item
	task   *core.Task
}

// inbox holds what the inbox plugins last reported. It is kept in memory
// only, for the life of the TUI.
type inbox struct {
	plugins []string
	lists   map[string]plugin.List // last complete list per plugin
	failed  map[string]bool        // last poll failed
	halted  map[string]bool        // speaks another protocol version: don't poll
	later   map[string]bool        // postponed for this session, by itemKey
}

func itemKey(plugin, id string) string { return plugin + "\x00" + id }

func newInbox(plugins []string) *inbox {
	return &inbox{plugins: plugins, lists: map[string]plugin.List{}, failed: map[string]bool{}, halted: map[string]bool{}, later: map[string]bool{}}
}

// update records the result of polling one plugin. Unchanged and failed
// polls keep the last list.
func (ib *inbox) update(name string, l plugin.List, err error) {
	ib.failed[name] = err != nil
	switch {
	case errors.Is(err, plugin.ErrAPI):
		ib.halted[name] = true
	case err != nil, l.Unchanged:
	default:
		ib.lists[name] = l
	}
}

// postpone hides an item until the TUI is next opened.
func (ib *inbox) postpone(name, id string) { ib.later[itemKey(name, id)] = true }

// remove drops an item from the current list, e.g. after a plugin verb on
// it worked. If the plugin lists it again, it comes back.
func (ib *inbox) remove(name, id string) {
	l := ib.lists[name]
	l.Items = slices.DeleteFunc(slices.Clone(l.Items), func(it plugin.Item) bool { return it.ID == id })
	ib.lists[name] = l
}

// cursor is what to pass as --cursor on the next poll.
func (ib *inbox) cursor(name string) string { return ib.lists[name].Cursor }

// stale is true while any plugin's last poll failed.
func (ib *inbox) stale() bool {
	for _, f := range ib.failed {
		if f {
			return true
		}
	}
	return false
}

// stopped is true for a plugin that must not be polled again.
func (ib *inbox) stopped(name string) bool { return ib.halted[name] }

// rows is the inbox to show: ↻ Updates first, then each plugin's groups in
// the plugin's order. Within a group, newest first.
func (ib *inbox) rows(st *core.State) []row {
	var updates []row
	var rest []row
	for _, name := range ib.plugins {
		l := ib.lists[name]
		byGroup := map[string][]row{}
		for _, it := range l.Items {
			if ib.later[itemKey(name, it.ID)] {
				continue
			}
			switch state, t := st.ClassifyItem(name, it.Key, it.UpdatedAt); state {
			case core.ItemUpdated:
				updates = append(updates, row{inbox: &entry{plugin: name, item: it, task: t}})
			case core.ItemNew:
				byGroup[it.Group] = append(byGroup[it.Group], row{inbox: &entry{plugin: name, item: it}})
			}
		}
		header := func(group string) string {
			if len(ib.plugins) > 1 {
				return name + " · " + group
			}
			return group
		}
		var other []row
		for _, g := range l.Groups {
			rest = appendGroup(rest, header(g.Name), byGroup[g.ID])
			delete(byGroup, g.ID)
		}
		for _, rs := range byGroup { // groups the plugin didn't declare
			other = append(other, rs...)
		}
		rest = appendGroup(rest, header("Other"), other)
	}
	return append(appendGroup(nil, "↻ Updates", updates), rest...)
}

// appendGroup adds a header and the group's rows, newest first, if any.
func appendGroup(rows []row, header string, group []row) []row {
	if len(group) == 0 {
		return rows
	}
	slices.SortStableFunc(group, func(a, b row) int {
		return b.inbox.item.UpdatedAt.Compare(a.inbox.item.UpdatedAt)
	})
	return append(append(rows, row{header: header}), group...)
}

// relinks are the items already linked to a task with nothing new: the
// plugin should be told again that they're handled.
func (ib *inbox) relinks(st *core.State) []entry {
	var out []entry
	for _, name := range ib.plugins {
		for _, it := range ib.lists[name].Items {
			if state, t := st.ClassifyItem(name, it.Key, it.UpdatedAt); state == core.ItemLinked {
				out = append(out, entry{plugin: name, item: it, task: t})
			}
		}
	}
	return out
}

// reservedKeys are dkt's own keys in the inbox. Plugin verbs on them are
// ignored.
var reservedKeys = map[string]bool{
	"t": true, "b": true, "T": true, "B": true, "z": true, "o": true, "r": true,
	"j": true, "k": true, "g": true, "G": true, "h": true, "l": true,
	"up": true, "down": true, "left": true, "right": true, "home": true, "end": true,
	"tab": true, "shift+tab": true, "enter": true, "esc": true,
	"1": true, "2": true, "3": true, "4": true,
	"I": true, "R": true, "/": true, "?": true, "q": true, "ctrl+c": true,
}

// verbs are the plugin's keys for one item, without any of dkt's keys.
func (ib *inbox) verbs(name string) []plugin.Verb {
	return slices.DeleteFunc(slices.Clone(ib.lists[name].Verbs), func(v plugin.Verb) bool { return reservedKeys[v.Key] })
}

// groupVerbs are the plugin's keys for a whole group, without dkt's keys.
func (ib *inbox) groupVerbs(name string) []plugin.GroupVerb {
	return slices.DeleteFunc(slices.Clone(ib.lists[name].GroupVerbs), func(v plugin.GroupVerb) bool { return reservedKeys[v.Key] })
}
