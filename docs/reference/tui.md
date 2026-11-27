# TUI reference

Run `dkt` with no command to open the TUI. It pulls from the remote before it opens.

## Screen

- **Top line:** the four tabs and the sync status. The status shows `synced`, `N unpushed` or `local only`, and it adds `syncing…` while a push or sync runs. With [inbox plugins](#inbox), it starts with the inbox counter, e.g. `📥 7`.
- **Filter line:** the active search and project filter, if any.
- **Leftover banner:** on the Week tab, when leftovers exist.
- **List:** tasks or projects under group headings.
- **Bottom lines:** the input prompt, messages and key help.

Task rows show the status box, the title, the project (not on Backlog), the tags, the blocked reason and the first line of the note (`✎`). The History tab also shows the date each task was closed.

## Tabs

| Key | Tab | Shows | Grouped by |
|---|---|---|---|
| `1` | Week | Open tasks on this week, plus tasks marked done since Monday 00:00 | Work, Personal |
| `2` | Backlog | Open tasks not on this week | Inbox, then projects |
| `3` | Projects | Projects that are not archived | None |
| `4` | History | Done and dropped tasks, newest first. Not hidden ones. | None |
| `5` | Hidden | Tasks whose inbox item you hid, newest hide first | None |

When you change tabs, the search clears. The project filter clears unless the new tab is Backlog.

## Keys

### Everywhere

| Key | Action |
|---|---|
| `j`, `↓` | Move down |
| `k`, `↑` | Move up (for `k` on a leftover, see [Leftovers](#leftovers)) |
| `g`, `Home` | First row |
| `G`, `End` | Last row |
| `tab`, `l`, `→` | Next tab |
| `shift+tab`, `h`, `←` | Previous tab |
| `1` to `5` | Go to that tab |
| `/` | Search (not on Projects) |
| `esc` | Clear the search and the project filter |
| `R` | Pull, push and reload |
| `I` | Open the [inbox](#inbox) |
| `?` | Show or hide full help |
| `q`, `ctrl+c` | Quit |

### Tasks

| Key | Action | Tabs |
|---|---|---|
| `a` | Add a task | Week, Backlog |
| `x` | Mark done, or mark a done task todo | Week, Backlog |
| `x` | Reopen | History |
| `i` | Mark doing, or mark a doing task todo | All task tabs |
| `b` | Block. Asks for a reason; `enter` with no text skips it. | All task tabs |
| `D` | Drop an open task | All task tabs |
| `w` | Move to this week | Backlog |
| `w`, `d` | Move an open task to the backlog | Week |
| `t` | Edit tags | All task tabs |
| `n` | Edit a one-line note | All task tabs |
| `E` | Open the task in `$EDITOR` (see [The edit form](cli.md#the-edit-form)) | All task tabs |
| `c` | Switch category between work and personal. Inbox tasks only. | All task tabs |

### Projects tab

| Key | Action |
|---|---|
| `a` | Add a project with category `default_category` |
| `enter` | Open the Backlog tab, filtered to this project |
| `r` | Rename |
| `e` | Edit the description |
| `c` | Switch category between work and personal |
| `A` | Archive. Asks `[y/N]`. Also drops the project's open tasks. |

### Leftovers

On the Week tab, a leftover row shows `⟲ leftover`. The cursor starts on the first leftover. These keys have a special meaning:

| Key | Action |
|---|---|
| `k` | Keep: stay on the week, and count as added this week. |
| `d` | Defer to the backlog. |
| `D` | Drop. |
| `esc` | Keep all leftovers. This works on any row of the Week tab while leftovers exist. |

`k` means keep only when the cursor is on a leftover. `↑` always moves up.

## Inbox

The inbox lists items from the plugins named in `inbox` in [`config.toml`](configuration.md), for example GitHub notifications from `dkt-gh`. Without inbox plugins, there's no inbox.

### Polling

- dkt polls each plugin when the TUI opens, then as often as the plugin asks (at least every 30 s). The CLI never polls.
- The top line shows the number of items, e.g. `📥 7`.
- If a poll fails, the last list stays, the counter turns grey with a `?` (`📥 7?`), and the reason is shown once. dkt retries after 60 s.
- When the first results arrive, dkt opens the inbox, but only if you haven't pressed a key yet and there are no leftovers. Otherwise only the counter changes.

### Layout

```
📥 Inbox (4)
↻ Updates
▸ RABIES#520  new activity · task: Review: atlas registration  1h
Needs you
  [review] RABIES#533  fix motion correction  2h
  [assigned] CPPQ#12  export fails on empty session  1d
FYI
  [subscribed] RABIES  v0.5.2 released  1d
```

- **↻ Updates:** items that are already tasks and need a look:
  - *new activity*: activity since the task was made;
  - *task closed, still open*: owed work whose task you closed, while the item is still open at the source;
  - *hidden · new activity*: activity on an item you hid, since you hid it.
- Then the plugin's groups, in its order. Items in a group the plugin didn't declare go under **Other**. With several plugins, each header starts with the plugin's name.
- Within a group, newest first.
- Items that are already tasks, with nothing new, aren't shown.
- **Owed work** (an issue assigned to you, say) stays in the inbox until it's a task or hidden. It can't be dismissed, because the plugin would list it again.

### Inbox keys

| Key | Action |
|---|---|
| `t` | Make a task on this week from the item, linked to it. The plugin is told (for GitHub: marked done). |
| `b` | The same, in the backlog. |
| `T`, `B` | The same, but edit the title first. |
| `z` | Hide the item until the TUI is next opened. |
| `o` | Open the item's web page. |
| `r` | On a ↻ item whose task is closed: reopen the task. On ↻ *hidden · new activity*: make it a real task, in the backlog. |
| `h` | Owed work only: hide it. It moves to the Hidden tab and comes back under ↻ only with new activity. |
| `esc` | Go to the Week tab. |

`t`, `b`, `T` and `B` aren't offered on ↻ items, so the same thing can't become two tasks.

The plugin adds its own keys, shown in the help line. For `dkt-gh`: `d` dismiss, `u` unsubscribe, and `D` dismiss the whole group (asks `[y/N]`). If a plugin key fails, the item stays and the reason is shown. Plugin keys aren't offered on owed work unless it's under ↻ because of new activity, and `D` leaves owed work out.

### Hidden tab

| Key | Action |
|---|---|
| `r` | Make it a real task: todo, in the backlog, no longer hidden. |
| `u` | Back to the inbox, as a new item. The hidden task stays in History as dropped. |
| `o` | Open the item's web page. |

A row shows *no longer open* when the plugin's latest list doesn't have the item (closed, or not yours any more). Other task keys don't work on this tab.

A task made from an item gets the plugin's suggested title, category, tags and note. It goes in the project linked to the item's project, or else in a project with the suggested name, or else in a new one.

## Input prompts

| Prompt | Behaviour |
|---|---|
| Add | Words that start with `#` become tags. On Week, the task goes on this week. On Backlog, it goes to the backlog, in the filtered project if one is set. |
| Search | Filters as you type. Words that start with `#` must be tags on the task. Other words must appear in the title. |
| Tags | The field holds the full tag set. Separate tags with spaces or commas. `tab` completes a tag from tags already in use. |
| Note | One line. If the note has several lines, use `E`. |
| Task title | From `T` or `B` in the inbox. Prefilled with the plugin's suggestion. |

In a prompt, `enter` saves and `esc` cancels. `esc` in the search prompt also clears the search.

## Sync behaviour

- Each change is saved and committed at once. A push then runs in the background.
- The TUI does not see changes from other processes or machines until you press `R`.
- Each change reloads all data from disk, so changes made by the CLI on the same machine appear on the next change.
