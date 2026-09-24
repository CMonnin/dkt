# dkt

A weekly to-do list for the terminal, with a CLI and a TUI.

- Plan one week at a time. Keep everything else in a backlog.
- At the start of each week, decide what to do with last week's unfinished tasks.
- Export a Markdown report of your work for your weekly meeting.
- Sync between machines through any git remote, with no server and no merge conflicts. It works offline.

## Install

Linux, amd64:

```sh
curl -sL https://github.com/CMonnin/dkt/releases/download/v0.1.0/dkt_0.1.0_linux_amd64.tar.gz | tar -xz dkt
mkdir -p ~/.local/bin && mv dkt ~/.local/bin/
```

Or, with Go:

```sh
go install github.com/CMonnin/dkt/cmd/dkt@latest
```

To update later, run `dkt self-update`.

dkt needs `git`.

## Quick start

```sh
dkt init                            # or: dkt init git@github.com:you/dkt-data.git
dkt add "rerun QC #adni"            # add to this week, with tag "adni"
dkt add "tidy the wiki" -b          # add to the backlog
dkt ls                              # show this week
dkt done rerun QC                   # refer to a task by words of its title
dkt export                          # report for the weekly meeting
dkt                                 # open the TUI
```

## Documentation

**Learn**

- [Tutorial: your first week with dkt](docs/tutorial.md)

**How-to guides**

- [How to sync dkt between machines](docs/how-to/sync-machines.md)
- [How to prepare for your weekly meeting](docs/how-to/weekly-meeting.md)
- [How to develop and release dkt](docs/how-to/develop.md)

**Reference**

- [CLI](docs/reference/cli.md)
- [TUI keys](docs/reference/tui.md)
- [Configuration and environment](docs/reference/configuration.md)
- [Data format](docs/reference/data-format.md)
- [Packages](docs/reference/packages.md)

**Explanation**

- [The weekly cycle](docs/explanation/weekly-cycle.md): weeks, leftovers, the meeting window
- [How dkt stores and syncs data](docs/explanation/storage-and-sync.md): op logs, replay, git
