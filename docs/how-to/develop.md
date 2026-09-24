# How to develop and release dkt

This guide is for contributors. It shows you how to build, test and try dkt without touching your real tasks, and how to make a release.

You need Go (the version in `go.mod`) and git.

## Build and test

```sh
go test ./...
go build -o bin/dkt ./cmd/dkt
scripts/e2e.sh
```

- `go test` runs the unit tests for `internal/core` and `internal/store`.
- `scripts/e2e.sh` needs `bin/dkt`. It simulates two machines on a local bare remote, under `.dev/e2e`. It ends with `all e2e checks passed`.

## Try dkt without touching your real data

Point dkt at `.dev` before you run it. `.dev` is in `.gitignore`.

```sh
export DKT_CONFIG_DIR=$PWD/.dev/config DKT_DATA_DIR=$PWD/.dev/data
git init --bare .dev/remote.git
bin/dkt init $PWD/.dev/remote.git
bin/dkt add "rerun QC #adni"
bin/dkt
```

If you forget the `export` line, dkt uses `~/.config/dkt` and `~/.local/share/dkt`, which hold your real tasks.

To simulate a second machine, use a second pair of directories and a different hostname:

```sh
DKT_HOSTNAME=other DKT_CONFIG_DIR=$PWD/.dev/other/config DKT_DATA_DIR=$PWD/.dev/other/data \
  bin/dkt init $PWD/.dev/remote.git
```

To start again, delete `.dev`.

## Find your way around the code

See the [package reference](../reference/packages.md).

## Rename the tool

Edit `internal/app/app.go` only. It holds the binary name, the environment variable prefix and the GitHub repository.

## Make a release

1. Check that `go test ./...` and `scripts/e2e.sh` pass.
2. Tag the commit and push the tag:

   ```sh
   git tag v0.2.0
   git push origin v0.2.0
   ```

The `release` GitHub workflow runs GoReleaser. It builds a static `linux/amd64` binary and publishes `dkt_<version>_linux_amd64.tar.gz` and `checksums.txt`. `dkt self-update` looks for an archive with this name. GoReleaser sets the version without the `v`, so `dkt version` prints `dkt 0.2.0`.
