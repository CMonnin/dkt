package plugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// fake puts testdata/dkt-fake first on PATH and makes it print stdout.
// It returns the file the fake appends its arguments to.
func fake(t *testing.T, stdout string) (log string) {
	t.Helper()
	td, err := filepath.Abs("../../testdata")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", td+string(os.PathListSeparator)+os.Getenv("PATH"))
	dir := t.TempDir()
	out := filepath.Join(dir, "stdout")
	if err := os.WriteFile(out, []byte(stdout), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_STDOUT", out)
	log = filepath.Join(dir, "log")
	t.Setenv("FAKE_LOG", log)
	return log
}

// The example from docs/design/gh-notifications.md, section 3.3.
const fullList = `{"type":"hello","api":1,"plugin":"gh","stream":false}
{"type":"groups","groups":[{"id":"needs-you","name":"Needs you"},{"id":"yours","name":"Your stuff"},{"id":"fyi","name":"FYI"}]}
{"type":"verbs","verbs":[{"key":"d","label":"dismiss"},{"key":"u","label":"unsubscribe"}],"group_verbs":[{"key":"D","verb":"d","label":"dismiss group","confirm":true}]}
{"type":"item","id":"27411703286","key":"CoBrALab/RABIES#533","group":"needs-you","label":"review","ref":"RABIES#533","title":"fix motion correction","url":"https://github.com/CoBrALab/RABIES/pull/533","updated_at":"2026-11-27T11:02:00Z","task":{"title":"Review: fix motion correction (RABIES#533)","category":"work","tags":["gh"],"note":"https://github.com/CoBrALab/RABIES/pull/533","project":{"key":"CoBrALab/RABIES","name":"RABIES"}}}
{"type":"end","cursor":"\"50a7407338ba387e\"","poll_after_s":60}
`

func TestInboxReadsFullList(t *testing.T) {
	fake(t, fullList)
	got, err := Inbox(context.Background(), "fake", "")
	if err != nil {
		t.Fatal(err)
	}
	want := List{
		Groups:     []Group{{"needs-you", "Needs you"}, {"yours", "Your stuff"}, {"fyi", "FYI"}},
		Verbs:      []Verb{{Key: "d", Label: "dismiss"}, {Key: "u", Label: "unsubscribe"}},
		GroupVerbs: []GroupVerb{{Key: "D", Verb: "d", Label: "dismiss group", Confirm: true}},
		Items: []Item{{
			ID: "27411703286", Key: "CoBrALab/RABIES#533", Group: "needs-you", Label: "review",
			Ref: "RABIES#533", Title: "fix motion correction",
			URL:       "https://github.com/CoBrALab/RABIES/pull/533",
			UpdatedAt: time.Date(2026, 11, 27, 11, 2, 0, 0, time.UTC),
			Task: SuggestedTask{
				Title: "Review: fix motion correction (RABIES#533)", Category: "work",
				Tags: []string{"gh"}, Note: "https://github.com/CoBrALab/RABIES/pull/533",
				Project: &SuggestedProject{Key: "CoBrALab/RABIES", Name: "RABIES"},
			},
		}},
		Cursor:    `"50a7407338ba387e"`,
		PollAfter: 60 * time.Second,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Inbox =\n%+v\nwant\n%+v", got, want)
	}
}

func readLog(t *testing.T, log string) string {
	t.Helper()
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("plugin did not run: %v", err)
	}
	return string(b)
}

func TestInboxPassesCursor(t *testing.T) {
	log := fake(t, fullList)
	if _, err := Inbox(context.Background(), "fake", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := Inbox(context.Background(), "fake", `"50a7 407"`); err != nil {
		t.Fatal(err)
	}
	if got, want := readLog(t, log), "inbox\ninbox --cursor \"50a7 407\"\n"; got != want {
		t.Fatalf("plugin calls =\n%q\nwant\n%q", got, want)
	}
}

func TestInboxUnchanged(t *testing.T) {
	fake(t, `{"type":"hello","api":1,"plugin":"fake","stream":false}
{"type":"unchanged","poll_after_s":90}
`)
	got, err := Inbox(context.Background(), "fake", "c1")
	if err != nil {
		t.Fatal(err)
	}
	if want := (List{Unchanged: true, PollAfter: 90 * time.Second}); !reflect.DeepEqual(got, want) {
		t.Fatalf("Inbox = %+v, want %+v", got, want)
	}
}

func TestInboxPollAfterBounds(t *testing.T) {
	for _, c := range []struct {
		last string
		want time.Duration
	}{
		{`{"type":"end","cursor":"c","poll_after_s":5}`, 30 * time.Second},
		{`{"type":"end","cursor":"c"}`, 60 * time.Second},
		{`{"type":"unchanged","poll_after_s":1}`, 30 * time.Second},
		{`{"type":"unchanged"}`, 60 * time.Second},
	} {
		fake(t, `{"type":"hello","api":1,"plugin":"fake","stream":false}`+"\n"+c.last+"\n")
		got, err := Inbox(context.Background(), "fake", "")
		if err != nil {
			t.Fatal(err)
		}
		if got.PollAfter != c.want {
			t.Errorf("%s: PollAfter = %v, want %v", c.last, got.PollAfter, c.want)
		}
	}
}

func TestInboxWithoutEndFails(t *testing.T) {
	fake(t, `{"type":"hello","api":1,"plugin":"fake","stream":false}
{"type":"groups","groups":[{"id":"g","name":"G"}]}
`)
	if _, err := Inbox(context.Background(), "fake", ""); err == nil {
		t.Fatal("Inbox succeeded on output with no end or unchanged; want an error")
	}
}

func TestInboxErrorMessage(t *testing.T) {
	fake(t, `{"type":"hello","api":1,"plugin":"fake","stream":false}
{"type":"error","message":"not logged in"}
`)
	_, err := Inbox(context.Background(), "fake", "")
	if err == nil || err.Error() != "dkt-fake: not logged in" {
		t.Fatalf("Inbox error = %v, want %q", err, "dkt-fake: not logged in")
	}
}

func TestInboxWrongAPIVersion(t *testing.T) {
	fake(t, `{"type":"hello","api":2,"plugin":"fake","stream":false}
{"type":"end","cursor":"c","poll_after_s":60}
`)
	if _, err := Inbox(context.Background(), "fake", ""); !errors.Is(err, ErrAPI) {
		t.Fatalf("Inbox error = %v, want ErrAPI", err)
	}
}

func TestInboxWithoutHelloFirst(t *testing.T) {
	fake(t, `{"type":"end","cursor":"c","poll_after_s":60}
{"type":"hello","api":1,"plugin":"fake","stream":false}
`)
	if _, err := Inbox(context.Background(), "fake", ""); !errors.Is(err, ErrAPI) {
		t.Fatalf("Inbox error = %v, want ErrAPI when hello is not the first line", err)
	}
}

func TestInboxNonZeroExitUsesLastStderrLine(t *testing.T) {
	fake(t, fullList) // a complete list doesn't save a failed run
	t.Setenv("FAKE_STDERR", "fetching notifications\nGitHub API: 502 Bad Gateway")
	t.Setenv("FAKE_EXIT", "1")
	_, err := Inbox(context.Background(), "fake", "")
	if want := "dkt-fake: GitHub API: 502 Bad Gateway"; err == nil || err.Error() != want {
		t.Fatalf("Inbox error = %v, want %q", err, want)
	}
}

func TestInboxKillsHungPlugin(t *testing.T) {
	fake(t, fullList)
	t.Setenv("FAKE_SLEEP", "5")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := Inbox(ctx, "fake", "")
	if want := "dkt-fake: timed out"; err == nil || err.Error() != want {
		t.Fatalf("Inbox error = %v, want %q", err, want)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Fatalf("Inbox took %v to give up on a hung plugin, want well under the 5 s it sleeps", took)
	}
}

func TestInboxBadJSONFails(t *testing.T) {
	fake(t, `{"type":"hello","api":1,"plugin":"fake","stream":false}
not json
{"type":"end","cursor":"c","poll_after_s":60}
`)
	_, err := Inbox(context.Background(), "fake", "")
	if err == nil || !strings.HasPrefix(err.Error(), "dkt-fake: line 2: ") {
		t.Fatalf("Inbox error = %v, want one starting %q", err, "dkt-fake: line 2: ")
	}
}

func TestInboxIgnoresUnknownMessageTypes(t *testing.T) {
	fake(t, `{"type":"hello","api":1,"plugin":"fake","stream":false}
{"type":"from-the-future","x":1}
{"type":"end","cursor":"c","poll_after_s":60}
`)
	got, err := Inbox(context.Background(), "fake", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Cursor != "c" {
		t.Fatalf("Cursor = %q, want %q", got.Cursor, "c")
	}
}

func TestInboxSkipsItemsWithoutIDOrKey(t *testing.T) {
	fake(t, `{"type":"hello","api":1,"plugin":"fake","stream":false}
{"type":"item","key":"a/b#1","title":"no id"}
{"type":"item","id":"2","title":"no key"}
{"type":"item","id":"3","key":"a/b#3","title":"fine"}
{"type":"end","cursor":"c","poll_after_s":60}
`)
	got, err := Inbox(context.Background(), "fake", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || got.Items[0].ID != "3" {
		t.Fatalf("Items = %+v, want only the item with id 3", got.Items)
	}
}

func TestInboxPassesPluginEnvironment(t *testing.T) {
	fake(t, fullList)
	envlog := filepath.Join(t.TempDir(), "env")
	t.Setenv("FAKE_ENVLOG", envlog)
	t.Setenv("DKT_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	t.Setenv("DKT_DATA_DIR", "/my/data")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Inbox(context.Background(), "fake", ""); err != nil {
		t.Fatal(err)
	}
	want := "DKT_BIN=" + exe + "\nDKT_CONFIG_DIR=/xdg/config/dkt\nDKT_DATA_DIR=/my/data\nDKT_PLUGIN_API=1\n"
	if got := readLog(t, envlog); got != want {
		t.Fatalf("plugin env =\n%s\nwant\n%s", got, want)
	}
}

func TestActRunsVerbOnItem(t *testing.T) {
	log := fake(t, "")
	if err := Act(context.Background(), "fake", "27411703286", "d"); err != nil {
		t.Fatal(err)
	}
	if got, want := readLog(t, log), "act 27411703286 d\n"; got != want {
		t.Fatalf("plugin calls = %q, want %q", got, want)
	}
}

func TestActFailureUsesLastStderrLine(t *testing.T) {
	fake(t, "")
	t.Setenv("FAKE_STDERR", "unsubscribe: 404 Not Found")
	t.Setenv("FAKE_EXIT", "1")
	err := Act(context.Background(), "fake", "1", "u")
	if want := "dkt-fake: unsubscribe: 404 Not Found"; err == nil || err.Error() != want {
		t.Fatalf("Act error = %v, want %q", err, want)
	}
}

func TestLinkedTellsPluginTheTask(t *testing.T) {
	log := fake(t, "")
	if err := Linked(context.Background(), "fake", "27411703286", "01M3A3NMCV"); err != nil {
		t.Fatal(err)
	}
	if got, want := readLog(t, log), "linked 27411703286 01M3A3NMCV\n"; got != want {
		t.Fatalf("plugin calls = %q, want %q", got, want)
	}
}

func TestLinkedFailureUsesLastStderrLine(t *testing.T) {
	fake(t, "")
	t.Setenv("FAKE_STDERR", "mark done: 401 Unauthorized")
	t.Setenv("FAKE_EXIT", "3")
	err := Linked(context.Background(), "fake", "1", "T1")
	if want := "dkt-fake: mark done: 401 Unauthorized"; err == nil || err.Error() != want {
		t.Fatalf("Linked error = %v, want %q", err, want)
	}
}

func TestInboxStripsControlCharacters(t *testing.T) {
	esc := `\u001b]0;pwned\u0007\u001b[2J\u009b31m`
	fake(t, `{"type":"hello","api":1,"plugin":"fake","stream":false}
{"type":"groups","groups":[{"id":"g","name":"G`+esc+`"}]}
{"type":"verbs","verbs":[{"key":"d","label":"dismiss`+esc+`"}],"group_verbs":[{"key":"D","verb":"d","label":"all`+esc+`"}]}
{"type":"item","id":"1","key":"a/b#1","group":"g","label":"rev`+esc+`","ref":"b#1`+esc+`","title":"fix`+esc+`\nit","task":{"title":"T`+esc+`","category":"work","tags":["gh`+esc+`"],"note":"line one`+esc+`\nline two\ttabbed","project":{"key":"a/b","name":"B`+esc+`"}}}
{"type":"end","cursor":"c","poll_after_s":60}
`)
	l, err := Inbox(context.Background(), "fake", "")
	if err != nil {
		t.Fatal(err)
	}
	it := l.Items[0]
	got := []string{l.Groups[0].Name, l.Verbs[0].Label, l.GroupVerbs[0].Label, it.Label, it.Ref, it.Title,
		it.Task.Title, it.Task.Tags[0], it.Task.Note, it.Task.Project.Name}
	want := []string{"G]0;pwned[2J31m", "dismiss]0;pwned[2J31m", "all]0;pwned[2J31m", "rev]0;pwned[2J31m", "b#1]0;pwned[2J31m",
		"fix]0;pwned[2J31m it", "T]0;pwned[2J31m", "gh]0;pwned[2J31m", "line one]0;pwned[2J31m\nline two\ttabbed", "B]0;pwned[2J31m"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("strings =\n%q\nwant\n%q", got, want)
	}
}

func TestInboxErrorTextHasNoControlCharacters(t *testing.T) {
	fake(t, `{"type":"hello","api":1,"plugin":"fake","stream":false}
{"type":"error","message":"bad\u001b[2J"}
`)
	if _, err := Inbox(context.Background(), "fake", ""); err == nil || err.Error() != "dkt-fake: bad[2J" {
		t.Fatalf("error = %q, want %q", err, "dkt-fake: bad[2J")
	}
	fake(t, "")
	t.Setenv("FAKE_STDERR", "boom\x1b[2J\x07")
	t.Setenv("FAKE_EXIT", "1")
	if err := Act(context.Background(), "fake", "1", "d"); err == nil || err.Error() != "dkt-fake: boom[2J" {
		t.Fatalf("act error = %q, want %q", err, "dkt-fake: boom[2J")
	}
}

func TestInboxKeepsOnlyWebURLs(t *testing.T) {
	urls := []string{"https://github.com/a/b/pull/1", "http://example.com/x", "file:///etc/passwd",
		"--help", "-https://x", "javascript:alert(1)", "https://", "ssh://host/x", "/relative"}
	var out strings.Builder
	out.WriteString(`{"type":"hello","api":1,"plugin":"fake","stream":false}` + "\n")
	for i, u := range urls {
		fmt.Fprintf(&out, `{"type":"item","id":"%d","key":"k%d","url":%q}`+"\n", i, i, u)
	}
	out.WriteString(`{"type":"end","cursor":"c"}` + "\n")
	fake(t, out.String())
	l, err := Inbox(context.Background(), "fake", "")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, it := range l.Items {
		got = append(got, it.URL)
	}
	want := []string{"https://github.com/a/b/pull/1", "http://example.com/x", "", "", "", "", "", "", ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("URLs =\n%q\nwant\n%q", got, want)
	}
}

func TestInboxReadsPersistent(t *testing.T) {
	fake(t, `{"type":"hello","api":1,"plugin":"fake","stream":false}
{"type":"item","id":"open:o/r#1","key":"o/r#1","persistent":true}
{"type":"item","id":"2","key":"o/r#2"}
{"type":"end","cursor":"c"}
`)
	l, err := Inbox(context.Background(), "fake", "")
	if err != nil {
		t.Fatal(err)
	}
	if !l.Items[0].Persistent || l.Items[1].Persistent {
		t.Fatalf("Persistent = %v, %v; want true, false", l.Items[0].Persistent, l.Items[1].Persistent)
	}
}
