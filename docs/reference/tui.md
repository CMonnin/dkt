# TUI reference

Run `dkt` with no command to open the TUI. It pulls from the remote before it opens.

## Screen

- **Top line:** the four tabs and the sync status. The status shows `synced`, `N unpushed` or `local only`, and it adds `syncing…` while a push or sync runs.
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
| `4` | History | Done and dropped tasks, newest first | None |

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
| `1` to `4` | Go to that tab |
| `/` | Search (not on Projects) |
| `esc` | Clear the search and the project filter |
| `R` | Pull, push and reload |
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

## Input prompts

| Prompt | Behaviour |
|---|---|
| Add | Words that start with `#` become tags. On Week, the task goes on this week. On Backlog, it goes to the backlog, in the filtered project if one is set. |
| Search | Filters as you type. Words that start with `#` must be tags on the task. Other words must appear in the title. |
| Tags | The field holds the full tag set. Separate tags with spaces or commas. `tab` completes a tag from tags already in use. |
| Note | One line. If the note has several lines, use `E`. |

In a prompt, `enter` saves and `esc` cancels. `esc` in the search prompt also clears the search.

## Sync behaviour

- Each change is saved and committed at once. A push then runs in the background.
- The TUI does not see changes from other processes or machines until you press `R`.
- Each change reloads all data from disk, so changes made by the CLI on the same machine appear on the next change.
