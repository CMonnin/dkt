// Package store owns on-disk layout: config, op log files, and locking.
package store

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/CMonnin/dwkt/internal/app"
)

// ConfigDir: $DWKT_CONFIG_DIR → $XDG_CONFIG_HOME/dwkt → ~/.config/dwkt.
func ConfigDir() string {
	return resolveDir(app.Env("CONFIG_DIR"), "XDG_CONFIG_HOME", ".config")
}

// DataDir: $DWKT_DATA_DIR → $XDG_DATA_HOME/dwkt → ~/.local/share/dwkt.
func DataDir() string {
	return resolveDir(app.Env("DATA_DIR"), "XDG_DATA_HOME", filepath.Join(".local", "share"))
}

func resolveDir(override, xdg, homeFallback string) string {
	if v := os.Getenv(override); v != "" {
		return v
	}
	if v := os.Getenv(xdg); v != "" {
		return filepath.Join(v, app.Name)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, homeFallback, app.Name)
}

// Hostname: $DWKT_HOSTNAME, else the short system hostname, made filename-safe.
func Hostname() string {
	h := os.Getenv(app.Env("HOSTNAME"))
	if h == "" {
		h, _ = os.Hostname()
		h, _, _ = strings.Cut(h, ".")
	}
	h = strings.Map(func(r rune) rune {
		if r == '-' || r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			return r
		}
		return '_'
	}, h)
	if h == "" {
		return "host"
	}
	return h
}
