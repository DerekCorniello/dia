# dia

dia: a cross-platform desktop launcher for dev workspaces.

Define a workspace in YAML, click start, and dia brings up the editor,
terminal, browser, and services for that project. No window manager, no
process snapshot, just deterministic rebuilds from a config.

See [PLAN.md](./PLAN.md) for the full design and implementation plan.

## Status

v0.1.0. All v1 phases (0-6) are complete. See [CHANGELOG.md](./CHANGELOG.md)
for what shipped, and the "Known limitations" section below for what
is intentionally deferred to v1.1.

## Features

- JetBrains-style desktop launcher (Wails + Svelte)
- Workspace definitions in YAML
- Built-in app types: `local`, `editor`, `terminal`, `service`, `custom`
  (all spawn a command), `open` and `browser` (open a URL), `gh` and
  `gh:pr`/`gh:issue`/`gh:checkout`/`gh:repo-clone` (wrappers around
  the `gh` CLI)
- Both global and project-local configs with discovery
- Cross-platform: Linux, macOS, Windows
- Scriptable CLI alongside the GUI

## Install

### From source (requires Go 1.23+ and Node 20+)

```sh
go install github.com/DerekCorniello/dia@latest
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

The GUI shows the same workspaces as `dia list`, with a Start/Stop button
per card. Click a card to expand and see its apps.

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

More examples live in `examples/`.

## App types

| Type            | Required fields          | What it does                                              |
|-----------------|--------------------------|-----------------------------------------------------------|
| (default)       | `cmd`                    | Runs the program. `Cmd` may be a single program name or a shell-style string with arguments. `args` is appended. |
| `local`         | `cmd`                    | Same as default.                                          |
| `editor`        | `cmd`                    | Label for `local`; renders with an editor icon.           |
| `terminal`      | `cmd`                    | Label for `local`; renders with a terminal icon.          |
| `service`       | `cmd`                    | Label for `local`; renders with a service icon.           |
| `custom`        | `cmd`                    | Label for `local`; renders with a generic icon.           |
| `open`          | `url`                    | Opens the URL in the OS default handler.                  |
| `browser`       | `url` (http/https)        | Opens the URL in the default browser.                     |
| `gh`            | `cmd` (subcommand)       | Runs `gh <cmd> <args...>`.                                |
| `gh:pr`         | -                        | Runs `gh pr <args...>`.                                   |
| `gh:issue`      | -                        | Runs `gh issue <args...>`.                                |
| `gh:checkout`   | -                        | Runs `gh checkout <args...>`.                             |
| `gh:repo-clone` | `url`                    | Runs `gh repo clone <url> [cwd]`.                         |

All launch types accept `cwd` (path, `~` and `$VAR` expanded) and `env`
(map of string to string).

Project-local configs are also supported. Drop a `.dia.yaml` at the root of
your repo and dia will pick it up automatically.

## CLI

```sh
dia                     # opens the GUI
dia list                # list all workspaces
dia list --json         # machine-readable output
dia start <name>        # start a workspace
dia stop <name>         # stop a workspace
dia stop --all          # stop every running workspace
dia status              # running instances and PIDs
dia new <name>          # create a starter workspace
dia edit <name>         # open the config in $EDITOR
dia open <name>         # reveal the workspace in the file manager
dia reconcile           # drop PIDs from state that are no longer running
dia doctor              # smoke checks
dia --version           # print version and exit
```

All list/status/doctor commands support `--json` for machine-readable
output.

## Build from source

```sh
git clone https://github.com/DerekCorniello/dia.git
cd dia
make dev      # wails dev with hot reload
make build    # wails build
```

Requirements: Go 1.23+, Node 20+, and the Wails CLI:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

## Project layout

See [PLAN.md](./PLAN.md) for the full architecture and phased
implementation plan. The short version:

```
main.go                Wails entrypoint; routes GUI vs CLI
internal/config        YAML, validation, discovery
internal/runtime       instance lifecycle, PID tracking
internal/platform      OS-specific process launching
internal/registry      app-type registry and built-ins
internal/state         XDG paths, JSON state store
internal/diag          shared smoke checks (doctor)
internal/cli           cobra commands
internal/wailsapp      bindings exposed to the Svelte UI
frontend/              Svelte + TypeScript + Vite + Tailwind
examples/              sample workspaces
```

## Known limitations (deferred to v1.1)

These are known gaps in the v1 release. Each is documented in PLAN.md
and the source has a `TODO` marker where the fix would land.

- **Cross-process state visibility.** Each dia process keeps its
  own `*state.Store` in memory. If you run `dia start foo` from the
  CLI while the GUI is open, the GUI will not see the new instance
  until you click the Refresh button. Fix path: watch
  `$XDG_STATE_HOME/dia/state.json` with `fsnotify` and re-Snapshot
  the runtime on change, then push the update to the Svelte UI as
  a wails event. Deferred because it adds an `fsnotify` dependency
  and pushes the runtime API toward a multi-reader design we
  don't need yet.

- **Wails binding package path.** The wails binding generator
  routes the Go-side `*wailsapp.App` under `wailsjs/go/wailsapp/App`
  in TypeScript, not `wailsjs/go/main/App` (a thin `main.App` facade
  was tried first; the generator still follows the return type's
  package). The Svelte frontend imports from
  `wailsjs/go/wailsapp/App`. This is documented in `main.go` near
  the binding call.

See [PLAN.md](./PLAN.md#what-dia-is-not) for the full out-of-scope
list (window positioning, sleep/resume, marketplace, etc.).

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
