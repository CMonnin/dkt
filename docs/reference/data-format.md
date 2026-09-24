# Data format reference

The data directory is a git repository. Each machine writes its own file:

```
ops/<hostname>-<machine_id>.jsonl
```

Each line of the file is one **op**, as a JSON object. dkt only appends to this file. It never changes or deletes a line.

## Op fields

| Field | Type | Present in | Meaning |
|---|---|---|---|
| `id` | ULID | All ops | Unique op ID. |
| `ts` | RFC 3339, UTC | All ops | When the op was made. |
| `host` | string | All ops | Hostname of the machine that wrote it. |
| `type` | string | All ops | See [Op types](#op-types). |
| `target` | ULID | All ops | ID of the task or project. |
| `field` | string | `*.set` | Name of the field to set. |
| `value` | JSON | `*.set` | New value of the field. |
| `tag` | string | `tag.*` | Tag, lowercase, with no `#`. |
| `task` | object | `task.create` | Initial task fields. |
| `project` | object | `project.create` | Initial project fields. |

## Op types

| Type | Effect |
|---|---|
| `task.create` | Creates a task. |
| `task.set` | Sets one task field. |
| `tag.add` | Adds a tag to a task. |
| `tag.remove` | Removes a tag from a task. |
| `project.create` | Creates a project. |
| `project.set` | Sets one project field. |

### `task.create` object

| Key | Type |
|---|---|
| `title` | string |
| `category` | `"work"` or `"personal"` |
| `project` | project ID; absent for inbox tasks |
| `status` | always `"todo"` when dkt writes it |
| `note` | string; optional |
| `tags` | array of strings; optional |
| `on_week` | boolean |
| `week_added_at` | RFC 3339; present when `on_week` is true |

### `task.set` fields

| Field | Value |
|---|---|
| `title` | string |
| `category` | `"work"` or `"personal"` |
| `project` | project ID, or `""` for the inbox |
| `status` | `"todo"`, `"doing"`, `"blocked"`, `"done"` or `"dropped"` |
| `note` | string |
| `blocked_reason` | string, one line |
| `on_week` | boolean |
| `week_added_at` | RFC 3339 |
| `completed_at` | RFC 3339, or `null` |

### `project.create` object

| Key | Type |
|---|---|
| `name` | string |
| `category` | `"work"` or `"personal"` |
| `description` | string; optional |

### `project.set` fields

| Field | Value |
|---|---|
| `name` | string |
| `category` | `"work"` or `"personal"` |
| `description` | string |
| `archived` | boolean |

## Examples

```json
{"id":"01M3A3NMCW...","ts":"2026-09-24T10:23:01Z","host":"laptop","type":"task.create","target":"01M3A3NMCV...","task":{"title":"Fix the broken contact form","category":"work","project":"01M3A3NMB9...","status":"todo","tags":["bug"],"on_week":true,"week_added_at":"2026-09-24T10:23:01Z"}}
{"id":"01M3A3NMD2...","ts":"2026-09-24T10:24:12Z","host":"laptop","type":"task.set","target":"01M3A3NMCV...","field":"status","value":"doing"}
{"id":"01M3A3NMD9...","ts":"2026-09-24T10:25:40Z","host":"laptop","type":"tag.add","target":"01M3A3NMCV...","tag":"urgent"}
```

IDs are shortened here.

## Replay rules

On each start, dkt reads every `ops/*.jsonl` file and builds the state from the ops:

1. It sorts all ops by `ts`, then by `id`.
2. It applies each op once. It skips an op whose `id` it has already applied.
3. After all ops, it removes any task or project that has no `create` op.

A line that is not valid JSON, or has no `id`, is skipped with a warning.

These status changes also write other fields:

| Change | Also writes |
|---|---|
| To `done` or `dropped` | `completed_at` = now |
| To an open status, from a closed one | `completed_at` = `null` |
| To `blocked` | `blocked_reason` |
| From `blocked` to another status | `blocked_reason` = `""` |

For the reasons behind this format, see [How dkt stores and syncs data](../explanation/storage-and-sync.md).
