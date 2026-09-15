package core

import (
	"fmt"
	"strings"
	"time"
)

// WeekStart is Monday 00:00 local of the week containing now.
func WeekStart(now time.Time, loc *time.Location) time.Time {
	n := now.In(loc)
	off := (int(n.Weekday()) + 6) % 7 // Monday = 0
	return time.Date(n.Year(), n.Month(), n.Day()-off, 0, 0, 0, 0, loc)
}

type Meeting struct {
	Day          time.Weekday
	Hour, Minute int
}

var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

// ParseMeeting accepts day as a weekday name ("thu", "Thursday") and time as HH:MM.
func ParseMeeting(day, hhmm string) (Meeting, error) {
	d := strings.ToLower(strings.TrimSpace(day))
	if len(d) >= 3 {
		d = d[:3]
	}
	wd, ok := weekdays[d]
	if !ok {
		return Meeting{}, fmt.Errorf("bad meeting_day %q", day)
	}
	t, err := time.Parse("15:04", strings.TrimSpace(hhmm))
	if err != nil {
		return Meeting{}, fmt.Errorf("bad meeting_time %q (want HH:MM)", hhmm)
	}
	return Meeting{Day: wd, Hour: t.Hour(), Minute: t.Minute()}, nil
}

// MeetingWindow is previous meeting -> current meeting. The current
// meeting is this week's; if it hasn't happened yet the window ends now.
// Dates are built from wall-clock fields so DST shifts keep 15:00 at 15:00.
func MeetingWindow(now time.Time, loc *time.Location, m Meeting) (since, until time.Time) {
	ws := WeekStart(now, loc)
	off := (int(m.Day) + 6) % 7
	cur := time.Date(ws.Year(), ws.Month(), ws.Day()+off, m.Hour, m.Minute, 0, 0, loc)
	prev := time.Date(ws.Year(), ws.Month(), ws.Day()+off-7, m.Hour, m.Minute, 0, 0, loc)
	if now.Before(cur) {
		return prev, now
	}
	return prev, cur
}
