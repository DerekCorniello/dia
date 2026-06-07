# dia

dia: a cross-platform desktop launcher for dev workspaces.

Define a workspace in YAML, click start, and dia brings up the editor,
terminal, browser, and services for that project. No window manager, no
process snapshot, just deterministic rebuilds from a config.

See [PLAN.md](./PLAN.md) for the full design and implementation plan.

## Status

v0.3.0 (unreleased). All v1 phases (0-6) are complete; the v1
plugin system landed in 0.3.0. See [CHANGELOG.md](./CHANGELOG.md)
for what shipped, and the "Known limitations" section below for
what is intentionally deferred to v1.1.

## Features

- JetBrains-style desktop launcher (Wails + Svelte)
- Workspace definitions in YAML
- Built-in app types: `local`, `editor`, `terminal`, `service`, `custom`
  (all spawn a command), `open` and `browser` (open a URL), `gh` and
  `gh:pr`/`gh:issue`/`gh:checkout`/`gh:repo-clone` (wrappers around
  the `gh` CLI)
- Both global and project-local configs with discovery
- In-process JS plugin system: drop a folder with a `plugin.json`
  and an `index.js` into `$XDG_STATE_HOME/dia/plugins/<id>/` (or
  `<cwd>/.dia/plugins/<id>/` for project-local). The GUI auto-wraps
  a panel from the manifest's `ui.type` (`list`/`grid`/`table`/
  `kv`/`text`); plugins call into dia through a capability-gated
  `dia.*` bridge.
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

## Plugins

dia's plugin system runs in two flavors:

- **Embedded panels** (`ui.type` of `list|grid|table|kv|text|canvas`):
  the GUI loads a plugin's `index.js` in a [goja](https://github.com/dop251/goja)
  interpreter and auto-wraps a panel from the plugin's manifest. The
  plugin author writes JS + a UI schema, not a frontend framework.
- **Window plugins** (`ui.type=window`): the host spawns a second
  dia process that opens a new OS-level window and serves the
  plugin's `panel/` folder. The plugin author writes plain
  HTML/CSS/JS - the host injects a `window.dia` proxy that
  dispatches to the host or to the plugin's goja runtime. No
  framework required.

### Plugin layout (embedded panels)

```
$HOME/.local/state/dia/plugins/hello-plugin/
  plugin.json
  index.js
  lib/                   # optional; require()-able from the entry
```

The manifest declares the plugin's id, name, capabilities, and a
UI schema. The entry script exports `getData()` to feed the
panel; the host renders the result.

### Scaffold a plugin

```sh
dia plugin new hello        # writes to the global plugins dir
dia plugin new hello --local  # writes to ./.dia/plugins/ in cwd
```

This generates a `plugin.json` and a starter `index.js` you can
edit. To build a window plugin, change `ui.type` to `"window"`
and add a `panel/` folder with `panel.js` (and optionally
`index.html` and `styles.css`); the whiteboard example is a
good starting point.

### Manifest

```json
{
  "id": "my-plugin",
  "name": "My Plugin",
  "version": "0.1.0",
  "description": "What it does",
  "author": "you",
  "entry": "index.js",
  "capabilities": ["workspaces:read"],
  "ui": {
    "type": "list",
    "title": "Things",
    "refreshable": true,
    "actions": [
      { "id": "open", "label": "Open", "capability": "workspaces:start" }
    ]
  }
}
```

| Field                       | Required | Notes                                              |
|-----------------------------|----------|----------------------------------------------------|
| `id`                        | yes      | `^[a-z0-9]([a-z0-9-]{1,38}[a-z0-9])?$`, no double-dashes, min 3 chars |
| `name`                      | yes      | 1-60 chars                                         |
| `version`                   | yes      | 1-32 chars                                         |
| `description`               | no       | 0-200 chars                                        |
| `author`                    | no       | 0-60 chars                                         |
| `entry`                     | no       | relative path; defaults to `index.js`              |
| `capabilities`              | no       | subset of the capability list (see below)          |
| `ui.type`                   | yes      | `list` \| `grid` \| `table` \| `kv` \| `text` \| `canvas` \| `window` |
| `ui.title`                  | yes      | panel title (and window title for `type=window`)   |
| `ui.entry`                  | window   | path to `panel.js` (default `panel/panel.js`)     |
| `ui.width`, `ui.height`     | window   | initial window size in px (default 900x700)       |
| `ui.refreshable`            | no       | show a refresh button                              |
| `ui.columns`                | table    | required when `type=table`; one entry per column  |
| `ui.column.format`          | no       | `badge` \| `duration` \| `text`                    |
| `ui.actions[]`              | no       | buttons in the panel header                        |
| `ui.action.id`              | yes      | passed to `onAction(id, ctx)`                      |
| `ui.action.label`           | yes      | button text                                        |
| `ui.action.confirm`         | no       | browser `confirm()` before invoking                |
| `ui.action.capability`      | no       | gate the call; empty string = no capability needed |

### UI types

`getData()` returns a value shaped to `ui.type`:

| Type      | Shape                                          | Host renders                                |
|-----------|------------------------------------------------|---------------------------------------------|
| `list`    | `[{ id, label, detail? }, ...]`                | vertical list                               |
| `grid`    | `[{ id, label, detail? }, ...]`                | card grid                                   |
| `table`   | `[{ col: value, ... }, ...]`                   | table; columns declared in `ui.columns`     |
| `kv`      | `{ key: value, ... }`                          | key/value list                              |
| `text`    | any string                                     | monospace block                             |
| `canvas`  | `{ strokes?, color, width }`                   | free-draw `<canvas>`. The host captures pointer events and passes `ctx.strokes` to actions |

### Entry

```js
module.exports = {
  // optional: getData is called when the panel mounts and on refresh
  getData: function () {
    return dia.listWorkspaces().map(function (w) {
      return { id: w.name, label: w.name };
    });
  },
  // optional: invoked when a ui.action is clicked
  onAction: function (id, ctx) {
    if (id === "open") {
      dia.startWorkspace(ctx.item.id);
    }
  }
};
```

### Capabilities

Read-only capabilities are granted by default at install time.
Mutating ones are opt-in and recorded in the persisted state.

| Capability          | Mutating | What it gates                          |
|---------------------|----------|----------------------------------------|
| `workspaces:read`   | no       | `dia.listWorkspaces()`, `dia.getWorkspace()` |
| `instances:read`    | no       | `dia.listInstances()`                  |
| `doctor:read`       | no       | `dia.doctor()`                         |
| `paths:read`        | no       | `dia.paths()`                          |
| `themes:read`       | no       | `dia.getTheme()`, `dia.listCustomThemes()` |
| `workspaces:start`  | yes      | `dia.startWorkspace()`                 |
| `instances:stop`    | yes      | `dia.stopInstance()`, `dia.stopAll()`  |
| `workspaces:create` | yes      | `dia.newWorkspace()`                   |
| `themes:write`      | yes      | `dia.setTheme()`, `dia.setCustomTheme()`, `dia.deleteCustomTheme()` |

Calling a method you don't have throws `capability "X" not granted`.
The host catches the error and surfaces it as a toast; the rest of
the plugin keeps running.

Grant capabilities explicitly:

```sh
dia plugin enable my-plugin --caps workspaces:read,workspaces:start
```

### `require()` and `module.exports`

`require('./lib/foo')` is plugin-scoped: relative paths only, no
`..` traversal, no absolute paths. Each required file is run with
its own `module`/`exports` and returns its `module.exports`. The
host caps each file at 1 MiB.

### Plugin directories

```
$XDG_STATE_HOME/dia/plugins/<id>/   # global; shared across projects
<cwd>/.dia/plugins/<id>/            # local; scoped to one project
```

On id collision the local copy wins. The CLI flag `--local` selects
the local dir for `new` and `list`.

### CLI

```sh
dia plugin new <id> [--local]
dia plugin list
dia plugin info <id>
dia plugin install <path> [--local]
dia plugin uninstall <id>
dia plugin enable <id> [--caps a,b,c]
dia plugin disable <id>
```

The GUI picks up enabled plugins on the next launch. Open Settings
> Plugins to toggle, see paths, and view the inline writing guide.

### Example: a black-pen whiteboard

`examples/whiteboard/` ships in this repo. It is a window plugin
that opens in its own OS-level window. The browser side is plain
HTML/CSS/JS in `panel/`; no Svelte, no build step.

Manifest (`plugin.json`):

```json
{
  "id": "whiteboard",
  "name": "Whiteboard",
  "version": "0.1.0",
  "entry": "index.js",
  "capabilities": [],
  "ui": {
    "type": "window",
    "title": "Whiteboard",
    "entry": "panel/panel.js",
    "width": 1100,
    "height": 750
  }
}
```

Layout:

```
whiteboard/
  plugin.json
  index.js           # loaded in goja (optional for window plugins)
  panel/
    panel.js         # browser-side: runs in the new window
    styles.css       # optional
```

`panel/panel.js` is plain browser JS. The host injects
`window.dia` with read-only host methods and a generic
`dia.call(method, args)` for any other method (including the
plugin's `module.exports` functions):

```js
(function () {
  // window.dia is ready before this script runs.
  window.dia.capabilities().then(function (caps) {
    console.log("host granted:", caps);
  });
})();
```

The `index.js` entry is loaded in goja by the host. For the
whiteboard it is a tiny stub; window plugins can omit the file
when no headless work is needed.

### Window plugins: how the spawn works

- The user enables a window plugin in Settings > Plugins and
  clicks "open window" on its panel in the main window.
- The main dia process calls `os.Executable()` and re-spawns
  the same binary with `--plugin-window=<id>`.
- The second process loads the plugin's `panel/panel.js` (or the
  `ui.entry` path) into a fresh wails window, generates a host
  `index.html` if the plugin does not ship one, and exposes
  `window.dia` as a wails binding back to Go.
- The plugin's `index.js` is loaded in goja so `module.exports`
  functions (e.g. `getData`, `onAction`) are reachable via
  `window.dia.call("getData")`.
- The new window process has no workspace runtime; mutating
  `dia.*` calls return an error. Read-only calls (`listWorkspaces`,
  `getTheme`, etc.) work against the shared state file.

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
internal/plugins       JS plugin host (goja + capability bridge)
internal/state         XDG paths, JSON state store
internal/diag          shared smoke checks (doctor)
internal/cli           cobra commands
internal/wailsapp      bindings exposed to the Svelte UI
frontend/              Svelte + TypeScript + Vite + Tailwind
examples/              sample workspaces and plugins
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
