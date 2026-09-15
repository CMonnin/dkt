package core

import (
	"errors"
	"testing"
)

func refFixture() *State {
	s := Replay([]Op{
		mkCreate("01A1", "01J8AAAA00000000000000000A", "2026-09-14T09:00:00Z", "rerun QC"),
		mkCreate("01A2", "01J8BBBB00000000000000000B", "2026-09-14T09:01:00Z", "rerun QC twice"),
		mkCreate("01A3", "01J8CCCC00000000000000000C", "2026-09-14T09:02:00Z", "rerun pipeline"),
		mkCreate("01A4", "01J8DDDD00000000000000000D", "2026-09-14T09:03:00Z", "email Bob"),
		mkSet("01A5", "01J8DDDD00000000000000000D", "2026-09-14T10:00:00Z", "status", "done"),
	})
	return s
}

func open(t *Task) bool { return t.Status.Open() }

func TestResolveTask(t *testing.T) {
	s := refFixture()
	tests := []struct {
		ref    string
		filter func(*Task) bool
		want   string // ID, "" = error expected
		errAs  any
	}{
		{"01j8aaaa", nil, "01J8AAAA00000000000000000A", nil},
		{"01J8", nil, "", new(*AmbiguousError)},
		{"Rerun QC", nil, "01J8AAAA00000000000000000A", nil}, // exact beats "rerun QC twice"
		{"pipe", nil, "01J8CCCC00000000000000000C", nil},
		{"rerun", nil, "", new(*AmbiguousError)},
		{"qc twice", nil, "01J8BBBB00000000000000000B", nil},
		{"email", nil, "01J8DDDD00000000000000000D", nil},
		{"email", open, "", new(*NotFoundError)},              // done task filtered out
		{"01J8DDDD", open, "01J8DDDD00000000000000000D", nil}, // ID prefix ignores filter
		{"nope", nil, "", new(*NotFoundError)},
	}
	for _, tt := range tests {
		got, err := ResolveTask(s, tt.ref, tt.filter)
		if tt.want != "" {
			if err != nil || got.ID != tt.want {
				t.Errorf("%q: got %v, %v; want %s", tt.ref, got, err, tt.want)
			}
			continue
		}
		switch target := tt.errAs.(type) {
		case **AmbiguousError:
			if !errors.As(err, target) {
				t.Errorf("%q: err = %v, want ambiguous", tt.ref, err)
			}
		case **NotFoundError:
			if !errors.As(err, target) {
				t.Errorf("%q: err = %v, want not found", tt.ref, err)
			}
		}
	}
}

func TestMatchProjectsSkipsArchivedByName(t *testing.T) {
	s := Replay(nil)
	now := at("2026-09-14T09:00:00Z")
	l := &log{t: t}
	s, a := l.must(CreateProject(s, "ADNI QC", Work, "", now))
	s = l.add(ArchiveProject(s, s.Projects[a], now.Add(1))...)
	s, b := l.must(CreateProject(s, "ADNI QC", Work, "", now.Add(2)))
	if m := MatchProjects(s, "adni", false); len(m) != 1 || m[0].ID != b {
		t.Fatalf("got %v, want only the live project", m)
	}
	if m := MatchProjects(s, "adni", true); len(m) != 2 {
		t.Fatalf("with archived: %d matches, want 2", len(m))
	}
}
