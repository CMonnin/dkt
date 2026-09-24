# Tutorial: your first week with dkt

In this tutorial, we will plan a small week of work, track progress on it and produce a report for a weekly meeting. It takes about 10 minutes.

We will use a practice data directory, so nothing we do here touches any real tasks. At the end, you will delete it.

You need `dkt` installed (see the [README](../README.md#install)) and `git`.

## Set up a practice space

Open a terminal and run these two lines. They tell dkt to keep its config and data in `~/dkt-tutorial`:

```sh
export DKT_CONFIG_DIR=~/dkt-tutorial/config
export DKT_DATA_DIR=~/dkt-tutorial/data
```

Keep this terminal open for the whole tutorial.

Now set up dkt:

```sh
dkt init
```

You will see something like this:

```
config: /home/you/dkt-tutorial/config
data:   /home/you/dkt-tutorial/data
log:    /home/you/dkt-tutorial/data/ops/laptop-b12ec4e2.jsonl
status: local only
```

`local only` means there is no remote yet. That is fine for now.

## Add some tasks

Add a work task:

```sh
dkt add "Write the release notes"
```

```
added 01M3A3NM75  Write the release notes  (week)
```

The first column is the task's ID. Yours will be different. `(week)` tells you the task is on this week's list.

Now add a personal task:

```sh
dkt add "Book a dentist appointment" -c personal
```

## Add a project

Tasks can belong to a project. Create one:

```sh
dkt project add "Website"
```

```
project 01M3A3NMB9  Website (work)
```

Add a task to it. The `#bug` word becomes a tag:

```sh
dkt add "Fix the broken contact form #bug" -p website
```

Notice that you typed `website` in lower case. dkt found the project anyway.

Add one more task to the project, but put it in the backlog, not on this week:

```sh
dkt add "Update the privacy page" -p website -b
```

```
added 01M3A3NMFY  Update the privacy page  (backlog)
```

## Look at your week

```sh
dkt ls
```

```
Work
  01M3A3NM75 [ ] Write the release notes
  01M3A3NMCV [ ] Fix the broken contact form  (Website) #bug
Personal
  01M3A3NM8B [ ] Book a dentist appointment
```

Work and personal tasks are in separate groups. The privacy page task is not here, because it is in the backlog.

## Make progress

Start work on the contact form. You do not need the ID. A few words of the title are enough:

```sh
dkt doing contact form
```

```
[~] 01M3A3NMCV  Fix the broken contact form
```

The release notes are stuck. Mark them blocked, with a reason:

```sh
dkt block release notes -r "waiting for the final build"
```

You booked the dentist. Mark it done:

```sh
dkt done dentist
```

Look at the week again:

```sh
dkt ls
```

```
Work
  01M3A3NM75 [!] Write the release notes — waiting for the final build
  01M3A3NMCV [~] Fix the broken contact form  (Website) #bug
Personal
  01M3A3NM8B [x] Book a dentist appointment
```

Each box shows a status: `[~]` doing, `[!]` blocked, `[x]` done. The done task stays on the list until Monday.

## Plan from the backlog

You have time for the privacy page after all. Look at the backlog:

```sh
dkt ls --backlog
```

```
Website
  01M3A3NMFY [ ] Update the privacy page
```

Move it onto this week:

```sh
dkt week privacy
```

```
on week: 01M3A3NMFY  Update the privacy page
```

## Produce the meeting report

Your weekly meeting is on Thursday at 15:00 by default. Produce the report:

```sh
dkt export
```

```markdown
# Work update: 2026-09-17 15:00 → 2026-09-24 12:23

## Inbox

**Blocked**
- Write the release notes — waiting for the final build

## Website

**In progress**
- Fix the broken contact form

**Next**
- Update the privacy page
```

Your dates will be different. Notice two things:

- The report groups tasks by project. "Inbox" holds the tasks with no project.
- The dentist appointment is not in the report. The report is for work, so dkt leaves personal tasks out.

## Use the TUI

Everything you did so far also works in a full-screen interface. Open it:

```sh
dkt
```

You see the same week as in `dkt ls`, on the **Week** tab.

1. Press `j` to move down to "Fix the broken contact form".
2. Press `x`. The task is done, and its title is crossed out.
3. Press `2` to see the **Backlog** tab. It is empty now.
4. Press `4` to see the **History** tab. The dentist appointment and the contact form are there.
5. Press `1` to go back to the Week tab.
6. Press `?` to see all the keys. Press `?` again to hide them.
7. Press `q` to quit.

Run `dkt ls` again. The contact form shows `[x]`. The CLI and the TUI use the same data.

## Clean up

Delete the practice space, and close the terminal so the two `export` settings go away:

```sh
rm -rf ~/dkt-tutorial
```

## What you did

You set up dkt, added tasks to the week and the backlog, made a project, changed task statuses, planned from the backlog and produced a meeting report. You did it from the CLI and from the TUI.

## Next steps

- To use dkt for real, run `dkt init` in a new terminal, without the practice settings.
- To use dkt on more than one machine, see [How to sync dkt between machines](how-to/sync-machines.md).
- To learn what happens to unfinished tasks on Monday, read [The weekly cycle](explanation/weekly-cycle.md).
