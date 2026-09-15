# dwkt (opus5 implementation)

A weekly to-do TUI/CLI. Go + Bubble Tea. The data is an append-only op log per host in a git repo, replayed into memory each launch. No database.

The user didn't name this implementation, so it lives in `~/dwkt-opus5`.

## Build, test, run

```sh
export PATH=$HOME/.local/go/bin:$PATH
go test ./...                       # core + store unit tests
go build -o bin/dwkt ./cmd/dwkt
scripts/e2e.sh                      # two simulated machines on a local bare remote

# manual use: always point at .dev, never the real ~/.config / ~/.local/share
export DWKT_CONFIG_DIR=$PWD/.dev/config DWKT_DATA_DIR=$PWD/.dev/data
git init --bare .dev/remote.git && bin/dwkt init $PWD/.dev/remote.git
bin/dwkt add "rerun QC #adni" && bin/dwkt      # no args opens the TUI
```

Releases: `.goreleaser.yaml` + `.github/workflows/release.yml` build a static linux/amd64 binary for each `v*` tag. They are configured but nothing has been published.

## Layout

| Package | Role |
|---|---|
| `internal/app` | the name `dwkt`, the env prefix, the GitHub repo and the version (rename here only) |
| `internal/core` | pure domain: ops, replay (LWW), commands → ops, week, leftovers, meeting window, export, ref resolution, `$EDITOR` form. All tested. |
| `internal/store` | XDG/env paths, `config.toml`, JSONL read/append, flock |
| `internal/gitsync` | git CLI wrapper: clone/init, commit, fetch+rebase, push, unpushed count |
| `internal/session` | ties the three together; every log write and git op runs under the lock |
| `internal/cli`, `internal/tui` | frontends |
| `internal/editor`, `internal/selfupdate` | small helpers |

## Spec ambiguities and the choices made

**Storage and sync**
- Log file: `ops/<hostname>-<machine_id>.jsonl`. `machine_id` is 8 hex chars in `config.toml`. Workstations that share an NFS config share that suffix, but their hostnames still keep the files apart.
- Op shape: each `task.set` / `project.set` op sets a single `field`/`value`. `task.create` carries the initial fields and tags. Ops for a task whose `create` hasn't arrived yet are ignored until it does.
- Replay applies ops sorted by (ts, ULID). This makes every scalar field last-write-wins, and tag add/remove behave as a set. Duplicate op IDs are applied only once.
- Pull = `git fetch` + `git rebase origin/main`, not merge. Each host only appends to its own file, so the rebase can't conflict. Every CLI command and the TUI launch pull first. If the pull fails offline, the error is silent.
- Background push: the CLI starts a detached `dwkt __push` child, and the TUI uses a goroutine. The "N unpushed" count is computed with `git rev-list origin/main..HEAD` rather than stored. With no remote, the status shows "local only".
- The flock is `.git/dwkt.lock`. Pull and push hold it, as the spec says. This means a TUI write can pause for a moment while a push is running (network timeout 30s, ssh ConnectTimeout 10s). A `.git/index.lock` is deleted as stale if it is over 1 minute old while we hold the flock.
- A torn line from a crash is skipped with a warning, and the next append starts on a new line.
- If `user.email` isn't set, commits use `dwkt@<host>`. The branch is always `main`. Cloning an empty remote, or one whose HEAD points at another branch, is handled.
- `dwkt init` with no URL creates a local-only repo. Running `dwkt init <url>` later adds the remote.

**Data model**
- `completed_at` is set on **done and dropped** (History sorts by it). Any change back to an open status clears it, so `reopen` does too.
- `blocked_reason` stores only the first line. It is cleared when a task leaves `blocked`.
- Tags are lowercased and stored without `#`.
- Project names must be unique among non-archived projects. `-p` with an unknown project is an error; projects are never auto-created. Passing `-c` that conflicts with the project's category is an error.
- Archiving writes `archived=true` and a `dropped` op for each open task at that moment.

**Views**
- Leftover = `on_week && status is open (todo/doing/blocked) && week_added_at < Monday 00:00`. Dropped tasks are not leftovers.
- The Week list shows open on-week tasks, plus tasks done since Monday, so a ticked row doesn't disappear. Older done tasks move to History.
- `plan` lists leftovers. Keep = `dwkt week <ref>` (it re-stamps `week_added_at`).

**Export**
- Window: before this week's meeting it is [previous meeting, now]; after it, [previous meeting, this meeting]. Done means `completed_at` in (since, until]. Dates are built from wall-clock fields, so DST weeks keep 15:00.
- Groups: "Inbox" (tasks with no project) first, then projects by name. Empty sections and empty projects are omitted, and an empty report says `_Nothing to report._`. There is a `# Work update: <since> → <until>` header.
- In progress and Blocked are taken by status alone. Next = `todo && on_week`.
- `--since/--until` accept `YYYY-MM-DD`, `YYYY-MM-DDTHH:MM`, or RFC3339 (local time). A bare `--until` date means the end of that day.

**CLI**
- Ref resolution: a ULID prefix (≥4 chars) is tried first, across all tasks. After that comes an exact case-insensitive title match, then titles containing all the words. Candidates: `done/doing/block/drop/week/defer` → open tasks, `reopen` → closed tasks, `edit/tag` → all tasks. If a ref is ambiguous, you get a numbered prompt on a TTY and an error listing candidates otherwise. Listings show 10-char ID prefixes.
- Multi-word refs need no quotes (`dwkt done rerun QC`), and flags can go anywhere.
- `block <ref> -r "reason"` gives the reason. `edit <ref>` with flags (`--title -n -p -c`) edits directly. Without flags it opens `$EDITOR` on a small form (title, project, category, status, tags, blocked_reason, note), the same form the TUI's `E` uses.
- `self-update` downloads `dwkt_<ver>_linux_amd64.tar.gz` from the latest GitHub release and swaps the binary atomically. It is untested because no release exists.

**TUI**
- On a leftover row in Week, `k` means **keep** (spec) rather than move up. Arrow-up always moves. `d` defers, `D` drops, and `esc` keeps all. The cursor starts on the first leftover.
- Extra keys: `i` in progress, `b` block (prompts for a reason; enter skips), `D` drop, `w` backlog→week (and week→backlog), `d` week→backlog, `c` toggles work/personal (inbox tasks and projects), `n` note, `r`/`e`/`A` project rename/describe/archive, `R` sync, `h/l` or `tab` switch tabs, `?` full help, `q` quit.
- In History, `x` reopens. `/` filters live (title words and `#tag`), and `esc` clears the search and the project filter.
- Adding: Week → on week; Backlog → backlog (in the filtered project if one is set); Projects → new project with `default_category`.
- There is no auto-refresh from other processes. `R` pulls, pushes and reloads, and every write reloads from disk.
