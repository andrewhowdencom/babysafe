# babysafe

> A tool that captures inputs entirely (Linux Only), so that babies can play without inadvertently breaking stuff.

`babysafe` grabs every available Linux input device (`/dev/input/event*`) so
that a small human can sit at the keyboard without accidentally triggering
actions on the host. Devices are released automatically on `SIGINT` /
`SIGTERM`, or held if `--release-on-exit=false` is set.

## Status

Skeleton. The CLI, configuration plumbing, and the input-grab primitives
are in place. End-to-end grab behaviour will be exercised as soon as the
project is run on a Linux host with access to `/dev/input/event*`.

## Quick start

```bash
# Build and run.
task build:linux
sudo ./bin/babysafe run

# With a config file.
sudo ./bin/babysafe --config ./babysafe.yaml run

# Different log verbosity.
./bin/babysafe --log-level debug run
```

## Configuration

The default configuration is in `internal/config/config.go`. Any of these
keys can be overridden:

| Key                          | Type      | Default              | Description                                       |
| ---------------------------- | --------- | -------------------- | ------------------------------------------------- |
| `log.level`                  | string    | `info`               | `debug`, `info`, `warn`, `error`.                 |
| `capture.device-globs`       | []string  | `["/dev/input/event*"]` | Globs expanded against the filesystem.         |
| `capture.excludes`           | []string  | `[]`                 | Substrings: any path matching is skipped.         |
| `capture.release-on-exit`    | bool      | `true`               | Release devices when the process exits.           |

Settings can be supplied in three ways (highest priority last):

1. Built-in defaults.
2. A YAML file at `--config <path>` (or `./babysafe.yaml` by default).
3. Environment variables prefixed with `BABYSAFE_`, e.g. `BABYSAFE_LOG_LEVEL=debug`.

## Project layout

```
cmd/babysafe/main.go      Entry point. Minimal.
internal/cli/             Cobra command tree.
internal/config/          Typed configuration + viper hydration.
internal/app/             Application logic; the seam between CLI and capture.
internal/version/         Build-time version stamp.
pkg/capture/              Linux input-grab primitives (importable as a Go library).
Taskfile.yml              Task runner — `task validate` runs the whole suite.
.golangci.yaml            golangci-lint configuration.
```

## Development

```bash
task setup       # Install wire, golangci-lint, gotests, godoc.
task validate    # format + vet + lint + test + build.
task test        # `go test -race ./...`
task lint        # `golangci-lint run`
task build       # Build the binary into ./bin/babysafe.
task build:linux # Same, but force GOOS=linux.
task run         # `go run ./cmd/babysafe` (honours `CLI_ARGS`).
task clean       # Remove build artifacts.
```

## Why Linux only?

`pkg/capture` issues `EVIOCGRAB`, a Linux-specific ioctl that asks the
kernel to route every event from a given device to this process
exclusively. There is no portable equivalent — the package documents
this loudly and refuses to attempt cross-platform compilation in CI.

## License

See `LICENSE`.