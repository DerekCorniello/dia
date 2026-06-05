# dia

dia: a cross-platform desktop launcher for dev workspaces.

Define a workspace in YAML, click start, and dia brings up the editor,
terminal, browser, and services for that project. No window manager, no
process snapshot, just deterministic rebuilds from a config.

See [PLAN.md](./PLAN.md) for the full design and implementation plan.

## Features

- JetBrains-style desktop launcher (Wails + Svelte)
- Workspace definitions in YAML
- Built-in app types: editor, terminal, browser, service, custom
- First-class `gh` CLI integration for GitHub workflows
- Third-party app types via `dia-*` executables on `PATH`
- Both global and project-local configs with discovery
- Cross-platform: Linux, macOS, Windows
- Scriptable CLI alongside the GUI

## Install

### From source (requires Go 1.26+)

```sh
go install github.com/DerekCorniello/dia/cmd/dia@latest
```

### Prebuilt binaries

Download the archive for your OS/arch from the
[GitHub Releases](https://github.com/DerekCorniello/dia/releases) page.

## Quickstart

Create a workspace:

```sh
dia new myproject
```

This writes a starter YAML to `~/.config/dia/workspaces/myproject.yaml` and
opens it in `$EDITOR`. Edit it to your liking, then:

```sh
dia start myproject
```

Or just run `dia` with no arguments to open the desktop launcher and pick
from your workspaces.

## Example workspace

```yaml
name: compiler
apps:
  - type: editor
    cmd: code .
    cwd: ~/projects/compiler

  - type: terminal
    cmd: cargo run
    cwd: ~/projects/compiler

  - type: browser
    url: http://localhost:8080
```

Project-local configs are also supported. Drop a `.dia.yaml` at the root of
your repo and dia will pick it up automatically.

## CLI

```sh
dia                     # opens the GUI
dia list                # list all workspaces
dia start <name>        # start a workspace
dia stop <name>         # stop a workspace
dia status              # running instances and PIDs
dia new <name>          # create a starter workspace
dia edit <name>         # open the config in $EDITOR
dia open <name>         # reveal the workspace in the file manager
dia doctor              # smoke checks
```

## Plugins

Third-party app types are just executables on your `PATH` named `dia-*`.
dia discovers them at startup and registers each as
`type: <name-without-prefix>`.

For example, `dia-foo` on `PATH` is invoked as:

```yaml
- type: foo
```

The plugin contract: dia sends the app spec as JSON on stdin. Exit code 0
means success; non-zero means error; stderr is surfaced in dia's log.

A reference implementation is provided at
`examples/plugins/dia-fake.sh`.

## Build from source

```sh
git clone https://github.com/DerekCorniello/dia.git
cd dia
make dev      # wails dev with hot reload
make build    # wails build
```

Requirements: Go 1.26+, Node 20+, and the Wails CLI:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

## Project layout

See [PLAN.md](./PLAN.md) for the full architecture and phased
implementation plan. The short version:

```
cmd/dia/             entry point
internal/config      YAML, validation, discovery
internal/runtime     instance lifecycle, PID tracking
internal/platform    OS-specific process launching
internal/apps        app-type registry, built-ins, plugins
internal/state       XDG paths, JSON state store
internal/cli         cobra commands
internal/wailsapp    bindings exposed to the Svelte UI
frontend/            Svelte + TypeScript + Vite + Tailwind
examples/            sample workspaces and a stub plugin
```

## Contributing

See [PLAN.md](./PLAN.md) for the implementation phases and code
conventions. The short version:

- No comments unless they explain why
- ASCII only in code, docs, and commits
- Hand-rolled validation with precise error messages
- Tests for non-trivial logic
- Prefer stdlib over a new dependency

## License

MIT. See [LICENSE](./LICENSE).
