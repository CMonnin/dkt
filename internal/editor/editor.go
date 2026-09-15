// Package editor round-trips text through $VISUAL/$EDITOR.
package editor

import (
	"os"
	"os/exec"
	"strings"

	"github.com/CMonnin/dkt/internal/app"
)

// Prepare writes content to a temp file and returns the editor command for
// it. Call Finish after the command exits to read the result back.
func Prepare(content string) (cmd *exec.Cmd, path string, err error) {
	f, err := os.CreateTemp("", app.Name+"-*.txt")
	if err != nil {
		return nil, "", err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, "", err
	}
	f.Close()
	ed := os.Getenv("VISUAL")
	if ed == "" {
		ed = os.Getenv("EDITOR")
	}
	if ed == "" {
		ed = "vi"
	}
	parts := strings.Fields(ed)
	return exec.Command(parts[0], append(parts[1:], f.Name())...), f.Name(), nil
}

func Finish(path string) (string, error) {
	defer os.Remove(path)
	b, err := os.ReadFile(path)
	return string(b), err
}

// Edit runs the editor attached to the current terminal.
func Edit(content string) (string, error) {
	cmd, path, err := Prepare(content)
	if err != nil {
		return "", err
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(path)
		return "", err
	}
	return Finish(path)
}
