// Package plugin runs dkt-<name> inbox plugins and reads what they print.
// The protocol is described in docs/reference/plugin-protocol.md.
package plugin

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/CMonnin/dkt/internal/app"
	"github.com/CMonnin/dkt/internal/store"
)

// List is the result of one successful inbox run.
type List struct {
	Unchanged  bool // nothing changed since the cursor; keep the previous list
	Groups     []Group
	Verbs      []Verb
	GroupVerbs []GroupVerb
	Items      []Item
	Cursor     string
	PollAfter  time.Duration
}

type Group struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Verb struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type GroupVerb struct {
	Key     string `json:"key"`
	Verb    string `json:"verb"`
	Label   string `json:"label"`
	Confirm bool   `json:"confirm"`
}

type Item struct {
	ID        string        `json:"id"`
	Key       string        `json:"key"`
	Group     string        `json:"group"`
	Label     string        `json:"label"`
	Ref       string        `json:"ref"`
	Title     string        `json:"title"`
	URL       string        `json:"url"`
	UpdatedAt time.Time     `json:"updated_at"`
	Task      SuggestedTask `json:"task"`
}

// SuggestedTask holds the task fields a plugin suggests for an item.
type SuggestedTask struct {
	Title    string            `json:"title"`
	Category string            `json:"category"`
	Tags     []string          `json:"tags"`
	Note     string            `json:"note"`
	Project  *SuggestedProject `json:"project"`
}

type SuggestedProject struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// API is the protocol version this dkt speaks, passed as DKT_PLUGIN_API.
const API = 1

// ErrAPI means the plugin speaks another protocol version. Polling it again
// won't help.
var ErrAPI = errors.New("plugin speaks a different protocol version")

// message is any one line a plugin prints.
type message struct {
	Type       string      `json:"type"`
	Groups     []Group     `json:"groups"`
	Verbs      []Verb      `json:"verbs"`
	GroupVerbs []GroupVerb `json:"group_verbs"`
	Item
	Cursor    string `json:"cursor"`
	PollAfter int    `json:"poll_after_s"`
	Message   string `json:"message"`
	API       int    `json:"api"`
}

const (
	minPollAfter     = 30 * time.Second
	defaultPollAfter = 60 * time.Second
)

// pollAfter turns poll_after_s into a wait: 60 s if missing, never under 30 s.
func pollAfter(seconds int) time.Duration {
	if seconds <= 0 {
		return defaultPollAfter
	}
	return max(time.Duration(seconds)*time.Second, minPollAfter)
}

// Inbox runs `dkt-<name> inbox` and reads the list it prints.
func Inbox(ctx context.Context, name, cursor string) (List, error) {
	args := []string{"inbox"}
	if cursor != "" {
		args = append(args, "--cursor", cursor)
	}
	out, err := run(ctx, name, args...)
	if err != nil {
		return List{}, err
	}
	var l List
	done := false
	sc := bufio.NewScanner(bytes.NewReader(out))
	for n := 1; sc.Scan(); n++ {
		var m message
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			return List{}, fmt.Errorf("%s-%s: line %d: %w", app.Name, name, n, err)
		}
		if n == 1 && m.Type != "hello" {
			return List{}, fmt.Errorf("%s-%s: first line is not hello: %w", app.Name, name, ErrAPI)
		}
		switch m.Type {
		case "hello":
			if m.API != API {
				return List{}, fmt.Errorf("%s-%s: api %d, want %d: %w", app.Name, name, m.API, API, ErrAPI)
			}
		case "groups":
			l.Groups = m.Groups
		case "verbs":
			l.Verbs, l.GroupVerbs = m.Verbs, m.GroupVerbs
		case "item":
			if m.ID != "" && m.Key != "" { // without them it can't be acted on or linked
				l.Items = append(l.Items, m.Item)
			}
		case "end":
			l.Cursor = m.Cursor
			l.PollAfter = pollAfter(m.PollAfter)
			done = true
		case "unchanged":
			l.Unchanged = true
			l.PollAfter = pollAfter(m.PollAfter)
			done = true
		case "error":
			return List{}, fmt.Errorf("%s-%s: %s", app.Name, name, m.Message)
		}
	}
	if !done {
		return List{}, fmt.Errorf("%s-%s inbox: output ended without end or unchanged", app.Name, name)
	}
	return l, nil
}

// Act runs `dkt-<name> act <id> <verb>`: a plugin verb on one item.
func Act(ctx context.Context, name, id, verb string) error {
	_, err := run(ctx, name, "act", id, verb)
	return err
}

// Linked runs `dkt-<name> linked <id> <task-id>`: dkt has just linked the
// item to a task, and the plugin does whatever "handled" means for it.
func Linked(ctx context.Context, name, id, taskID string) error {
	_, err := run(ctx, name, "linked", id, taskID)
	return err
}

// Environ is the environment every plugin run gets: dkt's own, plus
// DKT_BIN, DKT_CONFIG_DIR, DKT_DATA_DIR and DKT_PLUGIN_API.
func Environ() []string {
	exe, _ := os.Executable()
	return append(os.Environ(),
		app.Env("BIN")+"="+exe,
		app.Env("CONFIG_DIR")+"="+store.ConfigDir(),
		app.Env("DATA_DIR")+"="+store.DataDir(),
		app.Env("PLUGIN_API")+"="+strconv.Itoa(API),
	)
}

// Timeout is the longest a single plugin run may take.
const Timeout = 30 * time.Second

// run runs dkt-<name> with args and returns its stdout. If it fails, the
// error is the last line it wrote to stderr. On timeout it kills the plugin
// and everything the plugin started.
func run(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, app.Name+"-"+name, args...)
	cmd.Stderr = &stderr
	cmd.Env = Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = time.Second // in case something escaped the process group
	out, err := cmd.Output()
	if err != nil {
		msg := lastLine(stderr.String())
		if ctx.Err() != nil {
			msg = "timed out"
		} else if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("%s-%s: %s", app.Name, name, msg)
	}
	return out, nil
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	return strings.TrimSpace(s[strings.LastIndexByte(s, '\n')+1:])
}
