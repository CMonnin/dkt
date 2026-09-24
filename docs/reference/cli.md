# CLI reference

```
dkt [command] [arguments] [flags]
```

With no command, `dkt` opens the [TUI](tui.md).

Every command except `init`, `help`, `version` and `self-update` does these things in order:

1. It pulls from the remote. An offline pull fails silently.
2. It runs the command.
3. If the command wrote anything, it starts a background push and returns at once.

## Arguments and flags

Flags can go before, between or after positional arguments. `dkt block fix login -r "no VPN"` and `dkt block -r "no VPN" fix login` are the same.

Each short flag has a long form (`-p` and `--project`). Go's flag parser accepts one or two dashes (`-backlog` and `--backlog`).

### Task references

A `<ref>` identifies one task. Multi-word refs need no quotes.

dkt resolves a ref in this order:

1. **ID prefix.** If the ref is 4 to 26 characters of Crockford base32, dkt looks for task IDs that start with it. It searches all tasks. A match stops the search.
2. **Exact title.** It looks for a task with that title, ignoring case.
3. **Title words.** It looks for tasks whose titles contain every word of the ref, ignoring case.

Steps 2 and 3 search only the tasks that the command can act on:

| Commands | Tasks searched |
|---|---|
| `done`, `doing`, `block`, `drop`, `week`, `defer` | Open tasks |
| `reopen` | Closed tasks |
| `edit`, `tag` | All tasks |

If more than one task matches:

- On a terminal, dkt shows a numbered list and asks you to pick one.
- Otherwise, dkt exits with an error that lists the candidates.

Listings show the first 10 characters of each ID.

Project refs work the same way, but they match project names. By default, they search only projects that are not archived.

### Task status

| Status | Box | Open |
|---|---|---|
| `todo` | `[ ]` | yes |
| `doing` | `[~]` | yes |
| `blocked` | `[!]` | yes |
| `done` | `[x]` | no |
| `dropped` | `[-]` | no |

## Commands

### `init`

```
dkt init [git-url]
```

Sets up the config directory and the data repository. It is safe to run again.

| State before | Effect |
|---|---|
| No data repo, no URL | Creates a local-only git repo. |
| No data repo, URL given | Clones the URL. An empty remote is supported. |
| Local-only repo, URL given | Adds the URL as `origin`. |

On first run, `init` writes `config.toml` with a new `machine_id`. It then pulls, pushes and prints the config path, data path, log path and sync status.

### `add`

```
dkt add <title> [-p project] [-c work|personal] [-t tags] [-b]
```

Creates a task on this week's list.

| Flag | Meaning |
|---|---|
| `-p`, `--project` | Project ref. The project must exist. |
| `-c`, `--category` | `work` or `personal`. Default: `default_category`. A project task takes its project's category, and a different `-c` is an error. |
| `-t`, `--tags` | Tags, separated by commas or spaces. |
| `-b`, `--backlog` | Add to the backlog, not this week. |

Words in the title that start with `#` become tags and are removed from the title. `dkt add "rerun QC #adni"` creates the task "rerun QC" with the tag `adni`.

### `ls`

```
dkt ls [--backlog | --history] [-p project] [--tag tag]
```

| Mode | Shows | Grouped by |
|---|---|---|
| (default) | Open tasks on this week, plus tasks marked done since Monday 00:00 | Category (Work, Personal) |
| `--backlog` | Open tasks not on this week | Inbox, then projects by name |
| `--history` | Done and dropped tasks, newest first, with the date closed | None |

`--backlog` and `--history` cannot be used together. With `--history`, `-p` also matches archived projects.

In the default mode, leftover tasks show `[leftover]`. If there are leftovers and no filter is set, a count follows the list.

### `done`, `doing`, `drop`, `reopen`

```
dkt done <ref>
dkt doing <ref>
dkt drop <ref>
dkt reopen <ref>
```

Set the status to `done`, `doing`, `dropped` or `todo`. `reopen` accepts only closed tasks. The others accept only open tasks.

### `block`

```
dkt block <ref> [-r reason]
```

Sets the status to `blocked`. `-r`, `--reason` stores the first line of the reason. The reason shows in listings and in exports.

### `edit`

```
dkt edit <ref> [--title title] [-n note] [-p project] [-c category]
```

With one or more flags, sets those fields directly:

| Flag | Field |
|---|---|
| `--title` | Title. It cannot be empty. |
| `-n`, `--note` | Note. |
| `-p`, `--project` | Project ref. `-p ""` moves the task to the inbox. |
| `-c`, `--category` | Category. Only for inbox tasks. |

With no flags, opens the task in `$VISUAL`, `$EDITOR` or `vi`, in that order. See [The edit form](#the-edit-form).

### `tag`

```
dkt tag <ref> +add -remove ...
```

Arguments that start with `+` or `#` add a tag. Arguments that start with `-` remove a tag. All other arguments form the ref. You must give at least one tag change.

### `week`, `defer`

```
dkt week <ref>
dkt defer <ref>
```

`week` puts an open task on this week and sets the date it was added to now. This is also how you keep a leftover.

`defer` moves an open task to the backlog.

### `plan`

```
dkt plan
```

Lists leftovers from earlier weeks and the commands to keep, defer or drop each one. A leftover is an open task on the week list that was added before Monday 00:00.

### `export`

```
dkt export [--since time] [--until time] [--tag tag] [--out file]
```

Prints a Markdown report of **work** tasks. Personal tasks are never exported.

| Flag | Meaning |
|---|---|
| `--since` | Window start. Default: the previous meeting. |
| `--until` | Window end. Default: this week's meeting, or now if that meeting has not happened yet. |
| `--tag` | Only tasks with this tag. |
| `--out` | Write to this file, not to standard output. |

Time formats, all in local time unless the value has an offset:

| Format | Example | Meaning |
|---|---|---|
| `YYYY-MM-DD` | `2026-09-01` | 00:00 that day. As `--until`, 00:00 the next day. |
| `YYYY-MM-DDTHH:MM` | `2026-09-01T09:30` | That minute. |
| `YYYY-MM-DD HH:MM` | `"2026-09-01 09:30"` | That minute. |
| RFC 3339 | `2026-09-01T09:30:00+02:00` | That instant. |

Report structure:

```markdown
# Work update: 2026-09-17 15:00 → 2026-09-24 15:00

## Inbox

**Done**
- ...

## <Project name>

**In progress**
- ...

**Blocked**
- <title> — <reason>

**Next**
- ...
```

| Section | Tasks |
|---|---|
| Done | Status `done`, completed after `--since` and at or before `--until`. Oldest first. |
| In progress | Status `doing`. |
| Blocked | Status `blocked`. |
| Next | Status `todo` and on this week. |

Only **Done** uses the time window. The other sections show the current status. "Inbox" comes first, then projects by name. Empty sections and projects are left out. If nothing matches, the report says `_Nothing to report._`.

### `project`

```
dkt project add <name> [-c work|personal] [-d description]
dkt project ls [--all]
dkt project rename <ref> <new name>
dkt project describe <ref> <text>
dkt project archive <ref>
```

| Subcommand | Effect |
|---|---|
| `add` | Creates a project. Default category: `default_category`. Names must be unique among projects that are not archived, ignoring case. |
| `ls` | Lists projects with their category and number of open tasks. `--all` or `-a` includes archived projects. |
| `rename` | Renames a project. It takes exactly two arguments, so quote a ref or name that has spaces. |
| `describe` | Sets the description. The first argument is the ref; the rest is the text. |
| `archive` | Archives the project and drops each of its open tasks. |

dkt never creates a project for you. `add -p` with an unknown project is an error.

### `sync`

```
dkt sync
```

Pulls, pushes and prints the sync status: `synced`, `N unpushed` or `local only`. It exits with an error if the pull or the push fails.

### `self-update`

```
dkt self-update
```

Downloads the latest GitHub release of `CMonnin/dkt` for this OS and architecture and replaces the running binary. If the installed version is already the latest, it does nothing. It fails if no release exists.

### `help`, `version`

```
dkt help       # also -h, --help
dkt version    # also -v, --version
```

## The edit form

`dkt edit <ref>` with no flags and the TUI's `E` key open this form:

```
title: Fix the broken contact form
project: Website
category: work
status: doing
tags: bug
blocked_reason:
# project: empty = inbox. category is ignored for project tasks.
# status: todo|doing|blocked|done|dropped. Lines starting with # are ignored.
--- note below this line ---
Any number of lines of note.
```

- dkt changes only the fields you change.
- If you delete a line, the field stays as it was.
- Tags are separated by commas or spaces.
- Everything after the `---` line is the note.
- If the form has an error, dkt saves nothing.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success, or help was shown. |
| 1 | The command failed. |
| 2 | Unknown command. |

## See also

- [TUI reference](tui.md)
- [Configuration reference](configuration.md)
