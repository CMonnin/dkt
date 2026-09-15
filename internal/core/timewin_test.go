package core

import (
	"testing"
	"time"
	_ "time/tzdata"
)

func toronto(t *testing.T) *time.Location {
	loc, err := time.LoadLocation("America/Toronto")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

var thu15 = Meeting{Day: time.Thursday, Hour: 15, Minute: 0}

func TestMeetingWindow(t *testing.T) {
	loc := toronto(t)
	d := func(y int, m time.Month, day, h int) time.Time { return time.Date(y, m, day, h, 0, 0, 0, loc) }

	tests := []struct {
		name             string
		now              time.Time
		wantSince, wantU time.Time
	}{
		{"before meeting ends now", d(2026, 9, 17, 14), d(2026, 9, 10, 15), d(2026, 9, 17, 14)},
		{"after meeting ends at meeting", d(2026, 9, 17, 16), d(2026, 9, 10, 15), d(2026, 9, 17, 15)},
		{"monday before meeting", d(2026, 9, 14, 9), d(2026, 9, 10, 15), d(2026, 9, 14, 9)},
		{"sunday after meeting", d(2026, 9, 20, 9), d(2026, 9, 10, 15), d(2026, 9, 17, 15)},
		{"DST ends (Nov 1)", d(2026, 11, 5, 16), d(2026, 10, 29, 15), d(2026, 11, 5, 15)},
		{"DST starts (Mar 8)", d(2026, 3, 12, 16), d(2026, 3, 5, 15), d(2026, 3, 12, 15)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			since, until := MeetingWindow(tt.now, loc, thu15)
			if !since.Equal(tt.wantSince) || !until.Equal(tt.wantU) {
				t.Fatalf("window = %v → %v, want %v → %v", since, until, tt.wantSince, tt.wantU)
			}
		})
	}

	// Across a DST change the window is 7 days ± 1h of wall time, still 15:00 both ends.
	since, until := MeetingWindow(d(2026, 11, 5, 16), loc, thu15)
	if got := until.Sub(since); got != 7*24*time.Hour+time.Hour {
		t.Errorf("fall-back window length = %v, want 169h", got)
	}
	if since.In(loc).Hour() != 15 || until.In(loc).Hour() != 15 {
		t.Errorf("wall clock drifted: %v → %v", since.In(loc), until.In(loc))
	}
}

func TestWeekStart(t *testing.T) {
	loc := toronto(t)
	tests := []struct{ now, want time.Time }{
		{time.Date(2026, 9, 20, 23, 0, 0, 0, loc), time.Date(2026, 9, 14, 0, 0, 0, 0, loc)}, // Sunday
		{time.Date(2026, 9, 14, 0, 0, 0, 0, loc), time.Date(2026, 9, 14, 0, 0, 0, 0, loc)},  // Monday 00:00
		{time.Date(2026, 11, 4, 12, 0, 0, 0, loc), time.Date(2026, 11, 2, 0, 0, 0, 0, loc)}, // after DST change
	}
	for _, tt := range tests {
		if got := WeekStart(tt.now, loc); !got.Equal(tt.want) {
			t.Errorf("WeekStart(%v) = %v, want %v", tt.now, got, tt.want)
		}
	}
}

func TestParseMeeting(t *testing.T) {
	m, err := ParseMeeting("Thursday", "15:30")
	if err != nil || m != (Meeting{time.Thursday, 15, 30}) {
		t.Fatalf("got %+v, %v", m, err)
	}
	if _, err := ParseMeeting("thx", "15:00"); err == nil {
		t.Error("bad day accepted")
	}
	if _, err := ParseMeeting("thu", "3pm"); err == nil {
		t.Error("bad time accepted")
	}
}

func TestLeftovers(t *testing.T) {
	loc := toronto(t)
	l := &log{t: t}
	sunday := time.Date(2026, 9, 13, 18, 0, 0, 0, loc)
	monday := time.Date(2026, 9, 14, 8, 0, 0, 0, loc)

	s, stale := l.must(CreateTask(Replay(nil), NewTask{Title: "stale"}, sunday))
	s, done := l.must(CreateTask(s, NewTask{Title: "done"}, sunday.Add(time.Minute)))
	s, backlog := l.must(CreateTask(s, NewTask{Title: "backlog", Backlog: true}, sunday.Add(2*time.Minute)))
	s, fresh := l.must(CreateTask(s, NewTask{Title: "fresh"}, monday))
	s = l.add(SetStatus(s.Tasks[done], StatusDone, "", sunday.Add(time.Hour))...)

	later := monday.Add(time.Hour)
	got := map[string]bool{}
	for _, tk := range s.Leftovers(later, loc) {
		got[tk.ID] = true
	}
	if !got[stale] || got[done] || got[backlog] || got[fresh] || len(got) != 1 {
		t.Fatalf("leftovers = %v, want only stale", got)
	}

	// Keeping restamps week_added_at, so it stops being a leftover.
	s = l.add(MoveToWeek(s.Tasks[stale], later)...)
	if n := len(s.Leftovers(later, loc)); n != 0 {
		t.Fatalf("after keep: %d leftovers", n)
	}
}
