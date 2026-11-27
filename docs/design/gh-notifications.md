# Design: GitHub notifications in dkt, through an inbox plugin

**Status:** agreed, not implemented
**Date:** 2026-11-27
**Repos:** [CMonnin/dkt](https://github.com/CMonnin/dkt) (dkt side) · `CMonnin/dkt-gh` (new; the plugin)
**Goal:** triage every unread GitHub notification from inside dkt's TUI. Each one becomes a task, is dismissed, or is postponed.

This design also defines dkt's **plugin contract**. GitHub is its first user, and Worklenz (proposed on the `docs/worklenz-plugin-design` branch) is meant to be the second. When that branch merges, its "Part A" section should point here instead of describing the contract a second time.

---

## 1. What the user sees

1. **On opening the TUI:** if there are leftovers, the leftovers step comes first, as it does today. Then, if there's anything to triage, dkt opens on the **inbox**. Once the inbox is empty, or you press `esc`, you're on the Week tab.
2. **While the TUI is open:** dkt polls in the background. New items only update a counter in the status bar, e.g. `📥 7`. It never takes over the screen mid-session. `i` opens the inbox.
3. **When polling fails** (offline, GitHub down, not logged in): the last list stays visible, the counter turns grey with a `?` (`📥 7?`), the reason is shown once as a flash message, and polling retries quietly.
4. **CLI commands never touch the network.** The inbox exists only in the TUI.

### 1.1 Inbox layout

Groups from top to bottom. Within a group, newest first.

```
📥 Inbox (26)
  ↻ Updates
    RABIES#520   new activity · task: Review: atlas registration   1h
  Needs you
    [review]   RABIES#533   fix motion correction            2h
    [assigned] CPPQ#12      export fails on empty session    1d
    [mention]  upptime#88   is this still needed?            3d
  Your stuff
    [author]   ODC#401      CI failed on your PR             5h
  FYI
    [subscribed] RABIES     v0.5.2 released                  1d
```

- **↻ Updates** is dkt's own group, always at the top, for every plugin: new activity on something that's already a task (§4.3).
- All other groups and their order come from the plugin. For GitHub they're configurable (§5.3).

### 1.2 Keys

| Key | Action | Handled by | Effect in dkt | Effect on GitHub |
|---|---|---|---|---|
| `t` | Task, this week | dkt | Creates a linked task on the week | Mark done (via `linked`) |
| `b` | Task, backlog | dkt | Same, in the backlog | Mark done (via `linked`) |
| `T` / `B` | Same, edit the title first | dkt | Title prompt, prefilled | Same |
| `z` | Postpone | dkt | Hidden until the TUI is next opened; kept in memory only | Nothing; stays unread |
| `o` | Open | dkt | Opens the item's URL in the browser | Nothing |
| `r` | Reopen the linked task | dkt | Reopens it; only on ↻ items whose task is closed | Nothing |
| `d` | Dismiss | plugin | Nothing | Mark done |
| `u` | Dismiss and unsubscribe | plugin | Nothing | Mark done and unsubscribe |
| `D` | Dismiss the whole group (asks to confirm) | plugin | Nothing | Mark done, for each item |

`t`, `b`, `z`, `o` and `r` belong to dkt, and every plugin gets them. `d`, `u` and `D` are **declared by the plugin** (§3.3). dkt draws them and passes the chosen key back without knowing what it means.

On ↻ items, `t`/`b` are not offered, so a second task can't be created for the same thing.

---

## 2. Decisions and why

| Decision | Reason |
|---|---|
| Triage **every** unread notification, not only actionable ones | Owner's choice: dkt is where the inbox gets cleared. |
| **GitHub's unread state is the source of truth.** dkt stores no triage state | Each fact has one owner, there's nothing to sync between machines, and a failed write-back just means the item shows up again. Anything read on github.com drops out of dkt too. |
| Postpone lasts **until the TUI is next opened**, in memory | Simplest version that's still useful. Nothing on disk, nothing in the op log. |
| Write-back is limited to **notification state** (mark done, unsubscribe) | Acting on issues or PRs themselves (closing, commenting) acts publicly under your name, and brings back two-way sync problems. `o` is one keystroke away. |
| Write-back is **mark done**, not mark read | Once triaged, an item is handled. Read-but-listed items only clutter the github.com inbox. |
| A **plugin** (`dkt-gh`), not code built into dkt | Owner's choice. It also makes dkt's plugin contract real before Worklenz needs it. |
| A **one-shot** plugin process, with a message format that can later be streamed | GitHub has no push channel for user notifications, so a long-lived process would gain nothing now. Worklenz *may* push over Socket.IO, so the same messages can later be sent by a plugin that stays running (§3.5). |
| dkt **runs the dkt-side verbs, the plugin runs its own** | Only dkt writes the op log, so nothing commits behind the TUI's back. dkt learns no GitHub terms. |
| Tasks are linked by the **subject** (`owner/repo#N`), not the thread ID | Stable, readable, and doesn't depend on how GitHub reuses thread IDs. The thread ID is the fallback when there's no subject (Dependabot alerts). |
| **Projects are auto-created** from the repo and linked by a hidden field | Fewest keystrokes. Renaming the project is safe. The "dkt never creates a project" rule only guards typed `-p` refs against typos, and still applies there. |
| Plugins are **enabled in `config.toml`** (`inbox = ["gh"]`) | dkt never runs programs it wasn't told to. The config is per machine. |
| Auth order: `GH_TOKEN`, `GITHUB_TOKEN`, then `gh auth token` | Same order as `gh`. Works with or without `gh` installed. The token never touches the data directory. |
| dkt stays in **Go**. `dkt-gh` is **Go**, in **its own repo** | Owner's choice. The standard library covers HTTP and JSON, and it uses the same toolchain and release setup as dkt. |
| `ls --json` and `add --print-id` are **not built yet** | Nothing uses them: dkt creates tasks itself. They wait for Worklenz. |

---

## 3. The plugin contract (dkt side)

### 3.1 Dispatch

In `cli.Run`, when `args[0]` isn't a built-in command, look up `dkt-<name>` with `exec.LookPath`. If it's found, run it with the remaining args, with stdin, stdout and stderr attached, and exit with its exit code. If it isn't found, keep the current `unknown command` error. `dkt help` lists the `dkt-*` programs found on `PATH`, by name only, without running them.

The environment passed to every plugin run (dispatch and inbox):

| Variable | Value |
|---|---|
| `DKT_BIN` | `os.Executable()` of the running dkt |
| `DKT_CONFIG_DIR` | The resolved config directory |
| `DKT_DATA_DIR` | The resolved data directory |
| `DKT_PLUGIN_API` | `1` |

A plugin keeps its own files in `$DKT_CONFIG_DIR/plugins/<name>/`. It never writes in the data directory.

### 3.2 Hidden fields (`x.<plugin>.<key>`)

A per-task and per-project key/value store that plugins own, so dkt core never learns plugin terms.

- **Ops:** `task.set` and `project.set` with `field = "x.<plugin>.<key>"` and a string value. An empty string deletes the key.
- **Model:** `Task.Ext` and `Project.Ext`, both `map[string]string`. In `applyTaskSet` and `applyProjectSet`, any field starting with `x.` goes into `Ext`.
- **Compatibility:** older dkt binaries already ignore unknown `*.set` fields, because the `switch` in `internal/core/state.go` has no `default`. Machines that haven't upgraded are unaffected.
- Normal listings and the edit form don't show them, so you can't break a link by accident.
- There is no CLI for them yet (`dkt ext …` waits for a need).

Fields this design uses (plugin `gh`):

| On | Field | Value |
|---|---|---|
| Task | `x.gh.item` | The item key, e.g. `CoBrALab/RABIES#533` |
| Task | `x.gh.linked_at` | RFC 3339 time the link was made |
| Project | `x.gh.project.<key>` | `"1"`, e.g. `x.gh.project.CoBrALab/RABIES`. There is one field per repo, so several repos can share a project later. |

In general, dkt writes `x.<plugin>.item`, `x.<plugin>.linked_at` and `x.<plugin>.project.<key>`, with `<plugin>` set to the plugin's name. dkt writes these fields; the plugin never does.

### 3.3 Inbox protocol

**Enabling:** `config.toml` gets `inbox = ["gh"]`, default `[]`. Only plugins listed there are polled.

**Format:** the plugin writes newline-delimited JSON messages to stdout, one object per line, each with a `type`. dkt captures stderr and uses its last line as the error text if the run fails. Plugins must never write to the terminal.

#### `dkt-<name> inbox [--cursor <c>]`

Lists items to triage. `--cursor` is whatever the last successful run returned. It's opaque to dkt, and a missing cursor means "send everything".

```jsonl
{"type":"hello","api":1,"plugin":"gh","stream":false}
{"type":"groups","groups":[{"id":"needs-you","name":"Needs you"},{"id":"yours","name":"Your stuff"},{"id":"fyi","name":"FYI"}]}
{"type":"verbs","verbs":[{"key":"d","label":"dismiss"},{"key":"u","label":"unsubscribe"}],"group_verbs":[{"key":"D","verb":"d","label":"dismiss group","confirm":true}]}
{"type":"item","id":"27411703286","key":"CoBrALab/RABIES#533","group":"needs-you","label":"review","ref":"RABIES#533","title":"fix motion correction","url":"https://github.com/CoBrALab/RABIES/pull/533","updated_at":"2026-11-27T11:02:00Z","task":{"title":"Review: fix motion correction (RABIES#533)","category":"work","tags":["gh"],"note":"https://github.com/CoBrALab/RABIES/pull/533","project":{"key":"CoBrALab/RABIES","name":"RABIES"}}}
{"type":"end","cursor":"\"50a7407338ba387e\"","poll_after_s":60}
```

Or, when nothing has changed since `--cursor`:

```jsonl
{"type":"hello","api":1,"plugin":"gh","stream":false}
{"type":"unchanged","poll_after_s":60}
```

| Message | Meaning |
|---|---|
| `hello` | First line. `api` must equal `DKT_PLUGIN_API`, or dkt shows an error and stops polling that plugin. `stream` is always `false` for now (§3.5). |
| `groups` | Group IDs and labels, in display order. |
| `verbs` | Plugin keys for single items. They must not clash with dkt's keys (`t b T B z o r`, navigation and `esc`); dkt ignores any that do. `group_verbs` apply a verb to every item in the cursor's group. |
| `item` | One item. `id` is what `act` and `linked` receive. `key` is the stable link key that dkt stores. `task` holds the plugin's suggested task fields, and `project` its suggested project. |
| `end` | The list is complete. dkt replaces its list for this plugin, stores `cursor` in memory, and polls again after `poll_after_s`, at least 30 s. |
| `unchanged` | Keep the previous list. Poll again after `poll_after_s`. |
| `error` | `{"type":"error","message":"not logged in"}`. The poll failed (§1, point 3). |

A non-zero exit, or no `end`/`unchanged`, counts as a failed poll. Each run has a 30 s timeout.

#### `dkt-<name> act <id> <verb>`

Runs a plugin verb on one item. Exit 0 means it worked, and dkt removes the item. Non-zero means it failed: dkt keeps the item and flashes the plugin's last stderr line. For group verbs, dkt runs `act` once per item.

#### `dkt-<name> linked <id> <task-id>`

dkt has just linked the item to a task. The plugin does whatever "handled" means for it (for GitHub, mark done). On failure, nothing is lost (§4.4).

### 3.4 What dkt does with an inbox list

This lives in `internal/core` as a pure function, so it can be unit-tested. For each item:

1. If it's postponed this session (by plugin and `id`), skip it.
2. If some task has `x.<plugin>.item == key`:
   - if `updated_at` is later than `x.<plugin>.linked_at`, show it in **↻ Updates**, labelled with the task's title;
   - otherwise, don't show it, and run `linked <id> <task-id>` again in the background (§4.4).
3. Otherwise, show it in its plugin group.

With several plugins enabled, each plugin's groups are shown under its name. ↻ Updates stays a single group at the top.

### 3.5 Later: stream mode

If a plugin's `hello` says `"stream": true`, dkt would keep the process running and accept further `item`, `end` and `unchanged` messages at any time, instead of re-running it. **Not built now.** It waits until Worklenz shows that it can push task assignments over Socket.IO. The message format above was chosen so this needs no changes to it.

---

## 4. dkt-side behaviour in detail

### 4.1 `t` / `b`: making a task

In a single `Commit` (one lock, one git commit):

1. **Project.** Find a non-archived project with `x.<plugin>.project.<key>`. If there isn't one, but a non-archived project with the suggested name exists (ignoring case), link that one by setting the field. Otherwise create the project (suggested name, the suggested task's category) and link it.
2. `task.create` with the suggested title, category, tags and note, in that project. `on_week` is true for `t` and false for `b`.
3. `task.set x.<plugin>.item = key` and `task.set x.<plugin>.linked_at = now`.

Then, in the background, run `linked <id> <task-id>` and remove the item from the list. `T`/`B` show the title prompt, prefilled, before step 1.

### 4.2 Default task fields from `dkt-gh`

| Field | Value |
|---|---|
| Title | A verb by reason, then the subject title, then the short ref: `Review: …` (review requested), `Reply: …` (mention, team mention), no verb otherwise. E.g. `Review: fix motion correction (RABIES#533)` |
| Category | `work` |
| Tags | `gh` |
| Note | The item's web URL |
| Project | `{key: "owner/repo", name: "<repo>"}` |

### 4.3 ↻ Updates

GitHub marks a thread unread again when there's new activity. If the item is linked to a task and the activity is newer than the link, it shows up in ↻ Updates. The keys there are `o`, `d`, `z`, and `r` if the task is closed. `d` marks it done on GitHub. The next real activity brings it back again.

### 4.4 Failures

| Case | Behaviour |
|---|---|
| Poll fails | Keep the last list, grey `?` counter, flash the reason once, retry after `poll_after_s` (default 60 s). |
| `act` fails | The item stays, with a flash message. Retry or postpone. |
| `linked` fails after the task was made | The task exists, and the notification is still unread. On the next poll the item is linked, and its `updated_at` is not later than `linked_at`, so dkt silently runs `linked` again (§3.4, step 2). No false ↻. |

---

## 5. The plugin: `dkt-gh` (separate repo, Go)

### 5.1 GitHub API

| Use | Call |
|---|---|
| List unread | `GET /notifications` (unread only by default), all pages, following `Link`. Send `If-None-Match: <cursor>` when given a cursor. `304` → `unchanged`. `poll_after_s` comes from `X-Poll-Interval`, and `cursor` from the first page's `ETag`. Use `ETag`, not `Last-Modified`: see §8. |
| Mark done | `DELETE /notifications/threads/{id}` |
| Unsubscribe | `DELETE /notifications/threads/{id}/subscription` |

- **Item key:** taken from `subject.url`, `…/repos/<owner>/<repo>/(issues|pulls|discussions)/<n>` → `owner/repo#n`. Releases → `owner/repo@release/<id>`. No URL (Dependabot alerts) → `thread/<id>`.
- **Web URL:** the API subject URL converted to `https://github.com/<owner>/<repo>/(issues|pull|discussions)/<n>`. When there's none, use the repo's page.
- **`label`:** a short form of `reason` (`review`, `assigned`, `mention`, `author`, `comment`, `subscribed`, `security`, …).

### 5.2 Auth

`GH_TOKEN`, then `GITHUB_TOKEN`, then the output of `gh auth token`. If none of them works, `inbox` prints an `error` message ("not logged in: run `gh auth login` or set GH_TOKEN"). Fine-grained personal access tokens can't read notifications, so it needs a classic token or `gh`'s OAuth token. A 10 s HTTP timeout per request.

### 5.3 Config: `$DKT_CONFIG_DIR/plugins/gh/config.toml`

Optional. If the file is missing, these defaults apply:

```toml
# Groups, top to bottom. Reasons not listed fall into the last group.
[[group]]
name    = "Needs you"
reasons = ["review_requested", "assign", "mention", "team_mention"]

[[group]]
name    = "Your stuff"
reasons = ["author", "comment", "ci_activity"]

[[group]]
name    = "FYI"
reasons = []
```

There is deliberately no auto-dismiss option. It would contradict "triage them all".

### 5.4 Commands

```
dkt gh inbox [--cursor C]     # protocol (§3.3); also useful by hand for debugging
dkt gh act <id> <verb>        # d = mark done, u = mark done + unsubscribe
dkt gh linked <id> <task-id>  # mark done
dkt gh self-update            # its own copy of dkt's selfupdate (internal/ can't be imported)
dkt gh version
```

It's released with goreleaser, the same way as dkt, and installed next to `dkt` on `PATH`.

---

## 6. Changes to dkt, file by file

| Where | Change |
|---|---|
| `internal/core` | `Ext` on `Task` and `Project`; `x.` fields in `applyTaskSet` and `applyProjectSet`; builders for the link ops (§4.1); the pure inbox-matching function (§3.4). Unit tests for all of them. |
| `internal/plugin` (new) | Find plugins, build their environment, run `inbox`/`act`/`linked` with timeouts, parse and validate messages. |
| `internal/cli` | Dispatch to `dkt-<name>` (§3.1); list plugins in `help`. |
| `internal/store` | Read `inbox` from `config.toml`. |
| `internal/tui` | Inbox mode, after the leftovers step; status-bar counter; polling with `tea.Tick`; `i` key; postponed set in memory; keys from §1.2. |
| `testdata/` | A fake plugin (a script that prints canned messages and records the `act`/`linked` calls it gets), so dkt's tests never need `dkt-gh` or the network. |
| Docs | New `reference/plugin-protocol.md` (§3, the stable contract); updates to `reference/cli.md` (dispatch), `reference/configuration.md` (`inbox`), `reference/data-format.md` (`x.` fields), `reference/tui.md` (inbox) and `reference/packages.md` (`internal/plugin`). |

---

## 7. Build order

1. **Check the unverified facts (§8)** with `curl` against your own account, using one notification you don't mind marking done.
2. **Hidden fields** in `core`, with tests and `data-format.md`.
3. **Dispatch** in `cli`.
4. **Protocol:** `reference/plugin-protocol.md`, `internal/plugin` and the fake plugin, with tests.
5. **TUI inbox**, built against the fake plugin.
6. **`dkt-gh`:** client, `inbox`, `act`, `linked`, config, auth, release.
7. Try it against your real inbox. Tune the default groups and title verbs.

---

## 8. Verified facts and open questions

Checked on 2026-11-27 with `curl` against the owner's account, marking one release notification done (`CoBrALab/RABIES` "Release 0.6.3").

| Question | Result |
|---|---|
| Does `If-Modified-Since` give `304` on `/notifications`? | **No.** It always returns `200`, and each call costs a rate-limit point. |
| Does `ETag` / `If-None-Match` give `304`? | **Yes**, and a `304` doesn't count against the rate limit. The plugin polls this way (§5.1). |
| Does `Last-Modified` change after a thread is marked done? | **No.** It stayed the same, so it can't be used as a cursor. *Unverified:* that the `ETag` changes after mark done. It looks like a hash of the response body, so it should. |
| What does `DELETE /notifications/threads/{id}` (mark done) do? | `204`. The thread leaves the unread list, but the thread still exists: `GET /notifications/threads/{id}` returns it with `unread: false`, and it still appears with `?all=true`. |
| Does a thread keep its ID when it comes back after mark done? | **Not tested.** A release never gets new activity. The thread surviving mark done makes reuse very likely. Links use the subject key anyway, so nothing depends on it. |
| (Worklenz, later) Does Worklenz's Socket.IO push "task assigned to you" events? | Open. Decides whether stream mode (§3.5) is ever built. |

## 9. Out of scope (for now)

- Acting on issues and PRs themselves (closing, commenting, approving).
- `dkt ls --json`, `dkt add --print-id`, a `dkt ext …` CLI.
- Stream mode for plugins.
- A notification hint in CLI output.
- Auto-dismiss by reason.
- Merging several repos into one project (`dkt project link`). The data model already allows it.
