// Package cli implements the dwkt subcommands.
package cli

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mattn/go-isatty"

	"github.com/CMonnin/dwkt/internal/app"
	"github.com/CMonnin/dwkt/internal/core"
	"github.com/CMonnin/dwkt/internal/selfupdate"
	"github.com/CMonnin/dwkt/internal/session"
)

const pushCommand = "__push"

type ctx struct {
	s         *session.Session
	st        *core.State
	now       time.Time
	out       io.Writer
	committed bool
}

type handler func(c *ctx, args []string) error

var handlers = map[string]handler{
	"add":     cmdAdd,
	"ls":      cmdLs,
	"done":    statusCmd(core.StatusDone),
	"doing":   statusCmd(core.StatusDoing),
	"block":   statusCmd(core.StatusBlocked),
	"drop":    statusCmd(core.StatusDropped),
	"reopen":  statusCmd(core.StatusTodo),
	"edit":    cmdEdit,
	"tag":     cmdTag,
	"week":    cmdWeek,
	"defer":   cmdDefer,
	"project": cmdProject,
	"plan":    cmdPlan,
	"export":  cmdExport,
	"sync":    cmdSync,
}

func Run(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "-h", "--help", "help":
			fmt.Print(usage)
			return 0
		case "-v", "--version", "version":
			fmt.Println(app.Name, app.Version)
			return 0
		case "init":
			return exit(cmdInit(args[1:]))
		case "self-update":
			return exit(selfupdate.Run(os.Stdout))
		case pushCommand:
			if s, err := session.Open(); err == nil {
				_ = s.Push()
			}
			return 0
		}
	}

	var h handler
	if len(args) > 0 {
		var ok bool
		if h, ok = handlers[args[0]]; !ok {
			fmt.Fprintf(os.Stderr, "%s: unknown command %q\n\n%s", app.Name, args[0], usage)
			return 2
		}
	}

	s, err := session.Open()
	if err != nil {
		return exit(err)
	}
	_ = s.Pull() // offline is fine; the status shows unpushed commits

	if h == nil {
		return exit(runTUI(s))
	}
	st, err := s.Load()
	if err != nil {
		return exit(err)
	}
	for _, w := range s.Warnings {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
	c := &ctx{s: s, st: st, now: time.Now(), out: os.Stdout}
	err = h(c, args[1:])
	if c.committed {
		backgroundPush()
	}
	return exit(err)
}

func exit(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	fmt.Fprintf(os.Stderr, "%s: %v\n", app.Name, err)
	return 1
}

// backgroundPush detaches a child process so the command returns immediately.
func backgroundPush() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe, pushCommand)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if cmd.Start() == nil {
		_ = cmd.Process.Release()
	}
}

func (c *ctx) commit(ops []core.Op, msg string) error {
	gitErr, err := c.s.Commit(ops, msg)
	if err != nil {
		return err
	}
	if gitErr != nil {
		fmt.Fprintln(os.Stderr, "warning: saved, but git commit failed:", gitErr)
	}
	c.committed = len(ops) > 0
	return nil
}

func (c *ctx) resolveTask(ref string, filter func(*core.Task) bool) (*core.Task, error) {
	if strings.TrimSpace(ref) == "" {
		return nil, errors.New("missing task reference (ID prefix or title)")
	}
	m := core.MatchTasks(c.st, ref, filter)
	if len(m) == 1 {
		return m[0], nil
	}
	if len(m) == 0 || !interactive() {
		_, err := core.ResolveTask(c.st, ref, filter)
		return nil, err
	}
	labels := make([]string, len(m))
	for i, t := range m {
		labels[i] = core.ShortID(t.ID) + "  " + t.Title
	}
	i, err := pick(ref, labels)
	if err != nil {
		return nil, err
	}
	return m[i], nil
}

func (c *ctx) resolveProject(ref string, includeArchived bool) (*core.Project, error) {
	m := core.MatchProjects(c.st, ref, includeArchived)
	switch {
	case len(m) == 1:
		return m[0], nil
	case len(m) == 0:
		return nil, fmt.Errorf("no project matches %q (create it with `%s project add`)", ref, app.Name)
	case !interactive():
		e := &core.AmbiguousError{Ref: ref}
		for _, p := range m {
			e.Candidates = append(e.Candidates, core.ShortID(p.ID)+"  "+p.Name)
		}
		return nil, e
	}
	labels := make([]string, len(m))
	for i, p := range m {
		labels[i] = core.ShortID(p.ID) + "  " + p.Name
	}
	i, err := pick(ref, labels)
	if err != nil {
		return nil, err
	}
	return m[i], nil
}

func interactive() bool {
	return isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stderr.Fd())
}

func pick(ref string, labels []string) (int, error) {
	fmt.Fprintf(os.Stderr, "%q matches several:\n", ref)
	for i, l := range labels {
		fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, l)
	}
	fmt.Fprintf(os.Stderr, "pick [1-%d]: ", len(labels))
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || n < 1 || n > len(labels) {
		return 0, errors.New("cancelled")
	}
	return n - 1, nil
}

func newFlags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(app.Name+" "+name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	return fs
}

// parseFlags allows flags before, between, or after positional arguments.
func parseFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

func stringFlag(fs *flag.FlagSet, p *string, short, long, usage string) {
	fs.StringVar(p, short, "", usage)
	fs.StringVar(p, long, "", usage)
}

func boolFlag(fs *flag.FlagSet, p *bool, short, long, usage string) {
	fs.BoolVar(p, short, false, usage)
	if long != "" {
		fs.BoolVar(p, long, false, usage)
	}
}

func isSet(fs *flag.FlagSet, names ...string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) {
		for _, n := range names {
			if f.Name == n {
				set = true
			}
		}
	})
	return set
}

var usage = strings.ReplaceAll(`usage: dwkt [command]

With no command, opens the TUI.

  init [git-url]                         set up config and the data repo
  add "title #tag" [-p proj] [-c work|personal] [-t a,b] [-b]
                                         add to this week (-b: backlog)
  ls [--backlog|--history] [-p proj] [--tag t]
  done|doing|drop|reopen <ref>           ref = ID prefix or title words
  block <ref> [-r reason]
  edit <ref> [--title T] [-n note] [-p proj] [-c category]
                                         no flags: open $EDITOR
  tag <ref> +a -b                        add/remove tags
  week <ref> | defer <ref>               move to week / back to backlog
  project add <name> [-c category] [-d description]
  project ls [--all]
  project rename <ref> <new name>
  project describe <ref> <text>
  project archive <ref>                  also drops its open tasks
  plan                                   list last week's leftovers
  export [--since T] [--until T] [--tag t] [--out f.md]
  sync                                   pull and push now
  self-update                            install the latest release
`, "dwkt", app.Name)
