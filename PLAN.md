# dia — Project Plan (JetBrains-style Workspace Launcher)

## Overview
dia is a cross-platform workspace launcher that lets users define, start, and manage “development sessions” composed of multiple applications (editors, terminals, browsers, services) using declarative configs.

Instead of managing windows directly like a window manager, dia **recreates environments on demand** in a predictable way.

---

## Core Philosophy

- Not a window manager
- Not a process snapshot tool
- A **workspace orchestrator**
- “tmux UX, but at the system/app level”
- Deterministic rebuild > fragile state saving

---

## MVP Goals (v1)

### Must have
- JetBrains-style launcher UI (project picker)
- Workspace definitions via config files
- Launch multiple apps per workspace
- Basic app grouping (editor, terminal, browser, service)
- Cross-platform execution layer (Windows, macOS, Linux)
- “Start workspace” / “Stop workspace”

### Nice to have (v1.1)
- Recent workspaces
- Favorites / pinned workspaces
- Partial launch (run subset of apps)
- Simple layout hints (best-effort positioning)

---

## Tech Stack

### Core Language
- Go

### UI (choose one early)
- Option A: TUI (fast MVP)
  - `tview` or `bubbletea`
- Option B: Desktop GUI
  - Wails (Go + Web UI)
  - or Fyne

> Recommendation: start TUI → then upgrade to GUI

---

## High-Level Architecture

```

dia/
├── core/              # workspace logic (OS-agnostic)
├── runtime/           # process + app launching
├── platform/          # OS-specific implementations
├── config/            # workspace parsing + schema
├── ui/                # launcher interface
├── cli/               # command line entrypoint

````

---

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
````

---

### App Types

Standardized categories:

* `editor`
* `terminal`
* `browser`
* `service`
* `custom`

---

### Workspace Lifecycle

1. Load config
2. Validate schema
3. Resolve paths/env
4. Launch apps
5. Track running processes
6. Allow stop/restart

---

## Platform Layer Design

### Interface

```go
type Platform interface {
    Launch(command string, cwd string) (ProcessHandle, error)
    OpenURL(url string) error
    ListProcesses() []Process
    KillProcess(id string) error
}
```

---

### Implementations

* `platform_linux.go`
* `platform_windows.go`
* `platform_macos.go`

Each handles:

* process spawning
* optional window positioning (best effort)
* URL opening

---

## Runtime Layer

Responsible for:

* spawning apps
* tracking PIDs
* grouping processes into workspaces
* restart/stop logic

Key structs:

* WorkspaceInstance
* ProcessGroup
* AppInstance

---

## Config System

### Format: YAML (recommended)

Example:

```yaml
name: backend-dev
apps:
  - type: editor
    cmd: code .
  - type: terminal
    cmd: docker compose up
  - type: terminal
    cmd: go run main.go
```

### Requirements

* file-based storage
* versioned schema
* hot reload optional (later)

---

## UI Design (JetBrains-style)

### Main Screen

* Workspace list (folders / groups)
* Search bar
* Recent sessions
* “New workspace”

### Workspace Actions

* ▶ Start
* ⏹ Stop
* ✏ Edit config
* 📂 Open folder

---

## CLI Interface

```bash
dia list
dia start compiler
dia stop compiler
dia open compiler
dia new compiler
```

---

## Execution Model

When starting a workspace:

1. Parse config
2. Resolve environment variables
3. Spawn processes
4. Track PIDs
5. Optionally group windows (best effort)
6. Report status to UI

---

## Window Handling Strategy (Important)

### Strategy: “Best effort only”

dia will NOT:

* fully control window manager behavior
* guarantee layout precision

It WILL:

* launch apps
* optionally reposition windows (if supported)
* rely on OS-level hints

---

## Milestone Plan

### Phase 1 — Core Engine (Week 1–2)

* Workspace config parsing
* Process launching
* CLI interface
* Basic runtime tracking

---

### Phase 2 — Platform Support (Week 2–3)

* Linux implementation (first priority)
* macOS support
* Windows support

---

### Phase 3 — Launcher UI (Week 3–4)

* Workspace list UI
* Start/stop buttons
* Recent projects

---

### Phase 4 — Polish (Week 5+)

* Favorites
* Partial launch
* Error handling
* Logs/debug view

---

## Future Ideas (Post-MVP)

* Plugin system (custom app types)
* Workspace templates
* Docker/service orchestration integration
* Remote workspace execution
* “sleep/resume workspace” (relaunch-based snapshotting)
* Integration with tmux inside terminal apps

---

## Key Success Criteria

dia is successful if:

* A dev can open dia → click workspace → everything launches correctly
* Switching projects takes < 5 seconds
* No manual setup per session
* Works reliably across at least 2 OSes

---

## What dia is NOT

* Not a true window manager
* Not a process freezer
* Not a pixel-perfect layout engine
* Not dependent on OS-level snapshot APIs

---

## Summary

dia is a **deterministic workspace launcher** that recreates full dev environments from config.

It replaces:

* manual app opening
* scattered dev setup scripts
* context switching friction

With:

> “click workspace → everything appears”
