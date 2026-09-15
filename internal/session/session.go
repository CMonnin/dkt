// Package session ties config, the op log, and git together. Every log
// write and git operation runs under one flock.
package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/CMonnin/dwkt/internal/app"
	"github.com/CMonnin/dwkt/internal/core"
	"github.com/CMonnin/dwkt/internal/gitsync"
	"github.com/CMonnin/dwkt/internal/store"
)

const lockTimeout = 90 * time.Second

var ErrNotInitialized = fmt.Errorf("not initialized; run `%s init [git-url]`", app.Name)

type Session struct {
	ConfigDir, DataDir, Host string
	Cfg                      store.Config
	Meeting                  core.Meeting
	Loc                      *time.Location
	Repo                     gitsync.Repo
	Warnings                 []string // from the last Load
}

func Open() (*Session, error) {
	cfgDir, dataDir := store.ConfigDir(), store.DataDir()
	cfg, exists, err := store.LoadConfig(cfgDir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", store.ConfigPath(cfgDir), err)
	}
	repo := gitsync.Repo{Dir: dataDir}
	if !exists || cfg.MachineID == "" || !repo.IsRepo() {
		return nil, ErrNotInitialized
	}
	m, err := core.ParseMeeting(cfg.MeetingDay, cfg.MeetingTime)
	if err != nil {
		return nil, err
	}
	return &Session{
		ConfigDir: cfgDir, DataDir: dataDir, Host: store.Hostname(),
		Cfg: cfg, Meeting: m, Loc: time.Local, Repo: repo,
	}, nil
}

// Init creates config (machine_id) and the data repo, cloning url if given.
// Safe to re-run.
func Init(url string) (*Session, error) {
	cfgDir, dataDir := store.ConfigDir(), store.DataDir()
	cfg, _, err := store.LoadConfig(cfgDir)
	if err != nil {
		return nil, err
	}
	if cfg.MachineID == "" {
		cfg.MachineID = store.NewMachineID()
		if err := store.SaveConfig(cfgDir, cfg); err != nil {
			return nil, err
		}
	}
	repo := gitsync.Repo{Dir: dataDir}
	switch {
	case !repo.IsRepo() && url != "":
		err = gitsync.Clone(url, dataDir)
	case !repo.IsRepo():
		err = gitsync.Init(dataDir)
	case url != "" && !repo.HasRemote():
		err = repo.AddRemote(url)
	}
	if err != nil {
		return nil, err
	}
	s, err := Open()
	if err != nil {
		return nil, err
	}
	err = s.withLock(func() error {
		if err := os.MkdirAll(filepath.Dir(s.LogPath()), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(s.LogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return err
		}
		f.Close()
		_, err = s.Repo.CommitAll("init " + s.Host)
		return err
	})
	return s, err
}

func (s *Session) LogPath() string { return store.LogPath(s.DataDir, s.Host, s.Cfg.MachineID) }

func (s *Session) DefaultCategory() core.Category {
	if c, err := core.ParseCategory(s.Cfg.DefaultCategory); err == nil {
		return c
	}
	return core.Work
}

func (s *Session) withLock(fn func() error) error {
	unlock, err := store.Lock(s.Repo.LockPath(), lockTimeout)
	if err != nil {
		return err
	}
	defer unlock()
	s.Repo.CleanStaleIndexLock()
	return fn()
}

func (s *Session) Load() (*core.State, error) {
	ops, warnings, err := store.ReadAll(s.DataDir)
	s.Warnings = warnings
	if err != nil {
		return nil, err
	}
	return core.Replay(ops), nil
}

// Commit appends ops to this host's log and commits. The ops are durable
// once appended, so a git failure is reported separately as gitErr.
func (s *Session) Commit(ops []core.Op, msg string) (gitErr, err error) {
	if len(ops) == 0 {
		return nil, nil
	}
	for i := range ops {
		ops[i].Host = s.Host
	}
	err = s.withLock(func() error {
		if err := store.Append(s.LogPath(), ops); err != nil {
			return err
		}
		_, gitErr = s.Repo.CommitAll(msg)
		return nil
	})
	return gitErr, err
}

// Pull commits anything left uncommitted by a crash, then rebases onto the remote.
func (s *Session) Pull() error {
	return ignoreNoRemote(s.withLock(func() error {
		if _, err := s.Repo.CommitAll("recover uncommitted ops"); err != nil {
			return err
		}
		return s.Repo.Pull()
	}))
}

func (s *Session) Push() error {
	return ignoreNoRemote(s.withLock(s.Repo.Push))
}

type SyncStatus struct {
	Remote   bool
	Unpushed int
}

func (s *Session) Status() SyncStatus {
	n, _ := s.Repo.Unpushed()
	return SyncStatus{Remote: s.Repo.HasRemote(), Unpushed: n}
}

func (st SyncStatus) String() string {
	switch {
	case !st.Remote:
		return "local only"
	case st.Unpushed == 0:
		return "synced"
	}
	return fmt.Sprintf("%d unpushed", st.Unpushed)
}

func ignoreNoRemote(err error) error {
	if errors.Is(err, gitsync.ErrNoRemote) {
		return nil
	}
	return err
}
