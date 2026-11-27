package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakePlugin puts an executable dkt-<name> running body on a PATH of its own,
// and returns a file the body can write to through $OUT.
func fakePlugin(t *testing.T, name, body string) (out string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "dkt-"+name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	out = filepath.Join(t.TempDir(), "out")
	t.Setenv("OUT", out)
	return out
}

func readOut(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("plugin did not run: %v", err)
	}
	return string(b)
}

func TestRunDispatchesToPluginWithArgs(t *testing.T) {
	out := fakePlugin(t, "foo", `printf '%s\n' "$@" > "$OUT"`)
	if code := Run([]string{"foo", "a", "b c"}); code != 0 {
		t.Fatalf("Run = %d, want 0", code)
	}
	if got, want := readOut(t, out), "a\nb c\n"; got != want {
		t.Fatalf("plugin args = %q, want %q", got, want)
	}
}

func TestRunReturnsPluginExitCode(t *testing.T) {
	fakePlugin(t, "foo", `exit 7`)
	if code := Run([]string{"foo"}); code != 7 {
		t.Fatalf("Run = %d, want the plugin's exit code 7", code)
	}
}

func TestRunPassesPluginEnvironment(t *testing.T) {
	out := fakePlugin(t, "foo", `printf '%s\n' "$DKT_BIN" "$DKT_CONFIG_DIR" "$DKT_DATA_DIR" "$DKT_PLUGIN_API" > "$OUT"`)
	t.Setenv("DKT_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	t.Setenv("DKT_DATA_DIR", "/my/data")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if code := Run([]string{"foo"}); code != 0 {
		t.Fatalf("Run = %d, want 0", code)
	}
	want := exe + "\n/xdg/config/dkt\n/my/data\n1\n"
	if got := readOut(t, out); got != want {
		t.Fatalf("plugin env (BIN, CONFIG_DIR, DATA_DIR, PLUGIN_API) =\n%q\nwant\n%q", got, want)
	}
}

func TestRunPrefersBuiltinOverPlugin(t *testing.T) {
	out := fakePlugin(t, "version", `touch "$OUT"; exit 9`)
	if code := Run([]string{"version"}); code != 0 {
		t.Fatalf("Run = %d, want 0 from the built-in version command", code)
	}
	if _, err := os.Stat(out); err == nil {
		t.Fatalf("dkt-version ran, but the built-in command should have won")
	}
}

func TestRunUnknownCommandWithoutPlugin(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if code := Run([]string{"nosuch"}); code != 2 {
		t.Fatalf("Run = %d, want 2 for an unknown command", code)
	}
}

// captureStdout runs f and returns what it printed to os.Stdout.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()
	f()
	w.Close()
	b, _ := io.ReadAll(r)
	return string(b)
}

func TestHelpListsPluginsOnPath(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	write := func(dir, name string, mode os.FileMode) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\ntouch \"$OUT\"\n"), mode); err != nil {
			t.Fatal(err)
		}
	}
	write(a, "dkt-gh", 0o755)
	write(b, "dkt-foo", 0o755)
	write(b, "dkt-gh", 0o755)   // shadowed by the first on PATH
	write(b, "dkt-nope", 0o644) // not executable
	write(b, "other", 0o755)    // not a dkt plugin
	t.Setenv("PATH", a+string(os.PathListSeparator)+b)
	ran := filepath.Join(t.TempDir(), "ran")
	t.Setenv("OUT", ran)

	got := captureStdout(t, func() { Run([]string{"help"}) })
	if !strings.HasSuffix(got, "\nPlugins (dkt-<name> on PATH):\n  foo\n  gh\n") {
		t.Fatalf("help output does not end with the plugin list:\n%s", got)
	}
	if _, err := os.Stat(ran); err == nil {
		t.Fatalf("help ran a plugin; it should only list them")
	}
}

func TestHelpWithoutPluginsHasNoPluginSection(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if got := captureStdout(t, func() { Run([]string{"help"}) }); strings.Contains(got, "Plugins") {
		t.Fatalf("help shows a plugin section with no plugins installed:\n%s", got)
	}
}
