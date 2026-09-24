# How to sync dkt between machines

This guide shows you how to share one set of tasks between two or more machines through a git remote.

You need:

- A git remote that is empty or already holds dkt data. For example, a new private GitHub repository, or a bare repository on a server: `git init --bare ~/dkt.git`.
- Access to the remote from each machine **with no password prompt**, through an SSH key or a credential helper. dkt never asks for a password.

## Connect the first machine

If you have not used dkt on this machine yet:

```sh
dkt init git@github.com:you/dkt-data.git
```

If you already use dkt here with no remote (`local only`), run the same command. dkt adds the remote and pushes your existing tasks.

`init` prints the sync status. It must be `synced`. If it shows a warning that the push failed, see [If the status stays at "N unpushed"](#if-the-status-stays-at-n-unpushed).

## Connect each other machine

On each other machine, run the same command:

```sh
dkt init git@github.com:you/dkt-data.git
dkt ls
```

`dkt ls` shows the tasks from the first machine.

From now on, dkt pulls before each command and pushes after each change. You do not need to run anything else.

## Workstations that share a home directory

If several workstations share one NFS home directory, they also share one config and one data directory. Run `dkt init` **once**, on any of them. Each workstation writes to its own log file, because each file name includes the hostname.

If two machines have the same short hostname but do not share a home directory, set `DKT_HOSTNAME` to a different value on one of them before you run `init`:

```sh
export DKT_HOSTNAME=lab-pc-2
```

## Sync on demand

- In the CLI, run `dkt sync`.
- In the TUI, press `R`.

Do this when you want changes that another machine made while this TUI was open.

## If the status stays at "N unpushed"

1. Run `dkt sync` and read the error.
2. Test access to the remote with no prompt. Use your data directory if it is not the default:

   ```sh
   git -C ~/.local/share/dkt fetch
   ```

   If this asks for a password or a passphrase, add your key to `ssh-agent` or set up a credential helper.
3. Run `dkt sync` again. The status must be `synced`.

Your changes are safe while they are unpushed. They are committed on this machine, and the next successful push sends them.

## Related

- [How dkt stores and syncs data](../explanation/storage-and-sync.md)
- [Configuration reference](../reference/configuration.md)
