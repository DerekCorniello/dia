# dia - Project Plan

## Locked Decisions

Confirmed during planning; referenced throughout this doc.

- Module path: `github.com/DerekCorniello/dia` (matches the remote `git@github.com:DerekCorniello/dia.git`)
- Binary name: `dia`
- UI: Wails v2 desktop app (Svelte 4 + TypeScript 5 + Vite 5 + Tailwind 3)
- Plugin model: built-in app-type registry + external `dia-*` executables on `PATH`; in v1 plugins are exec-only (no JSON-RPC/stdin protocol); upgrade is a v1.1 candidate, not a v1 must-have
- GitHub integration: thin wrapper around the `gh` CLI (no separate auth in dia)
- Config locations: global (`~/.config/dia/workspaces/*.yaml`) and project-local (`.dia.yaml`), with discovery; project-local shadows global on name collision
- State store: JSON file in XDG state dir (`~/.local/state/dia/state.json`)
- Distribution: GitHub Releases only; Homebrew/Scoop deferred
- Config schema validation: hand-written Go validator with precise error messages
- ASCII only in code, docs, and commits
- Wails binding generator routes `*wailsapp.App` under `wailsjs/go/wailsapp/App`; we bind the implementation type directly from `main.go` (a `main.App` facade was tried first but the generator follows the return type's package)

## Overview

dia is a cross-platform workspace launcher that lets users define, start, and
manage development sessions composed of multiple applications (editors,
terminals, browsers, services) using declarative configs.

Instead of managing windows directly like a window manager, dia recreates
environments on demand in a predictable way.

## Core Philosophy

- Not a window manager
- Not a process snapshot tool
- A workspace orchestrator
- tmux UX, but at the system/app level
- Deterministic rebuild beats fragile state saving

## Status

**v0.1.0 released** (2026-06-04) -- all six original implementation phases (0-6)
complete. See CHANGELOG for details.

The phases below describe the v0.2.0 roadmap: user-facing polish, theming,
workspace organization, cross-process state, CLI hardening, and test coverage.

## Stack

### Backend
- Go 1.23+ (go.mod declares `go 1.23`; toolchain on dev box is 1.26)
- Wails v2 (desktop runtime; native webview, ~10MB binary)
- `spf13/cobra` (CLI subcommands + shell completion generation)
- `gopkg.in/yaml.v3` (config parsing)
- stdlib `log/slog` (structured logging)
- stdlib `crypto/rand` + base32 (instance IDs; no UUID dep)

### Frontend
- Svelte 4 + TypeScript 5 + Vite 5 + Tailwind 3
- daisyUI v4 (Tailwind CSS component library + theme system)

### CI / Release
- GitHub Actions matrix on linux, macos, windows
- GoReleaser publishing archives + checksum on tag push
- `go install github.com/DerekCorniello/dia@latest` for Go users

### Dep justification (per project practice: minimize deps)
- `wails.io/wails/v2` -- required for the desktop app
- `spf13/cobra` -- industry-standard multi-command Go CLI (gh, kubectl, docker all use it); also provides shell completion generation for free
- `gopkg.in/yaml.v3` -- required for YAML configs
- `daisyui` -- Tailwind plugin providing 30+ themes, component classes, and CSS-based theme switching; zero JS runtime, ~2KB per theme gzipped
- `fsnotify` -- added in Phase 4 for cross-process state watching; mature, stdlib-adjacent, no cgo
- No `adrg/xdg` -- stdlib `os.UserConfigDir()` plus a small XDG_STATE_HOME resolver is enough
- No `google/uuid` -- 12-char base32 from `crypto/rand` is enough
- No `golang.org/x/term` -- not needed
- No `golangci-lint` -- `go vet` + `gofmt -l` in CI is sufficient

## Module and Binary

- Module path: `github.com/DerekCorniello/dia`
- Binary name: `dia`
- Entry point: `main.go` (at the repo root, not under `cmd/dia/`)
- Routing: `dia` with no args opens the GUI; `dia <subcmd>` runs the CLI

## Repository Layout

```
dia/
  main.go                        Wails entrypoint; routes GUI vs CLI; binds *wailsapp.App
  internal/
    config/                      YAML loader, types, validator, discovery
    runtime/                     Instance, AppInstance, lifecycle, PID tracking
    platform/                    Platform interface + linux/darwin/windows impls
    registry/                    App-type registry, built-ins, gh wrappers, plugins
    state/                       XDG paths, atomic JSON store
    diag/                        Shared smoke checks (doctor, plugin scan)
    cli/                         cobra commands
    wailsapp/                    Methods exposed to the Svelte UI; bound directly by main.go
    version/                     Build-time version vars (ldflags)
  frontend/                      Svelte UI (wails-managed)
  examples/                      Sample workspace YAMLs and stub dia-* plugin
  .goreleaser.yaml
  wails.json
  go.mod / go.sum
  Makefile
  README.md / CHANGELOG.md / LICENSE / PLAN.md
  .github/workflows/ci.yml
  .github/workflows/release.yml
```

## Core Concepts

### Workspace

A declarative environment definition:

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

### App Spec

```go
type App struct {
    Type   string            // editor, terminal, browser, service, custom, plugin name
    Cmd    string            // command to run
    Args   []string          // optional argv
    Cwd    string            // working directory (tilde and env vars expanded)
    Env    map[string]string // extra env vars
    Url    string            // for browser apps
    Open   bool              // for editor apps: open the project, not a file
    Wait   bool              // if true, dia waits for this app to exit before starting the next
    Plugin string            // explicit plugin name (rare; type is usually enough)
}
```

### App Type Categories

Built-in categories:

- editor
- terminal
- browser
- service (run cmd, track PID, no window)
- custom (raw `exec.Command`)

Built-in launchers (best match per platform, all optional with warnings if missing):

- editor: vscode, vscodium, cursor, sublime, vim, nvim, emacs, idea, goland, fleet
- terminal: gnome-terminal, konsole, foot, alacritty, kitty, wezterm, iterm2, windows-terminal, Terminal.app
- browser: chrome, chromium, firefox, safari, edge, brave (fallback: `xdg-open` / `open` / `start`)
- service: any `cmd` (e.g. `docker compose up`)
- custom: any `cmd` the user wants

Third-party plugin launchers:

- Discovered at startup by scanning `PATH` for executables matching `dia-*`
- Registered as `type: <name-without-dia-prefix>` (e.g. `dia-foo` registers as `type: foo`)
- Contract: dia invokes the plugin with the app spec as JSON on stdin
  - Exit code 0 means success
  - Non-zero exit means error
  - Plugin stderr is surfaced in dia's log
- Reference implementation: `examples/plugins/dia-fake.sh`

GitHub wrappers (thin shells around `gh`):

- `type: gh:pr` -- open current branch's PR in browser
- `type: gh:issue` -- open repo issues page
- `type: gh:repo-clone` -- clone a repo and open a workspace for it
- `type: gh:checkout` -- check out a branch or PR

No auth in dia; the user logs into `gh` once and dia inherits the credentials.

### Workspace Lifecycle

1. Load config (global + project-local, merged)
2. Validate schema
3. Resolve paths, env vars, tildes
4. Resolve each app to a launcher (built-in, plugin, or custom)
5. Spawn processes concurrently (or sequentially for `wait: true`)
6. Track PIDs in state
7. Allow stop / restart / status
8. On crash recovery, scan state, drop dead PIDs, keep last-known status

## Platform Layer

### Interface

```go
type Platform interface {
    Launch(opts LaunchOpts) (ProcessHandle, error)
    OpenURL(url string) error
    IsRunning(pid int) (bool, error)
    Kill(pid int, force bool) error
    RevealInFileManager(path string) error
}

type ProcessHandle interface {
    PID() int
}
```

### Implementations

- `platform_linux.go` -- `os/exec` with `Setsid: true` for new process group; `syscall.Kill` for stop
- `platform_darwin.go` -- same as linux
- `platform_windows.go` -- `CREATE_NEW_PROCESS_GROUP`; cleanup via `taskkill /T /F` (best-effort tree kill; job objects deferred)

## Runtime Layer

Responsibilities:

- Spawning apps
- Tracking PIDs
- Grouping processes into instances
- Restart / stop / status logic
- Crash recovery on dia startup

Key types:

```go
type Instance struct {
    ID             string         // 12-char base32 from crypto/rand
    WorkspaceName  string
    StartedAt      time.Time
    Apps           []AppInstance
    Status         Status         // running, stopped, crashed
}

type AppInstance struct {
    Type   string
    Cmd    string
    PID    int
    Status Status
    Err    string // populated on failure
}
```

## Config System

### Format

YAML only. Hand-rolled validator returns precise errors, e.g.:

```
workspace.apps[2].cmd: required for type "editor"
workspace.name: must match ^[a-z0-9][a-z0-9-]*$
```

### Locations

- Global: `$XDG_CONFIG_HOME/dia/workspaces/*.yaml` (fallback `~/.config/dia/workspaces/`)
- Project-local: `.dia.yaml` walked up from cwd
- Merge: project-local shadows global on name collision
- Source path surfaced in the UI so users can `Edit` and jump to the right file

### Schema versioning

- Every config carries a `version` field
- Loader refuses to start a config with a newer major version than dia understands
- Backward-compatible additions allowed; breaking changes bump the major version

## State Store

- Backed by JSON in `$XDG_STATE_HOME/dia/` (fallback `~/.local/state/dia/`)
- Atomic writes (temp file + rename) with mutex
- Holds: `Instances map[string]Instance`, `Recent []string`, `Favorites []string`, `Theme string`, `Keybindings map[string]string`
- Log file: `dia.log` (append-only, no rotation)
- Concurrency: mutex-guarded; load + mutate + save as a single critical section

## Execution Model

1. Parse config
2. Validate schema
3. Resolve env vars, tildes
4. Resolve apps to launchers
5. Spawn processes (concurrent; sequential for `wait: true`)
6. Track PIDs in state
7. Emit `workspace:state-changed` Wails events for live UI
8. On stop: SIGTERM (5s grace), then SIGKILL; clear state

## Window Handling Strategy

dia does NOT:

- Fully control window manager behavior
- Guarantee layout precision
- Depend on OS-level snapshot APIs

dia WILL:

- Launch apps
- Provide a positioning interface hook (stub in v1; document the TODO)
- Rely on OS-level hints when implemented

## UI Design

### Main screen

- Workspace list with search, recents, favorites
- Workspace card with apps, start/stop, status pills, edit/open folder buttons
- Top bar: search input, refresh, new workspace, settings
- New workspace dialog with inline validation
- Settings panel with paths, doctor, plugins, theme selector, keybind editor

### Workspace actions

- Start (via card button or Enter key)
- Stop (via card button or Ctrl+. key, with confirmation)
- Edit config (opens the YAML in `$EDITOR`)
- Open folder (OS file manager)
- Favorite / unfavorite (star toggle)

### Keyboard shortcuts

All shortcuts are user-customizable via the Settings keybind editor. Defaults:

| Action | Default |
|---|---|
| Focus search | `/`, `CmdOrCtrl+F` |
| Start focused workspace | `Enter` |
| Stop focused workspace | `CmdOrCtrl+.` |
| New workspace | `CmdOrCtrl+N` |
| Close dialog / blur search | `Escape` |
| Toggle settings | `CmdOrCtrl+,` |
| Refresh | `CmdOrCtrl+R` |

### Theming

- daisyUI-based multi-theme system
- Dark (default) and Light themes built-in; users can pick from 30+ daisyUI presets
- Custom `"dia"` theme mapping the original JetBrains-inspired palette
- Theme selector in Settings panel
- Persisted in state store (`state.Data.Theme`)

## CLI Interface

```bash
dia                     # opens GUI
dia list                # list all workspaces
dia list --json         # JSON output
dia start <name>        # start workspace
dia start <name> --dry-run
dia stop <name>         # stop workspace
dia stop <name> --force
dia stop --all          # stop all
dia status              # running instances + PIDs
dia new <name>          # create starter YAML
dia new <name> --local  # create project-local .dia.yaml
dia edit <name>         # open config in $EDITOR
dia open <name>         # open workspace dir in OS file manager
dia doctor              # smoke checks
dia plugins             # list discovered plugins
dia reconcile           # clean dead PIDs
dia completion bash     # generate shell completion script
dia --version
```

Shell completions (bash/zsh/fish/powershell) via `dia completion <shell>`.

## Implementation Phases

### Phase 1 - Theme System (NEXT)

Goal: Add daisyUI-based theming with light/dark and user-selectable presets.

- Install `daisyui@latest` (v4.x, Tailwind 3 compatible)
- Add to `tailwind.config.js` plugins
- Define current dark palette as custom `"dia"` theme
- Define companion `"dia-light"` theme (inverted neutrals)
- Add `state.Data.Theme string` field for persistence
- Add `SetTheme(theme string)` / `GetTheme()` Go methods + wails bindings
- Add `<html data-theme="...">` binding in frontend, default to `"dia"`
- Add theme selector (dropdown) in SettingsPanel
- No existing components need changes (daisyUI coexists with raw Tailwind)

### Phase 2 - Workspace Organization

Goal: Search, recent workspaces, favorites, and a custom keybind system.

- **Search bar** in header -- client-side filter on workspace name, `/` and `CmdOrCtrl+F` to focus, `Escape` to blur/clear
- **Recent section** -- expose backend `state.Data.Recent` (already tracked via `pushRecent` on Start) through a new `GetRecent()` binding; render a Recent section above the workspace list
- **Favorites** -- add `ToggleFavorite(name string)` / `GetFavorites()` Go bindings to populate the currently write-only `state.Data.Favorites` field; star toggle on each WorkspaceCard; sort favorites first
- **Sort order**: favorites > recents > alphabetical
- **Custom keybind system**: `state.Data.Keybindings map[string]string` for user overrides; keyboard event handler at App.svelte level reading from store; default shortcut table (listed above); keybind editor in Settings (click-to-record per action)

### Phase 3 - GUI Interaction Polish

Goal: Refine the everyday interaction feel.

- Stop confirmation dialog (modal with Cancel/Stop)
- Inline validation in NewWorkspaceDialog (error next to input, not global banner)
- "Edit config" button on workspace card (calls `api.editWorkspace(name)` to launch `$EDITOR`)
- "Open folder" button on workspace card (reveals source dir in file manager)
- Show Reconcile feedback after clicking reconcile in Settings (brief inline result)
- Remove dead `$: $lastError` no-op in App.svelte
- Remove unused Nunito font + logo assets

### Phase 4 - Cross-Process State + Live Updates

Goal: GUI reflects CLI state changes without manual Refresh.

- Add `fsnotify` dependency
- In `App.Startup()`, start a goroutine watching `a.store.Path()` with fsnotify
- On file change (200ms debounce), re-Snapshot the runtime and emit Wails event `workspace:state-changed`
- Frontend subscribes to the event and auto-refreshes workspaces + instances
- Manual Refresh button becomes secondary

### Phase 5 - CLI Polish

Goal: Professional CLI experience with completions and fixed rough edges.

- **Shell completions** -- register cobra `completion` subcommand for bash/zsh/fish/powershell. Users run `dia completion bash > /etc/bash_completion.d/dia` to get tab-completion for commands, flags, and workspace names. Cobra handles the generation; ~10 lines of Go to wire the command.
- Fix `errAlreadyExists` exit code (currently ExitGeneral/1, should be ExitAlreadyExists/4)
- Add `$VISUAL` support to `edit` command (checked before `$EDITOR`)
- Add `--json` support to `open` command
- Add `--dry-run` flag to `start` (matches PLAN.md spec)
- Align `stop --all` JSON shape with `stop <name>` (string vs array inconsistency)

### Phase 6 - Frontend Tests + Hardening

Goal: Prevent regressions in the UI surface.

- Enable `svelte-check` in CI (already in devDeps)
- Add Vitest + Svelte Testing Library: component tests for WorkspaceCard (collapsed/expanded/running/stopped), NewWorkspaceDialog (validation inline), SettingsPanel (doctor/paths/plugins rendering)
- Playwright smoke test: launch built binary, verify empty state renders
- Fix Windows platform tests that currently skip

## Code Conventions

These are enforced in code review.

- No comments unless they explain why (not what)
- No restating-the-obvious comments
- Remove stale comments when modifying code
- Explicit error handling; no silent fallbacks
- Hand-rolled validation with precise error messages
- Tests for non-trivial logic; table-driven where applicable
- No emojis, no em-dashes, no non-ASCII characters in code, docs, or commits
- Prefer the standard library over a dependency
- Introduce abstractions only when they solve a real problem
- Keep files, functions, and types focused and cohesive
- Avoid God objects, God functions, and tightly coupled modules

## Out of Scope (post-v0.2)

- Window positioning implementation
- Sleep/resume workspaces (relaunch-based snapshotting)
- Remote workspaces
- Workspace templates marketplace
- TUI mode
- Log rotation
- JSON Schema validation file
- Homebrew/Scoop/apt distribution
- Lua or Go-plugin plugins (external executables only)
- Partial launch (run subset of apps)

## What dia is NOT

- Not a true window manager
- Not a process freezer
- Not a pixel-perfect layout engine
- Not dependent on OS-level snapshot APIs
- Not a replacement for tmux, just a peer to it

## Summary

dia is a deterministic workspace launcher that recreates full dev
environments from config.

It replaces:

- Manual app opening
- Scattered dev setup scripts
- Context switching friction

With:

- Click workspace, everything appears
- One YAML per project, or curated templates in a global dir
- Third-party app types via `dia-*` executables on PATH
- A desktop UI plus a scriptable CLI
