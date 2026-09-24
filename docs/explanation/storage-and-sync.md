# How dkt stores and syncs data

dkt has no database and no server. Your tasks live in a git repository as a set of append-only logs. This page explains that design, the reasons for it and what it costs.

## The problem

dkt is for one person who works on several machines, for example a laptop, a desktop and a few workstations that share an NFS home directory. Some machines are sometimes offline. The person must be able to add or change a task on any machine at any time, and every machine must reach the same state in the end, with no merge conflicts to fix by hand.

A to-do app usually solves this with a server. dkt uses a git remote that you already have, for example a private GitHub repository or a bare repository on a machine you can reach over SSH.

## Ops, not state

dkt does not store the current list of tasks. It stores each change as an **op**: "create this task", "set this task's status to done", "add this tag". The current state is what you get when you apply all the ops in order. dkt does this on every start. It is fast, because a personal to-do list is small.

Each machine writes to its own file, `ops/<hostname>-<machine_id>.jsonl`, and only appends to it. This is the key to conflict-free sync:

- Two machines never write to the same file.
- Git never has to merge two edits of one file.
- `git rebase` always succeeds, because the local commits only touch this machine's file.

This is why dkt pulls with `git fetch` and `git rebase`, not `git merge`. A rebase keeps the history linear, and it can't conflict here.

## Why the file name has two parts

The hostname makes the file easy to recognise. The `machine_id` is a random value that `dkt init` writes to `config.toml`.

Workstations that share an NFS home directory also share `config.toml`, so they share a `machine_id`. Their hostnames are still different, so their files are still different. If two machines had the same hostname and did not share config, the `machine_id` would keep their files apart.

## How conflicts resolve

Two machines can still change the same task while offline. For example, the laptop renames a task and the desktop marks it done. When both logs arrive, dkt applies every op in order of timestamp, with the op ID to break ties.

The result is **last write wins** for each field:

- The two changes above touch different fields, so both survive.
- If both machines renamed the task, the later rename wins.
- Tags are added and removed one at a time, so the tag set holds every change in time order.

Every machine applies the same ops in the same order, so every machine gets the same result. The order in which the logs arrived does not matter.

Because the order uses each machine's clock, a machine with a very wrong clock can win or lose conflicts that it should not. For one person's to-do list, this is an acceptable cost.

An op can arrive before the task it changes. This happens when a log from one machine has arrived but the log that created the task has not. dkt ignores such ops until the create op arrives. Nothing is lost; the change appears after the next pull.

## Why every command pulls and pushes

dkt pulls before each command and before the TUI opens, so you almost always act on current data. It pushes after each change, in the background. In the CLI, a detached child process does the push, so the command returns at once. In the TUI, the push runs while you keep working.

When a machine is offline, the pull fails silently and the push leaves commits waiting. The status shows `N unpushed`. dkt counts these commits from git each time; it does not store the count. The next successful pull or push brings the machine up to date.

## Locking

The CLI, the TUI and the background push can all run at the same time on one machine, and NFS workstations can run them on one shared directory. dkt takes an exclusive `flock` on `.git/dkt.lock` for every log write and every git operation. On NFSv4, a `flock` is also a lock between hosts.

The cost is that a change can wait while a push holds the lock. A push has a 30-second network timeout, and SSH has a 10-second connect timeout, so the wait has a limit.

If a crash leaves `.git/index.lock` behind, git refuses to run. When dkt holds its lock and finds an `index.lock` older than one minute, it deletes it. No other dkt process can own that file at that moment.

## Crash safety

dkt writes each batch of ops in one `write` call, then calls `fsync`. Only then does it commit. This order means:

- If git fails after the write, the change is still saved. dkt shows a warning, and the next pull commits the change.
- If the machine crashes during the write, the last line can be incomplete. dkt skips that line with a warning. The next write starts on a new line.

## What this design gives up

- **Deletes.** Nothing is ever removed from the log. "Dropped" and "archived" are statuses, not deletes.
- **Size.** The log grows forever. For a to-do list, this takes years to matter.
- **Real-time updates.** The TUI does not watch for changes. Press `R` to see changes from other machines.
- **Clock trust.** Conflicts resolve by timestamp, so clocks must be about right.

In return, dkt has no server, no database and no merge conflicts. It works offline, and git gives you the full history of every change.

## See also

- [Data format reference](../reference/data-format.md)
- [How to sync dkt between machines](../how-to/sync-machines.md)
