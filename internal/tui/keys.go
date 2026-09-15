package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up, Down, Top, Bottom, NextTab, PrevTab, Tabs key.Binding
	Add, Done, Doing, Block, Drop, Week, Defer    key.Binding
	Tags, Note, Editor, Search, Category          key.Binding
	Open, Rename, Describe, Archive               key.Binding
	Keep, KeepAll, Reopen, Sync, Esc, Help, Quit  key.Binding
}

func bind(help, desc string, keys ...string) key.Binding {
	return key.NewBinding(key.WithKeys(keys...), key.WithHelp(help, desc))
}

func newKeys() keyMap {
	return keyMap{
		Up:      bind("k/↑", "up", "k", "up"),
		Down:    bind("j/↓", "down", "j", "down"),
		Top:     bind("g", "top", "g", "home"),
		Bottom:  bind("G", "bottom", "G", "end"),
		NextTab: bind("tab/l", "next tab", "tab", "l", "right"),
		PrevTab: bind("S-tab/h", "prev tab", "shift+tab", "h", "left"),
		Tabs:    bind("1-4", "tab", "1", "2", "3", "4"),

		Add:   bind("a", "add", "a"),
		Done:  bind("x", "done", "x"),
		Doing: bind("i", "in progress", "i"),
		Block: bind("b", "block", "b"),
		Drop:  bind("D", "drop", "D"),
		Week:  bind("w", "to week", "w"),
		Defer: bind("d", "to backlog", "d"),

		Tags:     bind("t", "tags", "t"),
		Note:     bind("n", "note", "n"),
		Editor:   bind("E", "$EDITOR", "E"),
		Search:   bind("/", "search", "/"),
		Category: bind("c", "work/personal", "c"),

		Open:     bind("enter", "open backlog", "enter"),
		Rename:   bind("r", "rename", "r"),
		Describe: bind("e", "describe", "e"),
		Archive:  bind("A", "archive", "A"),

		// Help-only entries for keys handled by context.
		Keep:    bind("k", "keep leftover", "k"),
		KeepAll: bind("esc", "keep all", "esc"),
		Reopen:  bind("x", "reopen", "x"),

		Sync: bind("R", "sync", "R"),
		Esc:  bind("esc", "clear filter", "esc"),
		Help: bind("?", "help", "?"),
		Quit: bind("q", "quit", "q", "ctrl+c"),
	}
}
