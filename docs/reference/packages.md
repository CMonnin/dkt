# Package reference

| Package | Role |
|---|---|
| `cmd/dkt` | Entry point. Calls `cli.Run`. |
| `internal/app` | The tool's identity: name, environment variable prefix, GitHub repository, version. |
| `internal/core` | The domain: ops, replay, command builders, week and leftovers, meeting window, export, ref resolution, the edit form. It does not touch the file system, git or the clock. It has unit tests. |
| `internal/store` | Directory paths, `config.toml`, reading and appending the op logs, `flock`. |
| `internal/gitsync` | A wrapper for the git CLI: clone, init, commit, fetch and rebase, push, unpushed count. It does no locking. |
| `internal/session` | Joins config, the op logs and git. Every log write and git operation runs under the lock. |
| `internal/cli` | The subcommands and the background push. |
| `internal/tui` | The Bubble Tea interface. |
| `internal/editor` | Opens text in `$VISUAL` or `$EDITOR` and reads it back. |
| `internal/selfupdate` | Replaces the binary with the latest GitHub release. |

## Dependencies between packages

```
cmd/dkt → cli
cli     → tui, session, core, editor, selfupdate
tui     → session, core, editor
session → store, gitsync, core
store   → core
```

All packages can use `app`.
