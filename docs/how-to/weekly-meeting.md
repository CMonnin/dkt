# How to prepare for your weekly meeting

This guide shows you how to clean up the week and produce the report for your weekly meeting.

## Set your meeting time

Do this once. Edit `~/.config/dkt/config.toml`:

```toml
meeting_day = "tue"
meeting_time = "10:30"
```

The default is Thursday at 15:00. See the [configuration reference](../reference/configuration.md) for the file location and formats.

## Deal with last week's leftovers

On Monday, or when dkt first reports leftovers, decide what to do with each one.

**In the TUI:** open `dkt`. The Week tab shows a banner and puts the cursor on the first leftover. On each leftover, press:

- `k` to keep it on this week
- `d` to defer it to the backlog
- `D` to drop it

To keep all of them, press `esc`.

**In the CLI:**

```sh
dkt plan
```

Then, for each task in the list, run one of these:

```sh
dkt week <ref>     # keep
dkt defer <ref>    # back to the backlog
dkt drop <ref>     # drop
```

## Bring the statuses up to date

The report takes **In progress**, **Blocked** and **Next** from each task's current status. Before you export, check that the statuses are correct:

```sh
dkt ls
dkt doing <ref>
dkt block <ref> -r "waiting for access"
dkt done <ref>
```

Only work tasks go in the report. If a work task is marked personal:

- If it has no project, run `dkt edit <ref> -c work`.
- If it is in a personal project, move it to a work project with `dkt edit <ref> -p <project>`. Or, if the whole project is work, open the TUI Projects tab and press `c` on the project.

## Export the report

To print the report for the current meeting window:

```sh
dkt export
```

To write it to a file:

```sh
dkt export --out update.md
```

### If you need a different time window

If you missed a meeting, or you need a report for a specific period, set the window yourself:

```sh
dkt export --since 2026-09-01 --until 2026-09-14
```

A bare `--until` date includes the whole of that day. To set a time, use `2026-09-14T17:00`.

### If the meeting covers only one topic

Tag the tasks for that topic, then filter by the tag:

```sh
dkt tag <ref> +adni
dkt export --tag adni
```

## Related

- [The weekly cycle](../explanation/weekly-cycle.md)
- [CLI reference: `export`](../reference/cli.md#export)
