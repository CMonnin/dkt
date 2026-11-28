# Design: open GitHub work in the dkt inbox, and hiding items

**Status:** built on `feat/open-work` in both repos, not merged
**Date:** 2026-11-27
**Builds on:** [GitHub notifications in dkt](gh-notifications.md) (the plugin contract, `dkt-gh`, the TUI inbox)
**Repos:** [CMonnin/dkt](https://github.com/CMonnin/dkt) · [CMonnin/dkt-gh](https://github.com/CMonnin/dkt-gh)
**Goal:** the inbox is **complete**. Every open issue or PR you owe on GitHub is either a dkt task, hidden on purpose, or in the inbox.

---

## 1. Why

The inbox today lists *notifications*: events. Open work is *state*, and the two don't line up. On 2026-11-27 the owner had 45 open issues and PRs that were assigned to them or that they authored, and only 1 of them had an unread notification. Anything whose notification was read on github.com, or dismissed with `d`, has dropped out of dkt even though it's still open.

---

## 2. What the user sees

1. **Owed items join the inbox.** Open items you owe (§3) show up in their groups next to notifications, until each becomes a task or is hidden. An item that is both owed and has an unread notification shows once.
2. **They can't be dismissed.** `d`, `u` and other plugin keys aren't offered on an owed item, because it would come straight back on the next poll. The keys are `t` `b` `T` `B` (make a task), `h` (hide), `z` (postpone for the session) and `o` (open). To get rid of an owed item for good, close it or unassign yourself on GitHub.
3. **`h` hides an item.** It moves to a new **Hidden** tab and stops showing in the inbox. If new activity arrives on it (a notification), it shows up under ↻ Updates, marked *hidden*, so you notice when someone picks up that old issue.
4. **A closed task doesn't hide open work.** If you close the task (done or dropped) but the item is still open and still yours on GitHub, it shows under ↻ Updates as *task closed, still open on GitHub*, until you reopen the task (`r`), hide it (`h`), or GitHub agrees.

### 2.1 Where an owed item shows

| Linked task | Item | Shown |
|---|---|---|
| none | owed | Inbox, in its group |
| open (todo, doing, blocked) | owed | Not in the inbox. It's on Week or Backlog. ↻ on new activity, as today. |
| closed (done or dropped), not hidden | owed | ↻ *task closed, still open on GitHub* |
| hidden | owed | Hidden tab. ↻ *hidden · new activity* if a notification is newer than the hide. |
| any | not owed any more (closed, unassigned) | Not in the inbox. A hidden task stays on the Hidden tab, marked *no longer open* (§5.3). |

### 2.2 Keys

| Where | Keys |
|---|---|
| Owed item, no task | `t` `b` `T` `B` `h` `z` `o` |
| ↻ new activity (task open) | as today: plugin keys, `z`, `o` |
| ↻ *task closed, still open* | `r` reopen the task, `h` hide, `z`, `o` |
| ↻ *hidden · new activity* | plugin keys (e.g. `d` marks the notification done; the item stays hidden), `h` hide again from now, `r` make it a real task, `z`, `o` |
| Hidden tab (`5`) | `r` make it a real task (todo, backlog), `u` un-hide back to the inbox, `o` open |

`h` is only offered on owed items. A plain notification is dismissed with `d`.

---

## 3. What counts as owed

All four lists, by the owner's choice. Each can be turned off in `dkt-gh`'s config (§4.4).

| Name | GitHub search | Owner's count on 2026-11-27 | Reason it maps to |
|---|---|---|---|
| `assigned` | `is:open archived:false assignee:@me` | 16 issues | `assign` |
| `review_requested` | `is:open is:pr archived:false user-review-requested:@me` | 0 | `review_requested` |
| `authored_prs` | `is:open is:pr archived:false author:@me` | 11 | `author` |
| `authored_issues` | `is:open is:issue archived:false author:@me` | 23 | `author` |

- `user-review-requested` only matches reviews requested from you directly, not through a team. Team requests still arrive as notifications.
- A review request goes away on GitHub once you submit a review, so the item leaves the inbox by itself.
- Each item maps to a notification **reason**, so the existing `[[group]]` config sorts owed items too (assigned and review requests into Needs you; your own issues and PRs into Your stuff).

---

## 4. The plugin side (`dkt-gh`)

### 4.1 Protocol addition: `persistent`

One new, optional field on `item`. It stays protocol version 1: older dkt binaries ignore it.

| Field | Meaning |
|---|---|
| `persistent` | `true`: the item stays listed for as long as it's open at the source. dkt offers no plugin keys on it unless it's shown under ↻ because of new activity. It can be hidden with `h`. |

`updated_at` keeps its meaning, "the latest activity you haven't seen". For an owed item with no unread notification, that's its creation time. So editing an issue, or adding a label, doesn't put it under ↻.

### 4.2 Item IDs

| Item | `id` | `act` | `linked` |
|---|---|---|---|
| Has an unread notification | the thread ID, as today | as today | marks the thread done |
| Owed, no notification | `open:<key>`, e.g. `open:CoBrALab/RABIES#533` | exits 1: "open work can't be dismissed: make it a task or hide it" | does nothing, exits 0 |

The `act` error only shows on an older dkt that still offers `d` on owed items.

### 4.3 Merging

`inbox` builds one list from notifications and the owed searches, keyed by the item key (`owner/repo#n`):

| Notification | Owed | Item |
|---|---|---|
| yes | no | as today |
| no | yes | `persistent`, id `open:<key>`, label from §3 (`assigned`, `review`, `author`), `updated_at` = created |
| yes | yes | `persistent`, the thread's id, label and `updated_at` |

The suggested task, web URL and project are built the same way for both (§5.1 of the first design). Search results give `repository_url`, `number`, `title`, `html_url`, `created_at` and `pull_request` (for PRs), which is everything needed.

### 4.4 Polling the searches

The search API has **no ETag and never answers `304`**, and it has its own limit of 30 requests a minute (checked 2026-11-27). So `dkt-gh` doesn't search on every poll.

- **Cache:** the last search results are kept in `$DKT_CONFIG_DIR/plugins/gh/owed.json` (`fetched_at` and the items). It's a per-machine cache; losing it only costs one search.
- **When to search:** when the cache is older than `owed_poll_s` (default 300 s), or missing. Otherwise the cached results are used.
- **Cursor:** stays opaque to dkt. It becomes `<notifications ETag> <hash of the merged list>`, with no newline. `inbox --cursor C`:
  1. Asks for notifications with `If-None-Match`.
  2. If they're `304` and the searches aren't due: prints `unchanged`.
  3. Otherwise it builds the merged list (fetching notifications in full if needed, and searching if due), and prints `unchanged` if its hash equals the cursor's, or else the list with a new cursor.
- **Cost:** at most 4 search requests every 5 minutes, plus pages of 100. A notifications `304` stays free.
- **Search failures** (rate limit, 5xx): use the cache and carry on. If there's no cache, leave owed items out, and write the reason to stderr without failing the poll. Notifications still work.
- **Limits:** search returns at most 1000 results per query. That's plenty here; if a query reaches it, write a warning to stderr.

Config additions to `$DKT_CONFIG_DIR/plugins/gh/config.toml`:

```toml
owed = ["assigned", "review_requested", "authored_prs", "authored_issues"]  # default: all four
owed_poll_s = 300
```

`owed = []` turns owed items off and gives back today's behaviour.

---

## 5. The dkt side

### 5.1 Data

Hiding uses what dkt already has: a linked task and hidden fields.

| Action | Ops (one commit) |
|---|---|
| `h` on an owed item with no task | `LinkItem` as for `b` (backlog), then `status = dropped` and `x.<plugin>.hidden_at = now` |
| `h` on an item whose task exists | `status = dropped` if it's open, then `x.<plugin>.hidden_at = now` |
| `h` again, on ↻ *hidden · new activity* | `x.<plugin>.hidden_at = now` |
| `r` on the Hidden tab, or on ↻ *hidden* | `status = todo`, `on_week = false`, `x.<plugin>.hidden_at = ""` |
| `u` on the Hidden tab | `x.<plugin>.item = ""`, `x.<plugin>.hidden_at = ""`. The task stays dropped and moves to History. The item counts as new again. |

dkt writes these fields; the plugin never does. A task is **hidden** when any `x.<plugin>.hidden_at` on it is set. That's a new `Task.Hidden()` in `core`.

- History and `dkt ls --history` leave hidden tasks out. Export already leaves out dropped tasks.
- The append-only log means `u` can't delete the task. It stays in History as a dropped task, which is a true record.

### 5.2 Classification

`State.ClassifyItem(plugin, key, updatedAt)` gets the item's `persistent` flag and two new results:

| Result | When |
|---|---|
| `ItemNew` | no linked task |
| `ItemUpdated` | linked, task not hidden, `updatedAt` after `linked_at` (as today) |
| `ItemClosedOpen` | persistent, linked task closed and not hidden, nothing newer than `linked_at` |
| `ItemHiddenActivity` | linked task hidden, `updatedAt` after `hidden_at` |
| `ItemLinked` | anything else: hide it, and run `linked` again as today |

Non-persistent items behave exactly as now, except that a hidden task's notifications compare against `hidden_at`.

### 5.3 The TUI

- **Inbox:** owed items in their groups. ↻ Updates gets the two new kinds, with the labels from §2.2.
- **Plugin keys:** not offered on a persistent item unless it's ↻ because of new activity. Group keys (`D`) skip persistent items, and the `[y/N]` count only counts the items they act on.
- **Hidden tab:** tab `5`, after History. It lists hidden tasks, newest hide first, with the title, plugin and ref, how long ago it was hidden, and its last activity, if the current inbox list has it. A hidden task whose item isn't in the plugin's latest complete list (closed or unassigned on GitHub) is shown faint, marked *no longer open*. dkt doesn't change it by itself, since only the user's keys write the log.
- **The 📥 counter** counts owed items too. The Hidden tab isn't counted.

---

## 6. Build order

Test-first, as before. The test points are listed for each step.

1. **`dkt-gh` searches:** map a search result to an item; run the four queries with paging; cache and `owed_poll_s`. *Tested at* `toItem` for search results, and at `Run` against the fake API, including no ETag, rate limiting, and a missing or stale cache.
2. **`dkt-gh` merging and cursor:** one item per key; `persistent`; `open:` IDs for `act` and `linked`; the cursor as ETag plus hash. *Tested at* `Run`.
3. **`dkt` protocol:** `persistent` on `plugin.Item`, and the protocol doc. *Tested at* `plugin.Inbox`.
4. **`dkt` core:** `Task.Hidden`, the builders for hide, un-hide and make-a-task, and the new `ClassifyItem` results. History leaves hidden tasks out. *Tested at* the builders and `ClassifyItem` over replayed ops.
5. **`dkt` TUI:** owed items and the new ↻ kinds in the `inbox` type, then the Hidden tab and keys. *Tested at* the `inbox` type; the Model wiring checked by hand against `dkt-fake`, as before.
6. **Try it** against the real account, read-only first (`dkt gh inbox`), then in the TUI.

---

## 7. Decisions and why

| Decision | Reason |
|---|---|
| Owed = assigned, review requested, your open PRs, your open issues | Owner's choice: the inbox should be complete. Each list can be turned off. |
| Owed items can't be dismissed, only made a task, hidden or postponed | Dismissing can't stick: the next search would bring the item back. `h` is the deliberate "not now, maybe never". |
| Hidden = a dropped, linked task with `x.<plugin>.hidden_at` | No new kind of stored state. It syncs between machines through the op log, and it's reversible. |
| A separate Hidden tab | Keeps History meaning "work I closed", and gives one place to review what's being ignored. |
| A closed task with open work shows under ↻ | Owner's choice: nothing slips. Hiding is the explicit way to stop it. |
| New activity on a hidden item shows under ↻ | Someone finally answering your old issue is the moment you want to see it. |
| Searches are cached by the plugin, every 5 minutes | No ETag on search, and a 30-per-minute limit shared with everything else using the token. Assignments and review requests also arrive as notifications, so a 5-minute delay rarely shows. |
| `persistent` is a generic protocol field | Worklenz's assigned tasks are owed work in the same sense. |

---

## 8. Verified facts and open questions

Checked on 2026-11-27, read-only, with `gh`'s OAuth token (scopes `repo`, `read:org`, `gist`, `admin:public_key`).

| Question | Result |
|---|---|
| Does `/search/issues` send an `ETag`, or answer `If-None-Match` with `304`? | **No.** No `ETag` header, and the conditional request returned `200`, costing a request. |
| What's the search rate limit? | 30 a minute (`X-RateLimit-Resource: search`), separate from the core limit. |
| Do the queries work with this token? | Yes: `assignee:@me` returned 16; `user-review-requested:@me` and `review-requested:@me` returned 0. |
| What does the built `dkt-gh inbox` give on the owner's account? | 43 owed items, each once (16 assigned, 27 authored), alongside 0 unread notifications. Rerunning with the cursor gave `unchanged` without searching. The output, replayed through dkt's TUI, showed all 43 with `h` offered and no `d`. |
| Is an assignment always also a notification? | *Unverified.* Usually (reason `assign`), but not when you assign yourself. That's one reason the searches exist. |
| Do search results include archived repos? | Excluded with `archived:false`. *Unverified* that this covers repos you lost access to. |

## 9. Out of scope (for now)

- Un-assigning yourself, or closing items, from dkt (writes to issues and PRs; see the first design, §2).
- Team review requests as owed work.
- Showing *why* an item is owed when there are several reasons (assigned and authored): the first matching list in §3's order wins.
