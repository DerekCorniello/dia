# dia — UI/UX Fixes Plan

User-reported issues with their proposed fixes. This plan covers all of them in one pass.

---

## Issue 1 — Plugins list always visible

**Problem:** The plugins section in `App.svelte` is filtered to `enabledPlugins` (only enabled plugins). It appears/disappears as plugin state changes, which feels inconsistent.

**Fix:**
- Add `$: availablePlugins = $pluginsStore;` (all discovered plugins, not just enabled)
- Section header becomes "Plugins ({availablePlugins.length})"
- Per-plugin rendering still calls `getData` (the panel gracefully handles disabled state)
- Tab filter ("all / per-plugin") still works for choosing which plugin's panel to display

**File:** `frontend/src/App.svelte`

---

## Issue 2 — Sort dropdown (with sort by usage)

**Problem:** Currently a static "Recent" section. User wants a dropdown to sort by: recent, most used, name, source.

**Backend changes:**

`internal/state/store.go` — replace `Recent []string` with a richer entry that tracks usage:
```go
type RecentEntry struct {
    Name  string `json:"name"`
    Count int    `json:"count"`
}

type Data struct {
    // ...
    Recent []RecentEntry `json:"recent"`
    // ...
}
```

`internal/runtime/runtime.go` — update `pushRecent` to bump the count:
```go
func pushRecent(recent []RecentEntry, name string, limit int) []RecentEntry {
    count := 1
    for i, e := range recent {
        if e.Name == name {
            count = e.Count + 1
            recent = append(recent[:i], recent[i+1:]...)
            break
        }
    }
    out := append([]RecentEntry{{Name: name, Count: count}}, recent...)
    if len(out) > limit {
        out = out[:limit]
    }
    return out
}
```

Migration: handle legacy `[]string` format on load (convert to `[]RecentEntry` with `Count: 0`).

**Frontend (`App.svelte`):**
- Dropdown next to the "Workspaces" heading: "Sort: [recent / most used / name / source]"
- Reactive sort applies to the filtered list:
  - `'recent'`: by recency order
  - `'most used'`: by `Count` descending
  - `'name'`: alphabetical
  - `'source'`: grouped by source, alphabetical within
- The "Recent" section header only appears in `'recent'` sort mode

**File:** `frontend/src/App.svelte`

---

## Issue 3 — Responsive grid layout

**Problem:** Workspace cards lay out in a single column even on wide screens.

**Fix:** In `App.svelte`, wrap the workspace lists in a CSS grid:
```html
<div class="grid grid-cols-1 md:grid-cols-2 2xl:grid-cols-3 gap-3">
```

Both the Recent section and the All Workspaces section use this grid. The expanded cards still work; cards have `min-w-0` to prevent overflow.

**File:** `frontend/src/App.svelte`

---

## Issue 4 — Workspace description

**Backend (`internal/config/types.go`):**
```go
type Workspace struct {
    Version     int         `yaml:"version"`
    Name        string      `yaml:"name"`
    Description string      `yaml:"description,omitempty"`
    Apps        []App       `yaml:"apps"`
    Plugins     []PluginRef `yaml:"plugins,omitempty"`
}
```

**Backend (`internal/wailsapp/types.go`):**
- Add `Description string \`json:"description,omitempty"\`` to `WorkspaceInfo`
- Add `Description string \`json:"description"\`` to `WorkspaceEditor`

**Backend (`internal/wailsapp/app.go`):**
- `ListWorkspaces`: populate `out.Description = ws.Description`
- `GetWorkspaceEditor`: populate `editor.Description = ws.Description`
- `SaveWorkspaceEditor`: include `Description: editor.Description` when marshaling

**Frontend (`WorkspaceCard.svelte`):**
- Below the name/source line, show `workspace.description` in `text-xs text-fg-dim line-clamp-2` if non-empty
- Truncate to 2 lines max

**Frontend (`WorkspaceEditor.svelte`):**
- Add a "Description" textarea (multi-line) below the Name field
- `bind:value={editor.description}` on a `<textarea>`, placeholder "What this workspace is for"

**Files:** `internal/config/types.go`, `internal/wailsapp/types.go`, `internal/wailsapp/app.go`, `WorkspaceCard.svelte`, `WorkspaceEditor.svelte`

---

## Issue 5 — Remove window-type plugin hint

**Problem:** The "This plugin opens in a separate window" hint in `PluginPanel.svelte` is now useless since all plugins work the same way (auto-spawn with workspace).

**Fix:** In `PluginPanel.svelte`, delete the `{#if plugin.ui.type === 'window'}` block entirely. The panel now falls through to the regular plugin rendering regardless of `ui.type`.

**File:** `frontend/src/lib/components/PluginPanel.svelte`

---

## Issue 6 — Keybinds reactive + persistence

**Problem:** `defaultKeybinds` is computed only once in `onMount` and doesn't update when `modKey` is detected. Per-change auto-save flow may be broken.

**Fix in `SettingsPanel.svelte`:**
- Convert to a reactive declaration that re-computes when `modKey` is set:
```ts
let modKey = 'Ctrl';
let userOverrides: Record<string, string> = {};

const buildDefaults = (mod: string): Record<string, string> => ({
  'Focus search': '/',
  'New workspace': `${mod}+N`,
  'Toggle settings': `${mod}+,`,
  'Refresh': `${mod}+R`,
  'Close dialog': 'Escape',
});

$: defaultKeybinds = buildDefaults(modKey);
$: keybinds = { ...defaultKeybinds, ...userOverrides };

onMount(async () => {
  modKey = navigator.platform.includes('Mac') ? 'Cmd' : 'Ctrl';
  try {
    userOverrides = await api.getKeybindings();
  } catch {
    userOverrides = {};
  }
});
```

- Auto-save on every change: handlers call `api.setKeybinding(action, combo)` and update `userOverrides` directly
- Resetting one removes from `userOverrides`

**File:** `frontend/src/lib/components/SettingsPanel.svelte`

---

## Issue 7 — DEFERRED (no Save button, no unsaved-changes confirmation)

User changed their mind. The auto-save pattern from #6 stays. No modal for unsaved changes. Keep it simple.

---

## Issue 8 — More theming

**WorkspaceCard:**
- Left border accent when running: `border-l-4 border-l-accent` (in addition to existing `border-accent/25`)
- Source pill under the name:
  - Global: `rounded-full bg-bg-700 text-fg-mute px-2 py-0.5 text-[10px]`
  - Local: `rounded-full bg-accent/15 text-accent px-2 py-0.5 text-[10px]`
- Delete button: `bg-accent-err/10 text-accent-err` (always tinted red, not just on hover)
- Edit button: `hover:bg-accent/20 hover:text-fg`
- "RUNNING" pill next to the green dot when running: `bg-accent/15 text-accent rounded-full px-2 py-0.5 text-[10px] font-medium`

**Plugins section in `App.svelte`:**
- Per-plugin card border: `border-accent/30` when enabled (verify)
- The "all" tab when selected: `bg-accent/20 text-accent`
- Disabled plugin cards: muted background `bg-bg-800/50`, muted text, a "disabled" pill

**Header / search input:**
- When search has a value, add a small accent dot/ring on the left side of the input
- "Settings" button: when settings open, `bg-accent/20 text-accent`

**PluginPanel:**
- When the panel's plugin is disabled, the entire card has muted background `bg-bg-800/50`
- Action buttons (e.g. "open", "refresh") use `bg-accent/20` for primary actions

**Footer:**
- "X running" indicator: use `text-accent` when count > 0
- State file path: keep muted

**Per-plugin section ("all" selected):**
- Subtle gradient on section background: `bg-gradient-to-b from-bg-700/30 to-transparent`

**Files:** `WorkspaceCard.svelte`, `App.svelte`, `PluginPanel.svelte`

---

## Issue 9 — Fix Ctrl+N false positive

**Problem:** `Ctrl+N` and `Ctrl+Shift+N` both trigger "new workspace" because the handler checks `e.key === 'n' || 'N'` without considering other modifiers. The handler also has a blanket `e.preventDefault()` for any `mod` key, which breaks copy/paste/select-all (see #10 and #11).

**Fix:** This is resolved by the new selective handler in #11 below. Adding `!e.shiftKey` to the condition for the specific shortcut is the simple form. The proper app-wide fix is the whitelist approach in #11.

**File:** `frontend/src/App.svelte`

---

## Issue 10 — Copy/paste/select-all don't work

**Problem:** The current `onGlobalKey` in `App.svelte` has:
```ts
if (mod) e.preventDefault();
```
This blanket `preventDefault()` for any ctrl/cmd keydown blocks ALL native browser shortcuts — Ctrl+C, Ctrl+V, Ctrl+A, Ctrl+Tab, etc.

**Fix:** Remove the blanket preventDefault. Each app shortcut calls `preventDefault()` only for itself.

**File:** `frontend/src/App.svelte`

---

## Issue 11 — Whitelist app shortcuts (use correct keybinds)

**Problem:** Even after #10, the handler still intercepts the specific app shortcuts regardless of whether the user customized them. The proper app-wide fix is to whitelist the recognized app shortcuts and only `preventDefault` for them. All other keys pass through to the webview for native handling.

**Fix:**

`frontend/src/lib/stores.ts` — add a keybinds store so `App.svelte` can read user-customized bindings:
```ts
export const keybinds: Writable<Record<string, string>> = writable({});
```

`App.svelte`:
1. In `refresh`, load overrides from backend and merge with platform-aware defaults, store in the keybinds store
2. The handler uses an `isAppShortcut(e)` helper that checks if the pressed combo matches any of the recognized app shortcuts (using the current `keybinds` store values, not hardcoded)
3. Only `preventDefault` when `isAppShortcut` returns true

```ts
$: appShortcuts = (() => {
  const m = keybinds; // current bindings
  const set = new Set<string>();
  for (const v of Object.values(m)) {
    set.add(normalizeCombo(v, e.metaKey, e.ctrlKey));
  }
  return set;
})();

function isAppShortcut(e: KeyboardEvent): boolean {
  // Build the combo from the event
  const parts: string[] = [];
  if (e.ctrlKey) parts.push('Ctrl');
  if (e.metaKey) parts.push('Cmd');
  if (e.altKey) parts.push('Alt');
  if (e.shiftKey) parts.push('Shift');
  if (!['Control','Meta','Alt','Shift'].includes(e.key)) {
    parts.push(e.key.length === 1 ? e.key.toUpperCase() : e.key);
  }
  return appShortcuts.has(parts.join('+'));
}

function onGlobalKey(e: KeyboardEvent) {
  if (showSettings || showNew || selectedEditorName) return;
  if (!isAppShortcut(e)) return; // pass through to webview

  e.preventDefault();
  // dispatch action based on which app shortcut matched
  // ...
}
```

This way:
- `Ctrl+C`, `Ctrl+V`, `Ctrl+X`, `Ctrl+A`, `Ctrl+Tab`, `Ctrl+1-9` etc. — all native, never `preventDefault`'d
- The five app shortcuts (Focus search, New workspace, Toggle settings, Refresh, Close dialog) — always `preventDefault`'d
- If the user rebinds an app shortcut to a different combo, the new combo becomes the recognized one
- If the user rebinds a native shortcut (e.g. `Ctrl+S`) to an app action, the native behavior is replaced

**File:** `frontend/src/App.svelte`, `frontend/src/lib/stores.ts`

---

## Files to modify

| File | Issues addressed |
|---|---|
| `internal/state/store.go` | #2 (RecentEntry type) |
| `internal/runtime/runtime.go` | #2 (pushRecent with count) |
| `internal/wailsapp/types.go` | #2, #4 (Recent type, Description) |
| `internal/wailsapp/app.go` | #2, #4 (wire through) |
| `internal/config/types.go` | #4 (Description on Workspace) |
| `frontend/src/lib/stores.ts` | #11 (keybinds store) |
| `frontend/src/lib/components/SettingsPanel.svelte` | #6 (reactive keybinds) |
| `frontend/src/App.svelte` | #1, #2, #3, #9, #10, #11, plus #8 plugin section |
| `frontend/src/lib/components/WorkspaceCard.svelte` | #4, #8 |
| `frontend/src/lib/components/WorkspaceEditor.svelte` | #4 (description field) |
| `frontend/src/lib/components/PluginPanel.svelte` | #5, #8 |

---

## Order of implementation

1. Backend: state + config + types changes (Issues #2, #4)
2. Backend: app.go wiring
3. Frontend types: api.ts + App.svelte (Issue #1, #8 plugin section)
4. Frontend: WorkspaceCard (#4, #8)
5. Frontend: WorkspaceEditor (#4)
6. Frontend: PluginPanel (#5, #8)
7. Frontend: stores.ts keybinds store (#11)
8. Frontend: SettingsPanel reactive keybinds (#6)
9. Frontend: App.svelte sort dropdown, grid, plugins-always-visible, keybind handler (#2, #3, #1, #11, #10, #9)
10. Build and verify

---

## Not in scope (this round)

- Phase 3 from PLAN.md (Stop confirmation, inline validation, edit config button, open folder button, Reconcile feedback, asset cleanup)
- Phase 4 (Cross-process state + live updates via fsnotify)
- Phase 5 (CLI polish, shell completions, $VISUAL support)
- Phase 6 (Frontend tests, Vitest setup)
- Plugin commands as apps
- App reordering
- Workspace import/export
