// Package gitsync wraps the git CLI for the data repo. It does no locking;
// the session layer serializes calls under flock.
package gitsync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/CMonnin/dkt/internal/app"
)

const (
	Branch     = "main"
	remoteRef  = "refs/remotes/origin/" + Branch
	quick      = 10 * time.Second
	network    = 30 * time.Second
	staleIndex = time.Minute
)

var ErrNoRemote = errors.New("no git remote configured")

type Repo struct{ Dir string }

func run(ctx context.Context, dir string, args ...string) (string, error) {
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if os.Getenv("GIT_SSH_COMMAND") == "" {
		cmd.Env = append(cmd.Env, "GIT_SSH_COMMAND=ssh -o BatchMode=yes -o ConnectTimeout=10")
	}
	out, err := cmd.CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		return s, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, s)
	}
	return s, nil
}

func (r Repo) git(timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return run(ctx, r.Dir, args...)
}

func Init(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	_, err := Repo{dir}.git(quick, "init", "-q", "-b", Branch)
	return err
}

func Clone(url, dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*network)
	defer cancel()
	if _, err := run(ctx, "", "clone", "-q", url, dir); err != nil {
		return err
	}
	return Repo{dir}.ensureBranch()
}

// ensureBranch makes HEAD point at main, checking out origin/main if the
// clone left HEAD unborn (empty remote, or remote HEAD naming another branch).
func (r Repo) ensureBranch() error {
	if r.hasRef("HEAD") {
		return nil
	}
	if r.hasRef(remoteRef) {
		_, err := r.git(quick, "checkout", "-q", "-B", Branch, remoteRef)
		return err
	}
	_, err := r.git(quick, "symbolic-ref", "HEAD", "refs/heads/"+Branch)
	return err
}

func (r Repo) hasRef(ref string) bool {
	_, err := r.git(quick, "rev-parse", "--verify", "-q", ref)
	return err == nil
}

func (r Repo) IsRepo() bool {
	_, err := os.Stat(filepath.Join(r.Dir, ".git"))
	return err == nil
}

func (r Repo) HasRemote() bool {
	out, err := r.git(quick, "remote")
	return err == nil && strings.Contains("\n"+out+"\n", "\norigin\n")
}

func (r Repo) AddRemote(url string) error {
	_, err := r.git(quick, "remote", "add", "origin", url)
	return err
}

// LockPath is the flock file; inside .git so it is never committed.
func (r Repo) LockPath() string { return filepath.Join(r.Dir, ".git", app.Name+".lock") }

// CleanStaleIndexLock removes a leftover index.lock. Only call while holding
// the flock: then no dkt git process can legitimately own it.
func (r Repo) CleanStaleIndexLock() {
	p := filepath.Join(r.Dir, ".git", "index.lock")
	if st, err := os.Stat(p); err == nil && time.Since(st.ModTime()) > staleIndex {
		_ = os.Remove(p)
	}
}

// CommitAll commits everything under the data dir; reports whether a commit was made.
func (r Repo) CommitAll(msg string) (bool, error) {
	if _, err := r.git(quick, "add", "-A"); err != nil {
		return false, err
	}
	if _, err := r.git(quick, "diff", "--cached", "--quiet"); err == nil {
		return false, nil
	}
	args := []string{"commit", "-q", "--no-verify", "-m", msg}
	if out, _ := r.git(quick, "config", "user.email"); out == "" {
		host, _ := os.Hostname()
		args = append([]string{"-c", "user.name=" + app.Name, "-c", "user.email=" + app.Name + "@" + host}, args...)
	}
	_, err := r.git(quick, args...)
	return err == nil, err
}

// Pull fetches and rebases local commits onto origin/main. Each host only
// ever appends to its own log file, so the rebase cannot conflict.
func (r Repo) Pull() error {
	if !r.HasRemote() {
		return ErrNoRemote
	}
	if _, err := r.git(network, "fetch", "-q", "origin"); err != nil {
		return err
	}
	if !r.hasRef(remoteRef) {
		return nil
	}
	if !r.hasRef("HEAD") {
		return r.ensureBranch()
	}
	if _, err := r.git(network, "rebase", "-q", remoteRef); err != nil {
		_, _ = r.git(quick, "rebase", "--abort")
		return err
	}
	return nil
}

func (r Repo) Push() error {
	if !r.HasRemote() {
		return ErrNoRemote
	}
	if !r.hasRef("HEAD") {
		return nil
	}
	_, err := r.git(network, "push", "-q", "origin", "HEAD:refs/heads/"+Branch)
	return err
}

// Unpushed counts local commits not on origin/main (as of the last fetch/push).
func (r Repo) Unpushed() (int, error) {
	if !r.hasRef("HEAD") {
		return 0, nil
	}
	spec := "HEAD"
	if r.hasRef(remoteRef) {
		spec = remoteRef + "..HEAD"
	}
	out, err := r.git(quick, "rev-list", "--count", spec)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(out)
}
