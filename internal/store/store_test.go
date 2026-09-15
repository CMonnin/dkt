package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/CMonnin/dwkt/internal/core"
)

func TestAppendAndReadAllAcrossHosts(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)
	a, _ := core.CreateTask(core.Replay(nil), core.NewTask{Title: "from a"}, now)
	b, _ := core.CreateTask(core.Replay(nil), core.NewTask{Title: "from b"}, now)

	if err := Append(LogPath(dir, "a", "1111"), []core.Op{a}); err != nil {
		t.Fatal(err)
	}
	if err := Append(LogPath(dir, "b", "2222"), []core.Op{b}); err != nil {
		t.Fatal(err)
	}
	ops, warns, err := ReadAll(dir)
	if err != nil || len(warns) != 0 || len(ops) != 2 {
		t.Fatalf("ops=%d warns=%v err=%v", len(ops), warns, err)
	}
	if s := core.Replay(ops); len(s.Tasks) != 2 {
		t.Fatalf("tasks = %d, want 2", len(s.Tasks))
	}
}

func TestTornLineIsSkippedAndNotGluedToNextAppend(t *testing.T) {
	dir := t.TempDir()
	path := LogPath(dir, "a", "1111")
	now := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)
	first, _ := core.CreateTask(core.Replay(nil), core.NewTask{Title: "first"}, now)
	second, _ := core.CreateTask(core.Replay(nil), core.NewTask{Title: "second"}, now)

	if err := Append(path, []core.Op{first}); err != nil {
		t.Fatal(err)
	}
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString(`{"id":"01TORN","ts":"2026-`)
	f.Close()
	if err := Append(path, []core.Op{second}); err != nil {
		t.Fatal(err)
	}

	ops, warns, err := ReadAll(dir)
	if err != nil || len(ops) != 2 || len(warns) != 1 {
		t.Fatalf("ops=%d warns=%v err=%v", len(ops), warns, err)
	}
}

func TestDirResolution(t *testing.T) {
	t.Setenv("DWKT_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg")
	if got := ConfigDir(); got != filepath.Join("/xdg", "dwkt") {
		t.Errorf("xdg: %s", got)
	}
	t.Setenv("DWKT_CONFIG_DIR", "/override")
	if got := ConfigDir(); got != "/override" {
		t.Errorf("override: %s", got)
	}
	t.Setenv("DWKT_DATA_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "/home/u")
	if got := DataDir(); got != "/home/u/.local/share/dwkt" {
		t.Errorf("fallback: %s", got)
	}
}

func TestConfigRoundTripKeepsDefaults(t *testing.T) {
	dir := t.TempDir()
	if cfg, exists, err := LoadConfig(dir); err != nil || exists || cfg.MeetingDay != "thu" {
		t.Fatalf("missing file: %+v %v %v", cfg, exists, err)
	}
	os.WriteFile(ConfigPath(dir), []byte("machine_id = \"abcd\"\n"), 0o644)
	cfg, exists, err := LoadConfig(dir)
	if err != nil || !exists || cfg.MachineID != "abcd" || cfg.MeetingTime != "15:00" {
		t.Fatalf("got %+v %v %v", cfg, exists, err)
	}
	cfg.MeetingDay = "fri"
	if err := SaveConfig(dir, cfg); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := LoadConfig(dir); got != cfg {
		t.Fatalf("round trip: %+v != %+v", got, cfg)
	}
}
