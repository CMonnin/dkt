// Package app holds the program's identity. Renaming the tool means
// editing this file only.
package app

const (
	// Name is the binary name and the config/data directory name.
	Name = "dkt"
	// EnvPrefix prefixes every environment variable the tool reads.
	EnvPrefix = "DKT"
	// GitHubRepo is the code repository used by self-update.
	GitHubRepo = "CMonnin/" + Name
)

// Version is set at build time via -ldflags "-X .../internal/app.Version=v1.2.3".
var Version = "dev"

// Env returns the full name of a tool-specific environment variable.
func Env(suffix string) string { return EnvPrefix + "_" + suffix }
