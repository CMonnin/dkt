package core

import "time"

// ItemState says what the inbox does with a plugin item.
type ItemState int

const (
	ItemNew     ItemState = iota // not linked to a task: show it in its plugin group
	ItemUpdated                  // linked, with activity since the link: show it in ↻ Updates
	ItemLinked                   // linked, nothing new: hide it, and tell the plugin again
)

// ClassifyItem finds the task linked to a plugin item by its key, and says
// whether the item is new, updated since the link, or already handled. If
// several tasks are linked to the item (two machines linked it before
// syncing), the most recently linked one counts.
func (s *State) ClassifyItem(plugin, key string, updatedAt time.Time) (ItemState, *Task) {
	var best *Task
	var bestAt time.Time
	for _, t := range s.Tasks {
		if t.Ext(plugin, "item") != key {
			continue
		}
		at, _ := time.Parse(time.RFC3339, t.Ext(plugin, "linked_at"))
		if best == nil || at.After(bestAt) || at.Equal(bestAt) && t.ID > best.ID {
			best, bestAt = t, at
		}
	}
	switch {
	case best == nil:
		return ItemNew, nil
	case updatedAt.After(bestAt):
		return ItemUpdated, best
	}
	return ItemLinked, best
}
