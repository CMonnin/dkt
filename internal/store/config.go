package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	MeetingDay      string `toml:"meeting_day"`
	MeetingTime     string `toml:"meeting_time"`
	DefaultCategory string `toml:"default_category"`
	MachineID       string `toml:"machine_id"`
}

func DefaultConfig() Config {
	return Config{MeetingDay: "thu", MeetingTime: "15:00", DefaultCategory: "work"}
}

func ConfigPath(dir string) string { return filepath.Join(dir, "config.toml") }

// LoadConfig returns defaults overlaid with the file; exists is false if there is no file.
func LoadConfig(dir string) (cfg Config, exists bool, err error) {
	cfg = DefaultConfig()
	if _, err := toml.DecodeFile(ConfigPath(dir), &cfg); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, false, nil
		}
		return cfg, true, err
	}
	return cfg, true, nil
}

func SaveConfig(dir string, cfg Config) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".config-*.toml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := toml.NewEncoder(tmp).Encode(cfg); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), ConfigPath(dir))
}

// NewMachineID is the random suffix that makes a host's log file name unique.
func NewMachineID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
