# Plugin protocol reference

A plugin is a program called `dkt-<name>` on `PATH`. This page is the contract between dkt and plugins. It's stable: protocol version **1**.

A plugin can do two things:

- **Commands.** `dkt <name> …` runs the plugin with the remaining arguments, attached to the terminal. See [Plugins](cli.md#plugins) in the CLI reference.
- **Inbox.** dkt polls the plugin for items to triage, and runs it to act on them. The rest of this page is about the inbox.

## Environment

Every run, whether from a command or the inbox, gets dkt's environment plus:

| Variable | Value |
|---|---|
| `DKT_BIN` | Path of the running `dkt` |
| `DKT_CONFIG_DIR` | The resolved config directory |
| `DKT_DATA_DIR` | The resolved data directory |
| `DKT_PLUGIN_API` | `1` |

A plugin keeps its own files in `$DKT_CONFIG_DIR/plugins/<name>/`. It never writes in the data directory.

## Inbox runs

For inbox runs, the plugin:

- writes newline-delimited JSON to stdout: one object per line, each with a `type`;
- writes nothing to the terminal;
- exits 0 on success. On failure, it exits non-zero, and the last line it wrote to stderr is shown as the error;
- finishes within **30 seconds**. After that, dkt kills it and everything it started, and the run counts as failed.

### `dkt-<name> inbox [--cursor <c>]`

Lists items to triage. `--cursor` is the `cursor` from the last successful run. It's opaque to dkt. With no `--cursor`, send everything.

```jsonl
{"type":"hello","api":1,"plugin":"gh","stream":false}
{"type":"groups","groups":[{"id":"needs-you","name":"Needs you"},{"id":"yours","name":"Your stuff"},{"id":"fyi","name":"FYI"}]}
{"type":"verbs","verbs":[{"key":"d","label":"dismiss"},{"key":"u","label":"unsubscribe"}],"group_verbs":[{"key":"D","verb":"d","label":"dismiss group","confirm":true}]}
{"type":"item","id":"27411703286","key":"CoBrALab/RABIES#533","group":"needs-you","label":"review","ref":"RABIES#533","title":"fix motion correction","url":"https://github.com/CoBrALab/RABIES/pull/533","updated_at":"2026-11-27T11:02:00Z","task":{"title":"Review: fix motion correction (RABIES#533)","category":"work","tags":["gh"],"note":"https://github.com/CoBrALab/RABIES/pull/533","project":{"key":"CoBrALab/RABIES","name":"RABIES"}}}
{"type":"end","cursor":"\"50a7407338ba387e\"","poll_after_s":60}
```

When nothing has changed since `--cursor`:

```jsonl
{"type":"hello","api":1,"plugin":"gh","stream":false}
{"type":"unchanged","poll_after_s":60}
```

#### Messages

| `type` | Fields | Meaning |
|---|---|---|
| `hello` | `api`, `plugin`, `stream` | Must be the first line. `api` must be `1`. `stream` is always `false` for now. |
| `groups` | `groups`: `[{id, name}]` | The plugin's groups, in display order. |
| `verbs` | `verbs`: `[{key, label}]`, `group_verbs`: `[{key, verb, label, confirm}]` | Keys for the plugin's own actions on an item. A group verb runs `verb` on every item in the group, and asks first if `confirm` is true. dkt ignores any key that clashes with its own. |
| `item` | see [Item fields](#item-fields) | One item to triage. |
| `end` | `cursor`, `poll_after_s` | The list is complete. dkt replaces its list for this plugin and keeps `cursor` for the next run. |
| `unchanged` | `poll_after_s` | Nothing changed since `--cursor`. dkt keeps its previous list. |
| `error` | `message` | The run failed, with `message` as the reason. |

`poll_after_s` is how long dkt waits before polling again: 60 if missing, and never less than 30.

#### Item fields

| Field | Meaning |
|---|---|
| `id` | Required. What `act` and `linked` receive. |
| `key` | Required. The stable key dkt stores on a task it makes from the item, e.g. `owner/repo#123`. |
| `group` | ID of the item's group. |
| `label` | Short reason, shown in brackets, e.g. `review`. |
| `ref` | Short reference, e.g. `RABIES#533`. |
| `title` | The item's title. |
| `url` | Web page for the item. |
| `updated_at` | RFC 3339 time of the latest activity. |
| `task` | Suggested task: `title`, `category`, `tags`, `note`, and `project` as `{key, name}`. |
| `persistent` | Optional, default `false`. `true` for work the user owes (an issue assigned to them, say): the plugin lists it for as long as it's open at the source. See [Persistent items](#persistent-items). |

#### When a run fails

dkt counts an inbox run as failed, and keeps its previous list, if:

- the plugin exits non-zero, or takes longer than 30 seconds;
- it prints an `error` message;
- a line isn't valid JSON;
- the output ends without `end` or `unchanged`.

If the first line isn't `hello`, or `api` isn't `1`, dkt stops polling the plugin. Polling again won't help.

dkt skips an `item` with no `id` or `key`, or with a control character, tab or newline in either. They're identities, so dkt drops them rather than cleaning them. It also skips any message whose `type` it doesn't know.

dkt also cleans what it shows, because much of it is written by other people (an issue title, for example):

- It drops control characters, such as terminal escape sequences, from all text: labels, titles, tags, group and verb names, and error messages. In one-line fields, tabs and newlines become spaces. The suggested note keeps its tabs and newlines.
- It keeps `url` only if it's an `http` or `https` URL with a host. Otherwise the item has no link.

#### Persistent items

A persistent item can't be dismissed, because it would be listed again on the next run. So dkt:

- offers no plugin keys on it, unless it's shown under ↻ Updates because of new activity;
- leaves it out of group verbs;
- offers `h` to hide it, as well as `t`, `b`, `T`, `B`, `z` and `o`.

Its `updated_at` is the latest activity the user hasn't seen. With no such activity, use the time the item was created, so that edits don't count as new activity.

A persistent item with no notification behind it may have an `id` that `act` can't act on. `act` should then fail with a message saying so, and `linked` should succeed and do nothing.

### `dkt-<name> act <id> <verb>`

Runs one of the plugin's verbs on one item. Exit 0 means it worked, and dkt removes the item. Non-zero means it failed: dkt keeps the item and shows the last stderr line. For a group verb, dkt runs `act` once per item.

### `dkt-<name> linked <id> <task-id>`

dkt has just linked the item to a task. The plugin does whatever "handled" means for it; for GitHub, that's marking the notification done. If this fails, nothing is lost: on the next poll the item is still there and already linked, so dkt runs `linked` again.

## Hidden fields

A task made from an item gets hidden fields `x.<name>.item` (the item's `key`) and `x.<name>.linked_at`. Its project gets `x.<name>.project.<key>`. dkt writes these; plugins never do. See [Hidden fields](data-format.md#hidden-fields).
