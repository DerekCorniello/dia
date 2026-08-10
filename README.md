<p align="center">
  <img src="build/appicon.png" alt="dia" width="128" height="128" />
</p>

# dia

dia is a cross-platform desktop launcher for dev workspaces. Define a
workspace in YAML, click start, and dia brings up the editor, terminal,
browser, and services for that project. No window manager, no process
snapshot -- just deterministic rebuilds from a config.

dia is a desktop app first. The scriptable CLI is a convenience for
automation, not a rival interface.

## Status

v0.5.0 is released. This is the release that makes sessions real:
workspaces now survive closing the GUI. A headless daemon owns the
workspace runtime and keeps supervising your apps until `dia
shutdown`, so `start` attaches to what is already running and
`restart` replaces it. Earlier releases built the engine (v0.1.0),
theming + UI polish (v0.2.0), the JS plugin system (v0.3.0), and
plugin-provided app types, git installs, hooks, and GUI plugin
management (v0.4.0).

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
  `kv`/`text`/`canvas`) and full-window plugins (`window`) with plain
  HTML/CSS/JS
- Plugins can provide their own workspace app types, so `type:` is not
  limited to the built-ins
- Plugins install from a git repository, and update in place
- Lifecycle hooks (`pre_start`/`post_start`/`pre_stop`/`post_stop`)
- Session daemon: workspaces survive the GUI closing; `start` attaches
  to a running instance and `restart` replaces it
- Cross-platform: Linux, macOS, Windows
- Frameless window with custom title bar (no OS decoration)
- A scriptable CLI, for scripting and automation

## Install

### From source (requires Go 1.23+ and Node 20+)

The desktop app needs the Wails CLI and, on Linux, GTK/WebKit:

```sh
make install-tools   # Wails CLI (version taken from go.mod) + golangci-lint
make build           # writes build/bin/dia
make run             # build, then launch it
make install         # also installs a .desktop entry for the app launcher
```

The CLI is the same binary, so a plain `go install` gets you that too:

```sh
go install github.com/DerekCorniello/dia@latest
```

### Prebuilt binaries

Download the archive for your OS/arch from the
[GitHub Releases](https://github.com/DerekCorniello/dia/releases) page.

## Quickstart

Launch dia from your app launcher, or just run `dia` with no arguments.
You get a desktop launcher listing your workspaces, each with a
Start/Stop button per app. Click a card to expand it.

To add your first workspace, hit `+ New` in the header. That writes a
starter YAML to `~/.config/dia/workspaces/<name>.yaml` and opens it in
your editor. Edit it to your liking and it shows up in the grid.

From a terminal the same actions are available for scripting:

```sh
dia new myproject     # write a starter workspace
dia start myproject   # start it from the CLI
dia list              # what the GUI shows, as text
```

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

Plugins can add more app types; see
[Plugin-provided app types](#plugin-provided-app-types).

Project-local configs are also supported. Drop a `.dia.yaml` at the root of
your repo and dia will pick it up automatically.

## Hooks

Commands to run around a workspace's lifecycle. The usual case is
bringing up infrastructure the apps depend on:

```yaml
name: backend
hooks:
  pre_start:
    - docker compose up -d
  post_stop:
    - docker compose down
apps:
  - type: editor
    cmd: code .
  - type: browser
    url: http://localhost:8080
```

| Phase        | When it runs               | On failure                          |
|--------------|----------------------------|-------------------------------------|
| `pre_start`  | before any app launches    | fatal: nothing starts               |
| `post_start` | after all apps launch      | logged; the workspace stays up      |
| `pre_stop`   | before apps are terminated | logged; the stop continues          |
| `post_stop`  | after apps are terminated  | logged                              |

Rules:

- Each phase runs its commands **in order, one at a time**, blocking
  until each exits. The first failure ends that phase.
- Commands are parsed the same way as `cmd` (shell-style, quotes
  honored), but they are **not** run through a shell. Use
  `sh -c "..."` if you need pipes or globbing.
- Hooks run in the directory holding the workspace config. For a
  project-local `.dia.yaml` that is your repo root; for a global
  workspace it is the workspaces dir, so use absolute paths there.
- Each command is capped at 2 minutes. Long-running processes belong
  in `apps`, not hooks.
- `dia stop --force` skips `pre_stop` and `post_stop` entirely.

Only `pre_start` can block a workspace. The other phases wrap work
that has already happened, so a failure is reported and execution
continues -- a failing cleanup command should never leave you unable
to stop a workspace from dia.

There is deliberately no `depends_on` or per-app health checking. If
you need ordering between apps, `pre_start` is the supported way to
express it.

## Plugins

dia's plugin system runs in two flavors:

- **Embedded panels** (`ui.type` of `list|grid|table|kv|text|canvas`):
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
dia plugin new hello              # a GUI panel plugin (default)
dia plugin new compose --type=app # a plugin providing a workspace app type
dia plugin new hello --local      # writes to ./.dia/plugins/ in cwd
```

`--type=app` scaffolds a plugin that exports `resolveApp` and claims a
`type:` name workspaces can use. It requests the mutating
`apps:resolve` capability, so grant it before the type resolves:

```sh
dia plugin grant compose --caps apps:resolve
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
| `app_types`                 | no       | workspace app types this plugin provides; requires the `apps:resolve` capability |
| `ui.type`                   | yes      | `list` \| `grid` \| `table` \| `kv` \| `text` \| `canvas` \| `window` |
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
| `canvas`  | `{ color?, width? }`, not drawn                | free-draw canvas; see below                 |
| `window`  | n/a (`getData` is never called)                | separate OS window with `panel/panel.js`    |

### `canvas` panels

A `canvas` panel renders a free-draw surface with a built-in clear
button and a stroke counter. The panel still calls `getData` on load
and on refresh, so the entry must export it or the panel shows
`plugin does not export "getData"`. The returned value is not drawn,
but its optional `color` and `width` fields set the pen used for the
*next* stroke the user draws -- a plugin typically sources these from
`config_schema` (via `dia.getConfig()`) so they are configurable per
workspace. Strokes themselves are produced by the user drawing, so the
panel owns the drawing state and the plugin observes it through
actions.

Each action's `ctx` carries the current strokes:

```js
type Stroke = { color: string, width: number, points: [{ x, y }, ...] }
```

`onAction(id, ctx)` receives `ctx.strokes`. To replace what is on the
canvas (undo, clear, load a saved sketch), return an object with a
`strokes` array; the host swaps it in and repaints. Return nothing to
leave the canvas untouched.

```js
module.exports = {
  getData: function () { return null; },
  onAction: function (id, ctx) {
    if (id === "undo") {
      return { strokes: ctx.strokes.slice(0, -1) };
    }
    if (id === "save") {
      dia.exec("sh", ["-c", "echo saved"]);
    }
  }
};
```

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
Mutating ones are never granted by requesting them: the manifest only
declares what a plugin *wants*, and the user has to approve each one
explicitly with `dia plugin grant --caps`.

Grants are shared between the GUI and the CLI: Settings > Plugins
lists each plugin's requested capabilities and lets you toggle them,
and either side sees what the other granted. Changes apply
immediately -- no restart needed.

Settings > Plugins can also install from a git repository, update a
git-installed plugin, and uninstall one, so the CLI is not required
for any of it. Installing shows the plugin's identity, the app types
it would claim, and its requested capabilities before any code is
copied; nothing mutating is granted by installing.

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
| `apps:resolve`      | yes      | claiming app types via `app_types`     |

Calling a method you don't have throws `capability "X" not granted`.
The host catches the error and surfaces it as a toast; the rest of
the plugin keeps running.

Grant capabilities explicitly:

```sh
dia plugin grant my-plugin --caps workspaces:read,workspaces:start
```

### Plugin-provided app types

A plugin can add its own workspace app types, so `type:` is not
limited to what dia ships. Declare them in the manifest and export
`resolveApp`:

```json
{
  "id": "compose-app-type",
  "name": "Docker Compose App Type",
  "version": "0.1.0",
  "capabilities": ["apps:resolve"],
  "app_types": ["compose", "compose:down"],
  "ui": { "type": "kv", "title": "Compose App Type" }
}
```

```js
module.exports = {
  resolveApp: function (app) {
    return {
      cmd: "docker compose",
      args: ["up", "-d"].concat(app.args || []),
      cwd: app.cwd,
      env: app.env,
    };
  },
};
```

Then in a workspace:

```yaml
apps:
  - type: compose
    cwd: ~/projects/api
```

`resolveApp(app)` receives the workspace entry exactly as written
(`type`, `label`, `cmd`, `args`, `cwd`, `env`, `url`) and returns
**either** a command or a URL, never both:

| Key    | With   | Meaning                                     |
|--------|--------|---------------------------------------------|
| `cmd`  | launch | program to run; parsed like a workspace `cmd` (quotes honored) |
| `args` | launch | appended to whatever `cmd` already contained |
| `cwd`  | launch | working directory; falls back to the app's `cwd` |
| `env`  | launch | map of string to string, added to the environment |
| `url`  | open   | opened in the OS default handler; cannot be combined with the above |

Any other key is an error, so a typo like `command` is reported
rather than silently ignored.

**The resolver is pure.** It runs in a restricted runtime where the
only available calls are `dia.getConfig()` and `dia.pluginDir()` --
no workspaces, no `exec`, no `fetch`. This keeps `dia start --dry-run`
side-effect free, lets the CLI resolve app types without loading any
plugin UI, and stops a resolver from making a network call in the
middle of a start. `require()` works as usual, so shared logic can
live in `lib/`.

Rules worth knowing:

- `apps:resolve` is **mutating** and never granted by default. The
  command a resolver returns is executed by dia, which makes it
  `cmd:exec` by another route.
- A plugin cannot claim a built-in type (`editor`, `browser`, `gh:pr`,
  and so on). The claim is refused and the built-in keeps working.
- If two plugins claim the same type, the first wins deterministically
  (project-local before global, then by plugin id) and `dia doctor`
  reports the conflict.
- Both the GUI and the CLI honor plugin types, so `dia start` works
  the same either way.

`dia start --dry-run` resolves every app and shows what would run,
including which plugin resolved each non-builtin type:

```
api (dry run)
  hook pre_start    docker compose up -d
  launch compose    docker compose up -d  (via plugin compose-app-type)
  open   browser    http://localhost:8080
```

See `examples/compose-app-type/` for the full plugin.

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
dia plugin install <path|url> [--local] [--ref <branch|tag>] [--yes]
dia plugin update <id>
dia plugin uninstall <id>
dia plugin grant <id> [--caps a,b,c]
```

#### Installing from a git repository

`dia plugin install` takes either a local directory or a git remote:

```sh
dia plugin install ./my-plugin                       # local directory
dia plugin install github.com/user/dia-plugin        # normalized to https
dia plugin install https://github.com/user/plugin    # explicit URL
dia plugin install git@github.com:user/plugin.git    # scp-style
dia plugin install github.com/user/plugin --ref v1.2.0
```

Remotes are shallow-cloned with the system `git`, so existing
credential helpers and SSH config apply, and `.git` is not copied
into the plugins dir. A bare `owner/repo` is rejected on purpose: it
cannot be told apart from a relative path.

Installing a plugin puts code on your machine that dia will run, so
the requested capabilities are printed before anything is copied, and
confirmation is required when any of them are mutating. Use `--yes`
to skip the prompt in scripts; `--json` requires it.

The clone URL and ref are recorded next to the plugin, so:

```sh
dia plugin update my-plugin
```

re-clones it in place, preserving your granted capabilities. A plugin
installed from a local path has no recorded source and cannot be
updated this way.

The GUI picks up plugins on the next launch. Open Settings
> Plugins to grant capabilities, install from a folder, see paths, and
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

Window plugins are invoked by the workspace that owns them, not as
free-floating windows.

- A workspace lists a window plugin under `plugins:` with an optional
  per-workspace `config` map. When you expand a workspace in the main
  window, an "open window" action appears next to each attached window
  plugin; editing the workspace lets you attach or remove plugins.
- Starting the workspace spawns each attached window plugin
  automatically. The main dia process calls `os.Executable()` and
  re-spawns the same binary with `--plugin-window=<id>`,
  `--workspace=<name>`, and `--workspace-path=<path>`.
- The second process reads the workspace's config for that plugin,
  loads the plugin's `panel/panel.js` (or the `ui.entry` path) into a
  fresh wails window, generates a host `index.html` if the plugin does
  not ship one, and exposes `window.dia` as a wails binding back to Go.
- The plugin's `index.js` is loaded in goja so `module.exports`
  functions (e.g. `getData`, `onAction`) are reachable via
  `window.dia.call("getData")`.
- The window process opens with the plugin's persisted capability
  grants (the read-only defaults unless the user approved more), so a
  window plugin can `exec`, `fetch`, or `startWorkspace` only when the
  user granted them. Stopping the workspace kills that plugin's window
  process.

### Example plugins

#### Sketch Pad

The simplest example of the embedded `canvas` panel type: a free-draw
surface with a config-driven default pen color/width and an Undo
action. Unlike the other examples below, this one is not a window
plugin -- no `panel/` folder, just a manifest and an entry script.

```
examples/sketch-pad/
  plugin.json
  index.js
```

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

#### Time Tracker

A window plugin that logs work sessions by name. Start/stop from
the window; finished sessions land in localStorage and roll up into
today/this-week totals. No capabilities needed -- it never touches
the bridge, so read-only grants are all it gets. Export writes the
session log to a CSV download.

```
examples/time-tracker/
  plugin.json
  index.js
  panel/
    panel.js
```

#### API Playground

A window plugin that fires HTTP requests against an API and shows
the response: method, URL, headers, and body. Requests can be saved
as templates in localStorage and replayed. Unlike the other window
plugins it needs the mutating `fetch` capability, so approve it
when granting this one (`dia plugin grant api-playground --caps
fetch`). The response body is parsed as JSON when possible.

```
examples/api-playground/
  plugin.json
  index.js
  panel/
    panel.js
```

## CLI (scripting)

The same binary serves as a CLI for automation. It exposes what the
GUI does, for scripts, cron, and terminals:

```sh
dia                     # opens the GUI
dia list                # list all workspaces
dia list --json         # machine-readable output
dia start <name>        # start a workspace (attach if already running)
dia start <name> --dry-run  # resolve and print without launching
dia restart <name>      # stop if running, then start a fresh instance
dia stop <name>         # stop a workspace
dia stop --all          # stop every running workspace
dia status              # running instances and PIDs
dia reconcile           # drop PIDs from state that are no longer running
dia serve               # run the session daemon in the foreground
dia shutdown            # stop the daemon and everything it supervises
dia new <name>          # create a starter workspace
dia edit <name>         # open the config in $VISUAL or $EDITOR
dia open <name>         # reveal the workspace in the file manager
dia open <name> --json  # machine-readable output
dia doctor              # smoke checks
dia plugin list         # list installed plugins
dia plugin install <path|url>  # install from a directory or git repo
dia plugin update <id>  # re-clone a git-installed plugin
dia completion bash     # generate shell completion (bash/zsh/fish/powershell)
dia --version           # print version and exit
```

All list/status/doctor commands support `--json` for machine-readable
output.

### Sessions

Like tmux, a headless daemon owns every running workspace. The first
`dia start`, `stop`, `status`, `reconcile`, or GUI session spawns the
daemon; it survives the client and keeps supervising your apps until
`dia shutdown` (or you quit the GUI -- the daemon runs on). `start` is
idempotent: starting a name that is already running attaches to the
existing instance rather than launching a duplicate. There is no
auto-respawn -- when an app dies it stays dead until you run
`dia restart <name>`.

`dia serve` runs the daemon in the foreground (logs to
`$XDG_STATE_HOME/dia/dia.log`) for service managers; normally the
daemon spawns itself in the background on first use.

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
main.go                Wails entrypoint; routes GUI vs CLI vs serve
internal/config        YAML, validation, discovery
internal/runtime       instance lifecycle, PID tracking, reconcile
internal/daemon        session daemon: owns the runtime, socket server + client
internal/platform      OS-specific process launching
internal/registry      app-type registry and built-ins
internal/plugins       JS plugin host (goja + capability bridge)
internal/state         XDG paths, JSON state store
internal/diag          shared smoke checks (doctor)
internal/cli           cobra commands (lifecycle verbs are daemon clients)
internal/wailsapp      bindings exposed to the Svelte UI (also a daemon client)
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
