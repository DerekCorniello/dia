# dia

dia: a cross-platform desktop launcher for dev workspaces.

Define a workspace in YAML, click start, and dia brings up the editor,
terminal, browser, and services for that project. No window manager, no
process snapshot, just deterministic rebuilds from a config.

## Status

v0.3.0 (unreleased). All planned phases are complete:
core engine (v0.1.0), theming + UI polish (v0.2.0), JS plugin system
(v0.3.0) with embedded and window-style plugins, AI tool detection,
frameless window mode, and a custom title bar.

## Features

- JetBrains-style desktop launcher (Wails + Svelte)
- Workspace definitions in YAML
- Built-in app types: `local`, `editor`, `terminal`, `service`, `custom`
  (all spawn a command), `open` and `browser` (open a URL), `gh` and
  `gh:pr`/`gh:issue`/`gh:checkout`/`gh:repo-clone` (wrappers around
  the `gh` CLI)
- AI tool detection: claude, copilot, cursor, windsurf, t3code, codex
- Both global and project-local configs with discovery
- In-process JS plugin system: embedded panels (`list`/`grid`/`table`/
  `kv`/`text`) and full-window plugins (`window`) with plain HTML/CSS/JS
- Cross-platform: Linux, macOS, Windows
- Frameless window with custom title bar (no OS decoration)
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
dia start <name> --dry-run  # resolve and print without launching
dia stop <name>         # stop a workspace
dia stop --all          # stop every running workspace
dia status              # running instances and PIDs
dia new <name>          # create a starter workspace
dia edit <name>         # open the config in $VISUAL or $EDITOR
dia open <name>         # reveal the workspace in the file manager
dia open <name> --json  # machine-readable output
dia reconcile           # drop PIDs from state that are no longer running
dia doctor              # smoke checks
dia plugin list         # list installed plugins
dia completion bash     # generate shell completion (bash/zsh/fish/powershell)
dia --version           # print version and exit
```

All list/status/doctor commands support `--json` for machine-readable
output.

## Plugins

dia's plugin system runs in two flavors:

- **Embedded panels** (`ui.type` of `list|grid|table|kv|text`):
  the GUI loads a plugin's `index.js` in a [goja](https://github.com/dop251/goja)
  interpreter and auto-wraps a panel from the plugin's manifest. The
  plugin author writes JS + a UI schema, not a frontend framework.
- **Window plugins** (`ui.type=window`): the host spawns a second
  dia process that opens a new OS-level window and serves the
  plugin's `panel/` folder. The plugin author writes plain
  HTML/CSS/JS -- the host injects a `window.dia` proxy that
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
`index.html` and `styles.css`); the examples in `examples/` are
good starting points.

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
| `ui.type`                   | yes      | `list` \| `grid` \| `table` \| `kv` \| `text` \| `window` |
| `ui.title`                  | yes      | panel title (and window title for `type=window`)   |
| `ui.entry`                  | window   | path to `panel.js` (default `panel/panel.js`)      |
| `ui.width`, `ui.height`     | window   | initial window size in px (default 900x700)        |
| `ui.refreshable`            | no       | show a refresh button                              |
| `ui.columns`                | table    | required when `type=table`; one entry per column   |
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
| `window`  | n/a (no `getData`)                             | separate OS window with `panel/panel.js`    |

### Entry

```js
module.exports = {
  getData: function () {
    return dia.listWorkspaces().map(function (w) {
      return { id: w.name, label: w.name };
    });
  },
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
| `cmd:exec`          | yes      | `dia.exec(cmd, args)`                  |
| `fetch`             | yes      | `dia.fetch(url, opts)`                 |

Calling a method you don't have throws `capability "X" not granted`.
The host catches the error and surfaces it as a toast; the rest of
the plugin keeps running.

Grant capabilities explicitly:

```sh
dia plugin enable my-plugin --caps workspaces:read,workspaces:start
```

### `require()` and `module.exports`

`require('./lib/foo')` is plugin-scoped: relative paths only, no
`..` traversal, no absolute paths. A bare path resolves to its
`.js` file (`./lib/foo` loads `lib/foo.js`). Each required file runs
in its own scope with a fresh `module`/`exports` and returns its
`module.exports`. Modules are cached per plugin instance by resolved
path, so a file executes at most once and repeated requires return the
same exports. The host caps each file at 1 MiB.

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
> Plugins to enable/disable, install from a folder, see paths, and
view the inline writing guide.

### `dia.exec(cmd, args)`

Window plugins can run arbitrary shell commands via the `cmd:exec`
capability. Returns stdout as a string:

```js
dia.exec('gh', ['pr', 'list', '--json', 'number,title,state'])
  .then(function (output) {
    var prs = JSON.parse(output);
    // ...
  });
```

### `dia.fetch(url, opts)`

Window plugins can make HTTP requests via the `fetch` capability.
Returns an object with `ok`, `status`, `headers`, `body` (string):

```js
dia.fetch('https://api.github.com/repos/owner/repo/issues', {
  headers: { Authorization: 'Bearer ' + token },
}).then(function (resp) {
  if (resp.ok) {
    var data = JSON.parse(resp.body);
  }
});
```

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

### Example plugins

#### gh-dashboard

A window plugin that provides a GitHub dashboard with four tabs:
Pull Requests, Issues, Actions, and Projects. Data is fetched via
`dia.exec('gh', ...)` -- no PAT or API client required.

```
examples/gh-dashboard/
  plugin.json
  index.js
  panel/
    panel.js
```

#### Whiteboard

A window plugin with a free-draw canvas. Supports color/width
configuration, undo/redo history, eraser mode, and PNG export.

```
examples/whiteboard/
  plugin.json
  index.js
  panel/
    panel.js
    styles.css
```

#### Pomodoro

A window plugin implementing a pomodoro timer with focus, short
break, and long break cycles, configurable durations, desktop
notifications, and start/pause/reset controls.

```
examples/pomodoro/
  plugin.json
  index.js
  panel/
    panel.js
```

#### Quick Notes

A window plugin providing a plain-text scratchpad with
localStorage persistence, word-wrap toggle, font size
configuration, export-to-file, and clear.

```
examples/quick-notes/
  plugin.json
  index.js
  panel/
    panel.js
```

## AI tool detection

dia detects AI coding tools installed on the system and makes them
available in the Quick add panel. Detected tools:

- claude (Anthropic Code)
- copilot / github-copilot-app (GitHub Copilot CLI)
- cursor (Cursor editor)
- windsurf (Windsurf / Codeium)
- t3code (T3 Chat)
- codex (Codex CLI)

Detection is automatic on startup. Tools are listed in the GUI's
Quick add section under the "AI" category.

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

## Known limitations

- **Wails binding package path.** The wails binding generator
  routes the Go-side `*wailsapp.App` under `wailsjs/go/wailsapp/App`
  in TypeScript, not `wailsjs/go/main/App` (a thin `main.App` facade
  was tried first; the generator still follows the return type's
  package). The Svelte frontend imports from
  `wailsjs/go/wailsapp/App`. This is documented in `main.go` near
  the binding call.

Out of scope: window positioning, sleep/resume, workspace
templates marketplace, TUI mode, log rotation, JSON Schema
validation, Homebrew/Scoop/apt distribution, partial
workspace launch.

## Contributing

See [AGENTS.md](./AGENTS.md) for the layout, build/test/lint commands,
and conventions. In short: run `make check` before pushing, keep it
ASCII-only, comment only the *why*, and prefer the stdlib over a new
dependency.

## License

MIT. See [LICENSE](./LICENSE).
