package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/CMonnin/dwkt/internal/core"
)

func OpsDir(dataDir string) string { return filepath.Join(dataDir, "ops") }

func LogPath(dataDir, host, machineID string) string {
	return filepath.Join(OpsDir(dataDir), host+"-"+machineID+".jsonl")
}

// ReadAll reads every host's log. Malformed lines (e.g. a torn write from
// another host mid-append) are skipped and reported as warnings.
func ReadAll(dataDir string) (ops []core.Op, warnings []string, err error) {
	files, err := filepath.Glob(filepath.Join(OpsDir(dataDir), "*.jsonl"))
	if err != nil {
		return nil, nil, err
	}
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			return nil, warnings, err
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 16*1024*1024)
		for n := 1; sc.Scan(); n++ {
			line := sc.Bytes()
			if len(line) == 0 {
				continue
			}
			var op core.Op
			if err := json.Unmarshal(line, &op); err != nil || op.ID == "" {
				warnings = append(warnings, fmt.Sprintf("%s:%d: skipped malformed op", filepath.Base(path), n))
				continue
			}
			ops = append(ops, op)
		}
		err = sc.Err()
		f.Close()
		if err != nil {
			return nil, warnings, fmt.Errorf("%s: %w", path, err)
		}
	}
	return ops, warnings, nil
}

// Append writes ops as JSON lines in a single write. Callers hold the lock.
func Append(path string, ops []core.Op) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	var buf []byte
	// A crash mid-append leaves a torn last line; start on a fresh line so
	// the new ops aren't glued onto it.
	if st, err := f.Stat(); err == nil && st.Size() > 0 {
		last := make([]byte, 1)
		if _, err := f.ReadAt(last, st.Size()-1); err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if last[0] != '\n' {
			buf = append(buf, '\n')
		}
	}
	for _, op := range ops {
		b, err := json.Marshal(op)
		if err != nil {
			return err
		}
		buf = append(append(buf, b...), '\n')
	}
	if _, err := f.Write(buf); err != nil {
		return err
	}
	return f.Sync()
}

// Lock takes an exclusive flock (NFSv4 maps it to a byte-range lock, so it
// holds across hosts sharing the directory). Returns the unlock func.
func Lock(path string, timeout time.Duration) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(timeout)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) || time.Now().After(deadline) {
			f.Close()
			return nil, fmt.Errorf("lock %s: %w", path, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
