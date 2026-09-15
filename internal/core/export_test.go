package core

import (
	"testing"
	"time"
)

func exportFixture(t *testing.T) (*State, ExportOptions) {
	l := &log{t: t}
	since := at("2026-09-10T15:00:00Z")
	until := at("2026-09-17T15:00:00Z")
	c := since.Add(-48 * time.Hour) // creation time, before window
	step := func() time.Time { c = c.Add(time.Minute); return c }

	s, adni := l.must(CreateProject(Replay(nil), "ADNI QC", Work, "", step()))
	s, garden := l.must(CreateProject(s, "Garden", Personal, "", step()))
	s, alpha := l.must(CreateProject(s, "Alpha", Work, "", step()))
	_ = alpha // empty project: must not appear

	s, rerun := l.must(CreateTask(s, NewTask{Title: "rerun QC", Project: adni, Tags: []string{"adni"}}, step()))
	s, old := l.must(CreateTask(s, NewTask{Title: "old thing", Project: adni}, step()))
	s, report := l.must(CreateTask(s, NewTask{Title: "write report", Project: adni}, step()))
	s, scans := l.must(CreateTask(s, NewTask{Title: "fetch scans", Project: adni}, step()))
	s, _ = l.must(CreateTask(s, NewTask{Title: "review pipeline", Project: adni, Tags: []string{"adni"}}, step()))
	s, _ = l.must(CreateTask(s, NewTask{Title: "someday", Project: adni, Backlog: true}, step()))
	s, _ = l.must(CreateTask(s, NewTask{Title: "email Bob"}, step()))
	s, plant := l.must(CreateTask(s, NewTask{Title: "plant", Project: garden}, step()))
	s, _ = l.must(CreateTask(s, NewTask{Title: "groceries", Category: Personal}, step()))

	s = l.add(SetStatus(s.Tasks[old], StatusDone, "", step())...) // before window
	in := since.Add(24 * time.Hour)
	s = l.add(SetStatus(s.Tasks[rerun], StatusDone, "", in)...)
	s = l.add(SetStatus(s.Tasks[plant], StatusDone, "", in.Add(time.Minute))...)
	s = l.add(SetStatus(s.Tasks[report], StatusDoing, "", in.Add(2*time.Minute))...)
	s = l.add(SetStatus(s.Tasks[scans], StatusBlocked, "waiting on IT\nticket 123", in.Add(3*time.Minute))...)

	return s, ExportOptions{Since: since, Until: until, Loc: time.UTC}
}

func TestExport(t *testing.T) {
	s, opts := exportFixture(t)
	want := `# Work update: 2026-09-10 15:00 → 2026-09-17 15:00

## Inbox

**Next**
- email Bob

## ADNI QC

**Done**
- rerun QC

**In progress**
- write report

**Blocked**
- fetch scans — waiting on IT

**Next**
- review pipeline
`
	if got := Export(s, opts); got != want {
		t.Fatalf("export mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestExportTagFilter(t *testing.T) {
	s, opts := exportFixture(t)
	opts.Tag = "#adni"
	want := `# Work update: 2026-09-10 15:00 → 2026-09-17 15:00

_Tag: #adni_

## ADNI QC

**Done**
- rerun QC

**Next**
- review pipeline
`
	if got := Export(s, opts); got != want {
		t.Fatalf("export mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestExportEmpty(t *testing.T) {
	opts := ExportOptions{Since: at("2026-09-10T15:00:00Z"), Until: at("2026-09-17T15:00:00Z"), Loc: time.UTC}
	want := "# Work update: 2026-09-10 15:00 → 2026-09-17 15:00\n\n_Nothing to report._\n"
	if got := Export(Replay(nil), opts); got != want {
		t.Fatalf("got %q", got)
	}
}
