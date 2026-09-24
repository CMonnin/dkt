# Configuration reference

## Files and directories

| Item | Location |
|---|---|
| Config directory | `$DKT_CONFIG_DIR`, else `$XDG_CONFIG_HOME/dkt`, else `~/.config/dkt` |
| Config file | `<config directory>/config.toml` |
| Data directory (git repo) | `$DKT_DATA_DIR`, else `$XDG_DATA_HOME/dkt`, else `~/.local/share/dkt` |
| This machine's op log | `<data directory>/ops/<hostname>-<machine_id>.jsonl` |
| Lock file | `<data directory>/.git/dkt.lock` |

## `config.toml`

`dkt init` creates this file. dkt uses the default value for any key that is missing.

```toml
meeting_day = "thu"
meeting_time = "15:00"
default_category = "work"
machine_id = "b12ec4e2"
```

| Key | Default | Meaning |
|---|---|---|
| `meeting_day` | `"thu"` | Day of the weekly meeting. A day name or its first three letters, in any case: `"thu"`, `"Thursday"`. |
| `meeting_time` | `"15:00"` | Local time of the meeting, `HH:MM`, 24-hour. |
| `default_category` | `"work"` | Category for new inbox tasks and new projects: `"work"` or `"personal"`. |
| `machine_id` | (random) | 8 hex characters. It is set by `init`. Do not change it. |

The meeting day and time set the default [export window](cli.md#export).

## Environment variables

| Variable | Effect |
|---|---|
| `DKT_CONFIG_DIR` | Overrides the config directory. |
| `DKT_DATA_DIR` | Overrides the data directory. |
| `DKT_HOSTNAME` | Overrides the hostname in the log file name. The default is the short system hostname. Characters other than letters, digits, `-` and `_` become `_`. |
| `XDG_CONFIG_HOME`, `XDG_DATA_HOME` | Used when the `DKT_` overrides are not set. |
| `VISUAL`, `EDITOR` | The editor for the edit form, in that order. The default is `vi`. |
| `GIT_SSH_COMMAND` | If set, git uses it. If not set, dkt uses `ssh -o BatchMode=yes -o ConnectTimeout=10`. |

## Git

- The branch is always `main`. The remote is always `origin`.
- If git has no `user.email`, dkt commits as `dkt <dkt@<hostname>>`.
- dkt never asks for a password (`GIT_TERMINAL_PROMPT=0`, SSH `BatchMode`). Your SSH key or credential helper must work with no prompt.

| Operation | Timeout |
|---|---|
| Local git commands | 10 s |
| Fetch, rebase, push | 30 s |
| Clone | 60 s |
| Wait for the lock | 90 s |
