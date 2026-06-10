# Changelog

All notable changes to dia are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [0.3.0] - Unreleased

### Added

- **In-process JS plugin host (v1).** Plugins are folders with a
  `plugin.json` manifest and an `index.js` entry script that run
  in a [goja](https://github.com/dop251/goja) interpreter in the
  GUI process. The host auto-wraps a panel from the manifest's
  `ui.type` (`list`/`grid`/`table`/`kv`/`text`/`canvas`); plugin
  authors ship JS + a UI schema, not a frontend framework.
- **Window plugins (`ui.type=window`).** A plugin can opt to run
  in its own OS-level window instead of an embedded panel. The
  main dia process re-spawns itself with `--plugin-window=<id>`;
  the second process serves the plugin's `panel/` folder, injects
  a `window.dia` proxy (read-only host methods + `dia.call()` for
  arbitrary method dispatch), and loads the plugin's `index.js`
  in goja so `module.exports` functions are reachable. Plugin
  authors ship plain HTML/CSS/JS in `panel/`; no Svelte, no
  build step.
- **Capability-gated `dia.*` bridge.** Plugins call into the host
  through a single `dia` object; each method requires a capability
  the user has explicitly granted. Read-only defaults include
  `workspaces:read`, `instances:read`, `doctor:read`, `paths:read`,
  `themes:read`; mutating capabilities (`workspaces:start`,
  `instances:stop`, `workspaces:create`, `themes:write`) are
  opt-in and recorded in the persisted state.
- **Local + global plugin directories.** Global plugins live in
  `$XDG_STATE_HOME/dia/plugins/<id>/`; project-local plugins live
  in `<cwd>/.dia/plugins/<id>/`. On id collision the local copy
  wins. CLI flag `--local` switches between the two for
  `plugin new` and `plugin list`.
- **Plugin CLI (`dia plugin ...`).** `new`, `list`, `install`,
  `uninstall`, `enable`, `disable`, `info` subcommands for
  authoring, discovery, and toggling persisted state.
- **Plugin state persistence.** `state.Data.Plugins` records
  each plugin's `Enabled` flag and the granted capability list
  so the GUI starts the right goja runtime on launch.
- **`examples/hello-plugin`** showing the manifest + entry shape
  for a `list` panel that reads workspaces.
- **`examples/whiteboard`** showing a `ui.type=window` plugin
  that opens a new window with a free-draw `<canvas>`.
- **Workspace plugins.** A workspace YAML can list plugin IDs
  under `plugins: [{ id: whiteboard }]`. Listed plugins are
  enabled when the workspace starts and disabled when it stops.
  Window-type plugins show an "open" button on the workspace card
  while the workspace is running.
- **Escape key closes new workspace dialog.** The `+ New` dialog
  now closes on Escape and on backdrop click, matching the
  Settings panel behavior.
- **Open Directory.** The header has an "Open" button that opens
  a directory picker. The selected directory is persisted in state
  and used for project-local workspace discovery (`.dia.yaml` and
  `./.dia/*.yaml`). A "Project:" bar shows the active directory
  with a clear button. When no project directory is set and no
  workspaces exist, a launcher view is shown with "Open Project"
  and "New Workspace" buttons plus a recent workspaces list.
- **Cross-process state visibility.** The GUI watches `state.json`
  with fsnotify (200ms debounce) and emits a
  `workspace:state-changed` Wails event. The frontend subscribes
  and auto-refreshes when the CLI modifies state. Manual Refresh
  is still available in the header.
- **Stop confirmation dialog.** Clicking "stop" on a running
  workspace card now shows a confirmation dialog before
  terminating apps.
- **Inline validation in NewWorkspaceDialog.** The name input
  validates as you type and shows errors next to the input field.
  The submit button is disabled when the name is invalid.
- **Reconcile button in Settings.** The Doctor tab has a
  "Reconcile" button that drops stale PIDs and shows the result
  as an inline toast.
- **Shell completions.** `dia completion [bash|zsh|fish|powershell]`
  generates shell completion scripts. Tab-completion works for
  commands, flags, and workspace names.
- **`start --dry-run`.** The `start` command accepts `--dry-run`
  to resolve and print what would launch without executing.
- **`open --json`.** The `open` command now supports `--json` for
  machine-readable output.
- **`$VISUAL` support in `edit`.** The `edit` command checks
  `$VISUAL` before `$EDITOR` when resolving the editor.
- **Aligned `stop --all` JSON shape.** `stop --all` now returns
  `{"stopped": ["id1", "id2"]}` (array of IDs), matching the
  per-workspace `stop <name>` output.
- **`errAlreadyExists` exit code.** Creating a workspace that
  already exists returns exit code 4 instead of 1.
- **Component tests.** Vitest + Svelte Testing Library tests for
  WorkspaceCard, NewWorkspaceDialog, and SettingsPanel. 11 tests
  covering rendering, validation, and state transitions.
- **`svelte-check` in CI.** The CI workflow runs `svelte-check`
  and `vitest` on every push to main and on pull requests.

### Changed

- **Theming polish.** Workspace cards have left-border accents,
  source pills with distinct styles for global vs local, running
  badges, and delete buttons in tinted red.
- **Plugins section visible when disabled.** All discovered plugins
  are shown, not just enabled ones. Disabled plugins render with
  muted styling.
- **Global keyboard shortcut handler uses a whitelist.** Only
  recognized app shortcuts prevent default browser behavior.
  Native shortcuts (Ctrl+C, Ctrl+V, Ctrl+A, etc.) pass through.
- **Keybindings are reactive and platform-aware.** The modifier
  key is detected at load time (Cmd on macOS, Ctrl elsewhere).
  Defaults recompute when the modifier changes.

### Removed

- **Unused Nunito font and logo assets.** The frontend uses Outfit
  via Google Fonts. The leftover `src/assets/` directory has been
  removed.

### Dependencies

- `github.com/dop251/goja` v0.0.0-20260603125802-cfe4039cb6d7
  (Go 1.20-compatible pseudo-version, pinned to keep CI on Go 1.23)
- `github.com/fsnotify/fsnotify` v1.10.1 added for cross-process
  state file watching.

## [0.2.0] - Unreleased

### Breaking changes

- **External plugin system removed.** dia no longer scans `PATH`
  for `dia-*` executables. Built-in app types are the only way to
  launch apps. Existing workspaces that used `type: plugin` or
  `type: <unknown>` (with an implicit `dia-<name>` lookup) will
  fail to start with `unknown app type "<name>"`. Drop the
  `plugin` field and use a built-in `type` or a `local` app with
  an absolute `cmd` instead.
- **CLI subcommand `dia plugins` removed.** Use `dia doctor` to
  verify your environment.
- **Config schema: `plugin` field removed.** The `plugin:` key
  in workspace apps is no longer accepted; old configs will fail
  to parse with `unknown field "plugin"`.

### Removed

- `internal/registry/plugins.go` and `PluginResolver` type
- `dia plugins` subcommand and `Plugins()` wails binding
- `examples/plugins/dia-fake.sh` (and the empty `examples/plugins/`
  directory)
- `case "plugin"` validator and `ScanPlugins` from `internal/diag`
- "Plugins" tab in the GUI Settings panel
- Plugin count from the GUI footer

## [0.1.0] - 2026-06-04

First end-to-end publishable v1 release. All six implementation
phases (0-6) are complete; see PLAN.md for the full breakdown.

### Added

**Phase 0 - Skeleton**
- Wails v2 + Svelte + TypeScript + Vite desktop app
- `internal/version` package with build-time `Version`, `Commit`,
  `BuildTime` vars (overridable via `-ldflags`)
- `internal/cli` cobra-based CLI skeleton with `--version`/`-V`
  and `--help`
- `Makefile` with `dev`, `build`, `test`, `vet`, `fmt`, `tidy`,
  `clean`, `install-tools` targets
- `.goreleaser.yaml` for cross-platform release packaging (per-OS
  wails build hooks + tar.gz/zip archives)
- GitHub Actions CI: `go test` + `go vet` + `gofmt` on linux,
  macos, windows
- GitHub Actions release: matrix wails build per OS/arch +
  GoReleaser + `softprops/action-gh-release` publish

**Phase 1 - Config + state**
- `internal/config`: workspace YAML schema, hand-rolled validator
  with precise per-field error messages, discovery from global
  (`$XDG_CONFIG_HOME/dia/workspaces/*.yaml`) and project-local
  (`.dia.yaml`, walked up from cwd) with project-local shadowing
- `internal/state`: XDG state-dir resolution, atomic JSON store
  with mutex, instance/recent/favorites shape

**Phase 2 - Platform + runtime**
- `internal/platform`: `Platform` interface + Linux/macOS/Windows
  impls; `Launch` uses `Setsid` (unix) / `CREATE_NEW_PROCESS_GROUP`
  (windows); `IsRunning` via `kill 0` / `tasklist`; `Kill` via
  SIGTERM-to-SIGKILL with 5s grace; `OpenURL` via `xdg-open` /
  `open` / `cmd /c start ""`; `RevealInFileManager` per OS
- `internal/runtime`: `Runtime` with concurrent per-app launch,
  state persistence, `Stop`/`StopAll`/`Reconcile`; 12-char base32
  instance IDs from `crypto/rand` (no UUID dep); integration test
  launches real `sleep` and verifies PID/kill

**Phase 3 - App-type registry + plugins**
- `internal/registry`: `Registry` mapping app types to launch
  actions; built-ins for `local`/`editor`/`terminal`/`service`/
  `custom` (type aliases of `local`), `open`, `browser`, `gh` +
  `gh:pr`/`gh:issue`/`gh:checkout`/`gh:repo-clone` sugars, and
  `plugin` with implicit `dia-<type>` fallback
- `PluginResolver` with cache; `NewPluginResolver()` (real PATH)
  and `NewPluginResolverAt(dirs)` (test isolation); rejects
  path-traversal in plugin names
- `examples/plugins/dia-fake.sh` reference plugin (POSIX sh)
- Plugin contract in v1: exec-only, no JSON-RPC/stdin protocol

**Phase 4 - CLI**
- Subcommands: `start`, `stop` (with `--force`/`--all`), `status`,
  `list` (alias `ls`), `new` (with `--local`), `edit`, `open`,
  `reconcile`, `doctor`, `plugins`
- Persistent flags: `--json`, `--state-dir`, `--version`/`-V`
- Exit codes: 0 ok, 1 general, 2 usage, 3 not found, 4 already
  exists (matches POSIX conventions for shell scripting)
- Per-command `--json` for machine-readable output

**Phase 5 - Wails app + Svelte UI**
- `internal/wailsapp`: 13 methods exposed to the frontend
  (`ListWorkspaces`, `GetWorkspace`, `StartWorkspace`,
  `StopInstance`, `StopAll`, `ListInstances`, `Reconcile`,
  `Plugins`, `Doctor`, `Paths`, `OpenConfigFolder`,
  `OpenStateFolder`, `NewWorkspace`); wails-bound types use
  snake_case JSON tags matching the generated TS models
- Svelte 4 + TS 5 + Vite 5 + Tailwind 3 (upgraded from the wails
  template defaults of Svelte 3, Vite 3, TS 4.6)
- Components: App (layout with header/footer/error banner),
  WorkspaceCard (expandable), InstanceRow, SettingsPanel,
  NewWorkspaceDialog; Svelte stores for state
- First-run empty state with inline "Create one" CTA
- Wails binding generator routes methods under
  `wailsjs/go/wailsapp/App` (see "Known limitations" below)

**Phase 6 - Polish**
- `internal/diag` package: shared smoke checks
  (`RunChecks`, `ScanPlugins`, `PlatformOpenHelper`) used by both
  CLI and GUI
- README: status, install, quickstart, app-types table, plugin
  contract, project layout, known limitations
- CHANGELOG (this file)
- `examples/sample-workspace.yaml`: ready-to-start workspace
- `examples/plugins/dia-fake.sh`: reference plugin implementation
- `goreleaser check` validates config (no deprecations)

### Known limitations (deferred to v1.1)

- **Cross-process state visibility.** Each dia process keeps its
  own `*state.Store` in memory. The GUI does not see changes made
  by the CLI in another process until the user clicks Refresh.
  Fix path: watch `$XDG_STATE_HOME/dia/state.json` with `fsnotify`
  and push updates via wails events. Deferred to avoid the
  `fsnotify` dependency in v1.
- **Wails binding package path.** The generator routes
  `*wailsapp.App` under `wailsjs/go/wailsapp/App` in TypeScript,
  not `wailsjs/go/main/App`. A `main.App` facade was tried first
  but the generator follows the return type's Go package. The
  Svelte frontend imports from `wailsjs/go/wailsapp/App`; a
  comment in `main.go` documents the choice.
- **GUI keyboard shortcuts** (`/` for search, `Enter` to start,
  `Ctrl+.` to stop, `Ctrl+N` for new). Deferred; not on the
  critical path for v1.
- **Slog output to a log file.** v1 logs to stderr only. A
  `$XDG_STATE_HOME/dia/dia.log` file is a v1.1 addition.
- **SBOM in release artifacts.** GoReleaser config does not
  generate one in v1; can be added in v1.1 by enabling the
  `sboms` section.

### Notes

- Wails dev hot-reload is supported via `make dev`.
- `go install github.com/DerekCorniello/dia@latest` works for Go
  users who do not need the GUI; the CLI surface is full.
- Prebuilt binaries for linux/amd64, linux/arm64,
  darwin/universal, and windows/amd64 ship in the GitHub Release
  attached to the v0.1.0 tag.
