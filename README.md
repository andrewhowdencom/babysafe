# babysafe

> A tool that captures inputs entirely (Linux Only), so that babies can play without inadvertently breaking stuff.

`babysafe` grabs Linux input devices on demand so that a small human can sit
at the keyboard without accidentally triggering actions on the host. Devices
are released automatically on `SIGINT` / `SIGTERM`.

There is no configuration file. Everything is expressed on the command line.

## Quick start

```bash
# What input devices does the kernel see, and what are they called?
sudo babysafe list

# Just grab every keyboard.
sudo babysafe grab --match type=keyboard

# Grab a specific keyboard by friendly name.
sudo babysafe grab --match name=Logitech

# Grab by device path (glob works against by-id and by-path symlinks).
sudo babysafe grab --match path=/dev/input/by-id/usb-Logitech_*-event-kbd

# Combine rules. A device must satisfy every --match and no --exclude.
sudo babysafe grab \
  --match type=keyboard \
  --exclude path=/dev/input/event0
```

`grab` requires an interactive, color-capable terminal. Each initial keyboard
press creates a short, colorful effect that bounces around the screen alongside
other recent presses. Effects are capped at 64; releases and key autorepeat do
not create more effects. Printable characters use a US keyboard layout, while
modifier, navigation, function, lock, and media keys use named labels.

To stop a session from a grabbed keyboard, press `Ctrl+Alt+Esc`. This instruction
remains visible throughout the animation. You can also send `SIGINT` or
`SIGTERM` from another terminal. Every grabbed device is released and the
previous terminal screen and cursor are restored before the process exits.

## Commands

| Command   | Description                                       |
| --------- | ------------------------------------------------- |
| `list`    | List input devices and their detected types.      |
| `grab`    | Grab devices and animate initial keyboard presses. |
| `version` | Print the babysafe version and exit.              |

`grab` accepts repeatable `--match` and `--exclude` flags of the form
`key=value`. Supported keys are:

| Key    | Example value                                       | Meaning                                          |
| ------ | --------------------------------------------------- | ------------------------------------------------ |
| `path` | `/dev/input/by-id/usb-Logitech_*-event-kbd`         | Glob against the device path (symlinks resolved).|
| `name` | `Logitech G512`                                     | Case-insensitive substring of `EVIOCGNAME`.      |
| `type` | `keyboard` \| `mouse` \| `touchpad` \| `gamepad` \| `other` | Match a category inferred from capabilities. |

`babysafe` itself accepts `--log-level debug|info|warn|error`.

## Why Linux only?

`pkg/capture` issues `EVIOCGRAB`, a Linux-specific ioctl that asks the
kernel to route every event from a given device to this process
exclusively. There is no portable equivalent.

## Project layout

```
cmd/babysafe/main.go      Entry point. Minimal.
internal/animation/       Keyboard normalization and terminal animation.
internal/cli/             Cobra command tree (root, list, grab, version).
internal/version/         Build-time version stamp.
pkg/capture/              Linux input-grab primitives (importable as a Go library).
  capture.go              Session lifecycle, match-and-grab loop.
  match.go                path / name / type Matchers.
  list.go                 ListDevices + DeviceType detection.
Taskfile.yml              Task runner — `task validate` runs the whole suite.
.golangci.yaml            golangci-lint configuration.
```

`pkg/capture` lives under `pkg/` (not `internal/`) on purpose: any Go program
can import it to grab input devices without pulling in the babysafe CLI.

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

### Manual animation smoke test

On Linux, run `sudo babysafe grab --match type=keyboard` and verify:

- printable, modifier, navigation, function, and media keys animate;
- rapid mixed presses remain responsive and produce layered effects;
- holding a key produces only one effect;
- `Ctrl+Alt+Esc` exits; and
- the previous screen and visible cursor are restored.

## License

See `LICENSE`.
