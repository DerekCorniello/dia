# Plugin System v1 - Implementation Plan

## Scope

- **Framework:** Stay on Wails v2 stable (no migration)
- **Plugin runtime:** Goja (in-process JS interpreter, sandboxed)
- **UI model:** Two flavors - host auto-wraps from manifest
  (`ui.type` of `list|grid|table|kv|text|canvas`) for embedded
  panels, and `ui.type=window` for plugins that open in their
  own OS-level window. Window plugins ship a `panel/` folder of
  plain HTML/JS/CSS; the host injects a `window.dia` proxy and
  the browser runs whatever the plugin author writes. No
  Svelte, no build step.
- **Sandbox level:** Obsidian model - JS in goja is sandboxed
  (no DOM, no fetch, only `dia.*`); host auto-rendered UI is
  trusted host code (not plugin code). Window plugins are
  sandboxed only by the browser: the plugin's `panel/panel.js`
  has the full web platform and the `window.dia` proxy. Plugin
  capability gating is enforced server-side (in the new dia
  process) and the `window.dia` call is the only escape hatch.
- **TS support:** Plugins ship pre-compiled JS. No runtime
  transpilation. We provide a `dia plugin new <id>` scaffold
  that does the build for the author.
- **Out of scope (for v1):** Marketplace, auto-update, plugin-to-plugin
  IPC, network capabilities, hot reload (requires restart),
  plugin Svelte components. The "multi-window per plugin" item
  originally listed here is now in scope as `ui.type=window`
  (one window per plugin, spawn-on-open).

## Target architecture

Plugins live in two locations, scanned in order (local wins on id collision):

```
$XDG_STATE_HOME/dia/plugins/<id>/        # global: shared across all projects
<cwd>/.dia/plugins/<id>/                 # local: scoped to a single project
```

Each plugin folder has the same layout:

```
<plugin-dir>/
  plugin.json        # manifest (validated by host)
  index.js           # entry, runs in goja
  lib/               # optional, for require() within the plugin
    helper.js
  styles.css         # optional, scoped via plugin id
  panel/             # only when ui.type=window
    index.html       # optional (host generates a default)
    panel.js         # browser-side, runs in the new window
    styles.css       # optional
```

Host flow on app start:

```
1. Resolve global plugins dir (state.ResolveStateDir() + /plugins)
2. Resolve local plugins dir (cwd + /.dia/plugins, when cwd is set)
3. Scan local first, then global (local overrides on id collision)
4. For each: validate manifest, check capabilities vs. user grants
5. Build plugin registry (in-memory), expose via ListPlugins()
6. Frontend renders the "Plugins" tab from this list
```

Host flow on plugin panel mount:

```
1. Create a fresh goja.Runtime
2. Inject `dia` object with capability-gated methods
3. Provide a tiny require() that resolves to local files in the plugin folder
4. Run `require("./index.js")` (entry from manifest)
5. Plugin's exported getData() is called to populate the host-rendered panel
6. Plugin can listen for events (onMount, onRefresh, onAction) via exported functions
```

## File plan

### New Go files

| File | Lines (est) | Purpose |
|---|---|---|
| `internal/plugins/manifest.go` | ~150 | `PluginManifest` type, `Validate()` |
| `internal/plugins/manager.go` | ~250 | Discovery, enable/disable, lifecycle, error tracking |
| `internal/plugins/runtime.go` | ~300 | Goja wrapper, module loader, error capture |
| `internal/plugins/bridge.go` | ~200 | The `dia.*` object factory, capability gating |
| `internal/plugins/capabilities.go` | ~120 | Capability list, allowlist, validation |
| `internal/plugins/manager_test.go` | ~200 | Unit tests |
| `internal/plugins/manifest_test.go` | ~100 | Validation tests |
| `internal/plugins/runtime_test.go` | ~200 | goja integration tests |
| `internal/cli/cmd_plugin.go` | ~250 | `dia plugin new\|list\|install\|uninstall\|enable\|disable\|info` |
| `internal/cli/cmd_plugin_test.go` | ~150 | CLI tests |
| `examples/hello-plugin/plugin.json` | ~30 | Example manifest |
| `examples/hello-plugin/index.js` | ~50 | Example plugin logic |

### Modified Go files

| File | Change |
|---|---|
| `internal/wailsapp/app.go` | Add 5 methods: `ListPlugins`, `EnablePlugin`, `DisablePlugin`, `PluginCall`, `PluginRefresh`. Wire `*plugins.Manager` into the `App` struct. |
| `internal/wailsapp/types.go` | Add `PluginInfo`, `PluginManifestInfo`, `PluginError` types. |
| `internal/wailsapp/app_test.go` | Add tests for new bindings. |
| `internal/state/store.go` | Add `Data.Plugins` map (plugin id -> enabled/disabled, granted capabilities, last error, last refresh). |
| `internal/cli/cli.go` | Add `newPluginCmd` to root. |
| `go.mod` | Add `github.com/dop251/goja` (latest stable). |

### New frontend files

| File | Lines (est) | Purpose |
|---|---|---|
| `frontend/src/lib/components/PluginPanel.svelte` | ~300 | Host-renderer for plugin UIs (list/grid/cards/table/kv/text). Switches on `manifest.ui.type`. |
| `frontend/src/lib/plugin-bridge.ts` | ~80 | Client-side bridge: `dia.plugin.call(method, args)` proxies to `PluginCall` wails binding. |

### Modified frontend files

| File | Change |
|---|---|
| `frontend/src/lib/api.ts` | Add wrappers: `listPlugins`, `enablePlugin`, `disablePlugin`, `pluginCall`, `pluginRefresh`. Add types: `PluginInfo`, `PluginManifestInfo`. |
| `frontend/src/lib/stores.ts` | Add `plugins` writable. |
| `frontend/src/App.svelte` | Add a Plugins section. Add "Refresh plugins" / "Open plugins folder" buttons. |
| `frontend/wailsjs/go/wailsapp/App.{d.ts,js}` | Regenerated. |
| `frontend/wailsjs/go/models.ts` | Regenerated. |
| `PLAN.md` | Document the new plugin system. |
| `CHANGELOG.md` | Add 0.3.0 entry. |
| `README.md` | Mention plugin support. |

Total: ~2000 new lines of Go, ~500 new lines of Svelte/TS, ~30 lines of example plugin.

## Data shapes

### Plugin manifest (`plugin.json`)

```typescript
interface PluginManifest {
  id: string;                    // [a-z0-9-]{3,40}, unique
  name: string;                  // 1-60 chars
  version: string;               // semver
  description: string;           // 0-200 chars
  author: string;                // 0-60 chars
  entry: string;                 // default "index.js", must be relative path
  capabilities: Capability[];    // empty = read-only default set
  ui: PluginUISpec;
}

type Capability =
  | "workspaces:read"     // default-on
  | "instances:read"      // default-on
  | "doctor:read"         // default-on
  | "paths:read"          // default-on
  | "themes:read"         // default-on
  | "workspaces:start"    // opt-in
  | "instances:stop"      // opt-in
  | "workspaces:create"   // opt-in
  | "themes:write";       // opt-in

interface PluginUISpec {
  type: "list" | "grid" | "table" | "kv" | "text" | "canvas" | "window";
  title: string;
  entry?: string;            // for type=window: path to panel/panel.js
  width?: number;            // for type=window: initial window width
  height?: number;           // for type=window: initial window height
  refreshable?: boolean;     // show a refresh button
  actions?: PluginAction[];  // buttons in the panel
  columns?: { key: string; label: string; format?: "badge" | "duration" | "text" }[];  // for table
}

interface PluginAction {
  id: string;
  label: string;
  confirm?: boolean;          // require confirm dialog before invoking
  capability: Capability;     // required to invoke
}
```

### Plugin JS contract

```js
// index.js
// Module exports are optional. Only getData() and onAction() are required for the panel to work.

export async function getData() {
  // Returns data shaped to match manifest.ui.type
  //   - "list": [{ id, title, subtitle?, badge?: "running"|"stopped"|"error" }, ...]
  //   - "grid": same as list
  //   - "table": [{ col1: val, col2: val }, ...]
  //   - "kv":   { key1: val1, key2: val2 }
  //   - "text":  string (markdown rendered)
  return await dia.listWorkspaces();
}

export async function onAction(actionId, item, context) {
  // Called when user clicks an action button.
  // Default: re-fetch getData() after action returns.
}

// Optional lifecycle hooks
export async function onMount()   { /* panel first shown */ }
export async function onRefresh() { /* user clicked refresh */ }
export async function onUnmount() { /* panel closing */ }
```

### Backend types (`internal/wailsapp/types.go` additions)

```go
type PluginInfo struct {
    ID           string             `json:"id"`
    Manifest     PluginManifestInfo `json:"manifest"`
    Enabled      bool               `json:"enabled"`
    GrantedCaps  []string           `json:"grantedCapabilities"`
    LastError    string             `json:"lastError,omitempty"`
    LastRefresh  int64              `json:"lastRefresh,omitempty"`
    HasData      bool               `json:"hasData"`
    PluginDir    string             `json:"pluginDir"`
}

type PluginManifestInfo struct {
    ID           string   `json:"id"`
    Name         string   `json:"name"`
    Version      string   `json:"version"`
    Description  string   `json:"description"`
    Author       string   `json:"author"`
    Entry        string   `json:"entry"`
    Capabilities []string `json:"capabilities"`
    UI           map[string]any `json:"ui"`
}
```

## Capability allowlist

Default-on (read-only, granted automatically):
- `workspaces:read` -> `dia.listWorkspaces()`, `dia.getWorkspace(name)`
- `instances:read`  -> `dia.listInstances()`
- `doctor:read`     -> `dia.doctor()`
- `paths:read`      -> `dia.paths()`
- `themes:read`     -> `dia.getTheme()`, `dia.listCustomThemes()`

Opt-in (mutating, require explicit grant in manifest):
- `workspaces:start`  -> `dia.startWorkspace(name)`
- `instances:stop`    -> `dia.stopInstance(id)`, `dia.stopAll()`
- `workspaces:create` -> `dia.newWorkspace(name)`
- `themes:write`      -> `dia.setTheme(theme)`, `dia.setCustomTheme(info)`, `dia.deleteCustomTheme(name)`

Forbidden (never exposed to plugins):
- `openConfigFolder()`, `openStateFolder()` - user-only operations
- `reconcile()` - host-only operation
- `enablePlugin()`, `disablePlugin()`, `installPlugin()`, `uninstallPlugin()` - no plugin can manage other plugins

## The `dia` object in goja

Built by `internal/plugins/bridge.go`. Each method:
1. Checks the plugin's granted capabilities
2. If denied, throws a JS Error
3. If allowed, calls the bound Go method
4. Converts the result to a JS value (json.Marshal -> goja value)

Promise-returning methods (start, stop, etc.) return a JS Promise that resolves when the Go call completes.

## Host panel renderer (`PluginPanel.svelte`)

Takes a `PluginInfo` and a `refresh` function. Renders the appropriate UI based on `manifest.ui.type`:
- `list`: vertical list of items, each with `title`, `subtitle`, optional `badge` (color-coded), optional inline actions
- `grid`: responsive card grid
- `table`: tabular data with `columns` schema from manifest
- `kv`: key-value pairs in a two-column layout
- `text`: rendered text (markdown supported via marked or similar)
- `canvas`: free-draw `<canvas>` with pointer capture; strokes are passed to actions as `ctx.strokes`
- `window`: shows a brief description and an "open window" button that spawns a new dia process via `OpenPluginWindow(id)`

When user clicks an action button, host calls `PluginCall(pluginID, "onAction", [actionId, item])` and re-fetches data on resolution.

When user clicks refresh, host calls `PluginCall(pluginID, "onRefresh", [])` and then re-fetches data.

## Window plugin flow

A plugin with `ui.type=window` opens in a new OS-level window:

1. User clicks "open window" on the plugin's panel in the main
   app, or the main app auto-spawns the window when the plugin
   is enabled (toggle-on auto-open is a v1.1 feature; for v1 the
   button is explicit).
2. The main dia process calls `os.Executable()` and re-spawns
   the same binary with `--plugin-window=<id>`.
3. The second process resolves the plugin dir, loads its
   `panel/index.html` (or generates a default with `<div id="root">`),
   and serves `panel/panel.js` + `styles.css` via a custom
   `assetserver.Options.Handler`.
4. The second process loads the plugin's `index.js` in goja and
   binds a single `DiaCall(method, argsJSON)` wails method. The
   host injects a generated `dia.js` that creates `window.dia`
   with read-only host methods and a generic `dia.call(method)`
   that hits `DiaCall`.
5. `DiaCall` dispatches: dia.* methods go to the read-only host
   (state file read), everything else goes to the goja runtime
   (the plugin's `module.exports.*`).
6. The new window has no workspace runtime; mutating operations
   (`startWorkspace`, `stopInstance`, etc.) return an error.

Window plugins keep their `Enabled` flag and granted
capabilities in `state.Data.Plugins` so the user can re-open
them across restarts. The second process is short-lived - it
exits when the user closes the window.

## Lifecycle

- **Discovery:** Scan plugin dirs on app start. Re-scan on `ReloadPlugins` call.
- **Enable:** Instantiate goja runtime, load entry module, call `onMount`.
- **Disable:** Stop goja runtime, call `onUnmount`.
- **Refresh:** Call plugin's `getData()`, update panel state.
- **Error:** Caught in goja, stored in `PluginInfo.LastError`, displayed in plugin card. Plugin is auto-disabled if error count exceeds threshold.
- **Crash:** Caught by host, plugin marked as errored. User can disable.
- **State:** Persisted in `state.Data.Plugins` so enabled/disabled and granted capabilities survive restarts.

## CLI command: `dia plugin`

```
dia plugin list                 # list installed plugins
dia plugin new <id>             # scaffold a new plugin in cwd
dia plugin install <path>       # move/copy a folder into the plugins dir
dia plugin uninstall <id>       # remove a plugin folder
dia plugin enable <id>          # enable a plugin
dia plugin disable <id>         # disable a plugin
dia plugin info <id>            # show details
```

## Tests

- Manifest validation: good manifests pass, bad ones fail with specific errors
- Capability enforcement: plugin cannot call a method it doesn't have
- goja error handling: JS exception becomes a Go error
- Plugin discovery: scan finds plugins, ignores bad ones
- Scaffolding: `dia plugin new <id>` creates a working plugin folder
- Install: `dia plugin install <path>` moves the folder correctly

## Effort estimate

- Day 1: Manifest type + validation, capabilities
- Day 2: Manager (discovery, enable/disable, state)
- Day 3: goja Runtime + bridge (dia.* object)
- Day 4: Wails bindings + frontend wrappers
- Day 5: PluginPanel.svelte (host renderer)
- Day 6: CLI `dia plugin` command + scaffold
- Day 7: Example plugin + tests
- Day 8: Polish, docs, final checks

Total: ~5-8 days.
