<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api, describeError, type RecentEntry, type WorkspaceInfo } from './lib/api';
  import {
    workspaces,
    doctor,
    paths,
    loading,
    lastError,
    theme,
    customThemes,
    plugins as pluginsStore,
    pluginPaths as pluginPathsStore,
    keybinds as keybindsStore,
    projectDir,
  } from './lib/stores';
  import { buildAllCustomThemesCss } from './lib/colors';
  import { EventsOn } from '../wailsjs/runtime';
  import WorkspaceCard from './lib/components/WorkspaceCard.svelte';
  import SettingsPanel from './lib/components/SettingsPanel.svelte';
  import NewWorkspaceDialog from './lib/components/NewWorkspaceDialog.svelte';
  import WorkspaceEditor from './lib/components/WorkspaceEditor.svelte';
  import PluginPanel from './lib/components/PluginPanel.svelte';

  type SortKey = 'recent' | 'mostUsed' | 'name' | 'source';
  const SORT_KEYS: SortKey[] = ['recent', 'mostUsed', 'name', 'source'];

  function sortLabel(s: SortKey): string {
    if (s === 'recent') return 'Recent';
    if (s === 'mostUsed') return 'Most Used';
    if (s === 'name') return 'Name';
    return 'Source';
  }

  let showSettings = false;
  let showNew = false;
  let showSortMenu = false;
  let selectedEditorName: string | null = null;
  let search = '';
  let searchInput: HTMLInputElement;
  let recent: RecentEntry[] = [];
  let sortKey: SortKey = 'recent';
  let zoomLevel = 1.0;
  const ZOOM_MIN = 0.5;
  const ZOOM_MAX = 2.0;
  const ZOOM_STEP = 0.1;

  let customThemeStyle: HTMLStyleElement | null = null;
  let unsubStateChanged: (() => void) | null = null;

  function applyCustomThemes(css: string) {
    if (typeof document === 'undefined') return;
    if (!customThemeStyle) {
      customThemeStyle = document.createElement('style');
      customThemeStyle.setAttribute('data-custom-themes', '');
      document.head.appendChild(customThemeStyle);
    }
    customThemeStyle.textContent = css;
  }

  $: applyCustomThemes(
    buildAllCustomThemesCss(
      $customThemes.map((t) => ({
        name: t.name,
        colorScheme: t.color_scheme as 'light' | 'dark',
        colors: t.colors,
      })),
    ),
  );

  $: availablePlugins = $pluginsStore;

  // Platform-aware modifier: macOS uses Cmd, everything else uses Ctrl.
  // Effective keybindings = platform-aware defaults + user overrides.
  // The store only persists user overrides; defaults are derived in
  // the UI so the matcher always knows the full set of recognized
  // app shortcuts even when the user has not rebound anything.
  let platformMod = 'Ctrl';

  $: effectiveKeybinds = {
    'Focus search': '/',
    'New workspace': `${platformMod}+N`,
    'Toggle settings': `${platformMod}+,`,
    'Refresh': `${platformMod}+R`,
    'Close dialog': 'Escape',
    'Zoom in': `${platformMod}+=`,
    'Zoom out': `${platformMod}+-`,
    'Zoom reset': `${platformMod}+0`,
    ...$keybindsStore,
  };

  function comboMatchesEvent(combo: string, e: KeyboardEvent): boolean {
    return combo.toLowerCase() === normalizeEventCombo(e);
  }

  function normalizeEventCombo(e: KeyboardEvent): string {
    const parts: string[] = [];
    if (e.ctrlKey) parts.push('ctrl');
    if (e.metaKey) parts.push('cmd');
    if (e.altKey) parts.push('alt');
    if (e.shiftKey) parts.push('shift');
    const k = e.key;
    if (k && k.length === 1) parts.push(k.toLowerCase());
    else if (k === ',') parts.push(',');
    else if (k === '/') parts.push('/');
    else if (k === 'Escape') parts.push('escape');
    return parts.join('+');
  }

  // Filtered + sorted workspace list.
  $: filtered = $workspaces
    .filter((w) => !search || w.name.toLowerCase().includes(search.toLowerCase()))
    .sort((a, b) => sortWorkspaces(a, b, sortKey, recent));

  // Filtered plugins list (search matches name/id).
  $: filteredPlugins = $pluginsStore.filter(
    (p) => !search || (p.name || p.id).toLowerCase().includes(search.toLowerCase()),
  );

  function sortWorkspaces(
    a: WorkspaceInfo,
    b: WorkspaceInfo,
    key: SortKey,
    rec: RecentEntry[],
  ): number {
    if (key === 'name') return a.name.localeCompare(b.name);
    if (key === 'source') {
      const c = (a.source || '').localeCompare(b.source || '');
      return c !== 0 ? c : a.name.localeCompare(b.name);
    }
    if (key === 'mostUsed') {
      const ac = a.useCount ?? 0;
      const bc = b.useCount ?? 0;
      if (ac !== bc) return bc - ac;
      return a.name.localeCompare(b.name);
    }
    // 'recent' = recency order, with non-recent falling back to alphabetical
    const aRecent = rec.findIndex((r) => r.name === a.name);
    const bRecent = rec.findIndex((r) => r.name === b.name);
    if (aRecent !== -1 && bRecent !== -1) return aRecent - bRecent;
    if (aRecent !== -1) return -1;
    if (bRecent !== -1) return 1;
    return a.name.localeCompare(b.name);
  }

  $: recentWs = sortKey === 'recent' ? filtered.filter((w) => recent.some((r) => r.name === w.name)) : [];
  $: otherWs = sortKey === 'recent' ? filtered.filter((w) => !recent.some((r) => r.name === w.name)) : filtered;

  onDestroy(() => {
    customThemeStyle?.remove();
    customThemeStyle = null;
    unsubStateChanged?.();
  });

  async function refresh() {
    loading.set(true);
    lastError.set(null);
    try {
      const [ws, doc, p, ct, pl, pp, rec, pd] = await Promise.all([
        api.listWorkspaces(),
        api.doctor(),
        api.paths(),
        api.listCustomThemes(),
        api.listPlugins(),
        api.pluginPaths(),
        api.getRecent(),
        api.getProjectDir(),
      ]);
      workspaces.set(ws);
      doctor.set(doc);
      paths.set(p);
      customThemes.set(ct);
      pluginsStore.set(pl);
      pluginPathsStore.set(pp);
      recent = rec;
      projectDir.set(pd);
    } catch (e) {
      lastError.set(`refresh: ${describeError(e)}`);
    } finally {
      loading.set(false);
    }
  }

  async function changeTheme(id: string) {
    theme.set(id);
    document.documentElement.dataset.theme = id;
    try {
      await api.setTheme(id);
    } catch (e) {
      lastError.set(`theme: ${describeError(e)}`);
    }
  }

  function applyZoom() {
    document.documentElement.style.zoom = String(zoomLevel);
  }

  function zoomIn() {
    zoomLevel = Math.min(ZOOM_MAX, Math.round((zoomLevel + ZOOM_STEP) * 10) / 10);
    applyZoom();
  }

  function zoomOut() {
    zoomLevel = Math.max(ZOOM_MIN, Math.round((zoomLevel - ZOOM_STEP) * 10) / 10);
    applyZoom();
  }

  function zoomReset() {
    zoomLevel = 1.0;
    applyZoom();
  }

  function openNew() { showNew = true; }
  function closeNew() { showNew = false; }

  async function openProject() {
    try {
      const dir = await api.selectProjectDir();
      if (dir) {
        projectDir.set(dir);
        await refresh();
      }
    } catch (e) {
      lastError.set(`open project: ${describeError(e)}`);
    }
  }

  async function closeProject() {
    try {
      await api.clearProjectDir();
      projectDir.set('');
      await refresh();
    } catch (e) {
      lastError.set(`close project: ${describeError(e)}`);
    }
  }

  function toggleSettings() { showSettings = !showSettings; }
  function closeSettings() { showSettings = false; }

  function onNewCreated(event: CustomEvent<string>) {
    selectedEditorName = event.detail;
  }

  function onEditorClose() { selectedEditorName = null; }

  async function onEditorSaved() {
    selectedEditorName = null;
    await refresh();
  }

  async function onEditorDeleted() {
    selectedEditorName = null;
    await refresh();
  }

  function onGlobalKey(e: KeyboardEvent) {
    if (showSettings || showNew || selectedEditorName) return;
    if (showSortMenu && e.key === 'Escape') {
      showSortMenu = false;
      return;
    }

    // Pass through native shortcuts the webview handles itself
    // (Ctrl+C, Ctrl+V, Ctrl+A, Ctrl+Tab, Ctrl+1-9, etc.). Only
    // intercept keys the user has bound to a dia action.
    const matchedAction = Object.entries(effectiveKeybinds).find(
      ([, combo]) => combo && comboMatchesEvent(combo, e),
    );
    if (!matchedAction) return;

    e.preventDefault();
    const [action] = matchedAction;

    if (action === 'Focus search') {
      searchInput?.focus();
      return;
    }
    if (action === 'New workspace') {
      openNew();
      return;
    }
    if (action === 'Toggle settings') {
      toggleSettings();
      return;
    }
    if (action === 'Refresh') {
      refresh();
      return;
    }
    if (action === 'Close dialog') {
      if (search) {
        search = '';
        searchInput?.blur();
      }
      return;
    }
    if (action === 'Zoom in') { zoomIn(); return; }
    if (action === 'Zoom out') { zoomOut(); return; }
    if (action === 'Zoom reset') { zoomReset(); return; }
  }

  onMount(async () => {
    platformMod = navigator.platform.includes('Mac') ? 'Cmd' : 'Ctrl';
    try {
      const [t, ct, overrides] = await Promise.all([
        api.getTheme(),
        api.listCustomThemes(),
        api.getKeybindings(),
      ]);
      theme.set(t);
      document.documentElement.dataset.theme = t;
      customThemes.set(ct);
      keybindsStore.set(overrides);
    } catch (e) {
      // fall back to defaults
    }
    try {
      projectDir.set(await api.getProjectDir());
    } catch {
      projectDir.set('');
    }
    unsubStateChanged = EventsOn("workspace:state-changed", () => {
      refresh();
    });
    await refresh();
  });
</script>

<svelte:window on:keydown={onGlobalKey} />

<div class="flex h-screen flex-col">
  <header class="flex items-center gap-3 border-b border-primary/20 px-5 py-3">
    <h1 class="shrink-0 flex h-6 items-center gap-2">
      <span class="text-xl font-bold text-primary">dia</span>
      <span class="text-fg/30">|</span>
      <span class="text-sm font-semibold tracking-wider text-secondary">The Do-It-All App</span>
    </h1>
    <div class="flex-1 flex items-center justify-center">
      <div class="relative w-full max-w-md">
        {#if search}
          <span class="pointer-events-none absolute left-2 top-1/2 h-2 w-2 -translate-y-1/2 rounded-full bg-primary shadow-glow-sm"></span>
        {/if}
        <input
          bind:this={searchInput}
          type="text"
          bind:value={search}
          placeholder="Search..."
          class="block w-full rounded border border-bg-600 bg-bg-800 pl-6 pr-3 py-1.5 text-xs placeholder:text-fg-mute focus:border-primary focus:ring-2 focus:ring-primary/30 focus:outline-none"
        />
      </div>
    </div>
    <div class="flex items-center gap-2 shrink-0">
      <button
        type="button"
        on:click={openProject}
        class="rounded bg-bg-600 px-3 py-1.5 text-xs text-fg-dim hover:bg-bg-600/70 hover:text-fg"
        title="open project directory"
      >
        Open
      </button>
      <button
        type="button"
        on:click={openNew}
        class="rounded bg-primary/20 px-3 py-1.5 text-xs font-medium text-primary hover:bg-primary/30"
      >
        + New
      </button>
      <button
        type="button"
        on:click={toggleSettings}
        class="rounded px-3 py-1.5 text-xs {showSettings ? 'bg-primary/20 text-primary' : 'bg-bg-600 hover:bg-bg-600/70'}"
      >
        Settings
      </button>
    </div>
  </header>

  {#if $lastError}
    <div class="flex items-center gap-2 border-b border-error/30 bg-error/10 px-5 py-2 text-sm text-error">
      <span class="flex-1 break-all">{$lastError}</span>
      <button
        type="button"
        on:click={() => lastError.set(null)}
        class="shrink-0 rounded p-0.5 text-error/60 hover:text-error"
        aria-label="dismiss error"
      >
        <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
      </button>
    </div>
  {/if}

  {#if $projectDir}
    <div class="flex items-center gap-2 border-b border-primary/15 bg-bg-800/50 px-5 py-1.5 text-xs">
      <span class="text-fg-mute">Project:</span>
      <span class="font-mono text-fg-dim truncate flex-1">{$projectDir}</span>
      <button
        type="button"
        on:click={closeProject}
        class="rounded p-0.5 text-fg-mute hover:text-fg"
        title="clear project directory"
        aria-label="clear project directory"
      >
        <svg xmlns="http://www.w3.org/2000/svg" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
      </button>
    </div>
  {/if}

  <div class="flex-1 overflow-y-auto px-5 py-4">
    <div class="grid gap-5 {showSettings ? 'lg:grid-cols-2' : ''}">
      <section>
        <div class="mb-3 flex items-center gap-3">
          <h2 class="flex h-5 items-center text-[10px] font-semibold uppercase tracking-wide text-fg-dim">
            Workspaces ({filtered.length})
          </h2>
          <div class="relative">
            <button
              type="button"
              on:click={() => (showSortMenu = !showSortMenu)}
              class="flex h-5 items-center gap-0.5 rounded bg-bg-600 px-2 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg"
              aria-haspopup="true"
              aria-expanded={showSortMenu}
            >
              Sort: {sortLabel(sortKey)}
              <svg class="h-2.5 w-2.5" viewBox="0 0 10 6" fill="currentColor"><path d="M0 0l5 6 5-6z"/></svg>
            </button>
            {#if showSortMenu}
              <div
                class="absolute left-0 top-full z-20 mt-1 w-40 rounded border border-bg-600/80 bg-bg-800 shadow-2xl"
                role="menu"
              >
                {#each SORT_KEYS as opt}
                  <button
                    type="button"
                    on:click={() => { sortKey = opt; showSortMenu = false; }}
                    class="block w-full text-left px-3 py-1.5 text-xs {sortKey === opt ? 'bg-accent/20 text-accent' : 'text-fg-dim hover:bg-accent/15 hover:text-fg'}"
                  >
                    {sortLabel(opt)}
                  </button>
                {/each}
              </div>
            {/if}
          </div>
        </div>
        {#if filtered.length === 0}
          {#if search}
            <div class="rounded-lg border border-dashed border-bg-600 p-8 text-center text-sm text-fg-mute">
              No workspaces matching "{search}".
            </div>
          {:else if !$projectDir}
            <div class="flex flex-col items-center justify-center py-16">
              <div class="text-4xl font-bold text-primary mb-2">dia</div>
              <p class="text-sm text-fg-mute mb-6">The Do-It-All App</p>
              <div class="flex items-center gap-3">
                <button
                  type="button"
                  on:click={openProject}
                  class="rounded bg-primary px-4 py-2 text-sm font-medium text-primary-content hover:bg-primary/90"
                >
                  Open Project
                </button>
                <button
                  type="button"
                  on:click={openNew}
                  class="rounded border border-primary/30 bg-bg-700 px-4 py-2 text-sm text-primary hover:bg-primary/10"
                >
                  New Workspace
                </button>
              </div>
              {#if recent.length > 0}
                <div class="mt-8 w-full max-w-md">
                  <h3 class="mb-2 text-[10px] font-semibold uppercase tracking-wide text-fg-mute">Recent</h3>
                  <div class="space-y-1">
                    {#each recent.slice(0, 5) as r}
                      <button
                        type="button"
                        on:click={async () => { search = r.name; await refresh(); }}
                        class="block w-full rounded px-3 py-1.5 text-left text-sm text-fg-dim hover:bg-bg-600 hover:text-fg"
                      >
                        {r.name}
                        <span class="ml-2 text-[10px] text-fg-mute">({r.count}x)</span>
                      </button>
                    {/each}
                  </div>
                </div>
              {/if}
            </div>
          {:else}
            <div class="rounded-lg border border-dashed border-bg-600 p-8 text-center text-sm text-fg-mute">
              No workspaces in this directory.
              <button type="button" on:click={openNew} class="ml-1 text-primary hover:underline">Create one</button>
              or
              <button type="button" on:click={closeProject} class="ml-1 text-primary hover:underline">change directory</button>.
            </div>
          {/if}
        {:else}
          <div class="grid grid-cols-1 gap-2 md:grid-cols-2 2xl:grid-cols-3 items-start">
            {#each filtered as w (w.path)}
              <WorkspaceCard workspace={w} onChanged={refresh} />
            {/each}
          </div>
        {/if}
      </section>

      {#if availablePlugins.length > 0}
        <section class="lg:col-span-2">
          <h2 class="mb-3 text-[10px] font-semibold uppercase tracking-wide text-fg-dim">
            Plugins ({filteredPlugins.length})
          </h2>
<div class="grid grid-cols-1 gap-2 md:grid-cols-2 2xl:grid-cols-3 items-start">
              {#each filteredPlugins as p (p.id)}
              <PluginPanel plugin={p} on:refresh={refresh} />
            {/each}
          </div>
        </section>
      {/if}

      {#if showSettings}
        <div class="lg:col-span-2">
          <SettingsPanel
            doctor={$doctor}
            paths={$paths}
            plugins={$pluginsStore}
            pluginPaths={$pluginPathsStore}
            on:close={closeSettings}
            on:themeChange={(e) => changeTheme(e.detail.id)}
            on:refresh={refresh}
          />
        </div>
      {/if}
    </div>
  </div>

  <footer class="flex items-center justify-between border-t border-primary/15 px-5 py-2 text-xs text-fg-mute">
    <span class={$workspaces.filter((w) => w.running).length > 0 ? 'text-primary font-medium' : ''}>
      {$workspaces.filter((w) => w.running).length} running
    </span>
    <div class="flex items-center gap-2">
      <span class="font-mono text-fg-mute">{$paths?.state_file ?? ''}</span>
      <span class="mx-1 h-3 w-px bg-fg-mute/30"></span>
      {#if zoomLevel !== 1.0}
        <button type="button" on:click={zoomReset} class="rounded px-1 py-0.5 text-[10px] text-fg-mute hover:text-fg">reset</button>
      {/if}
      <button type="button" on:click={zoomOut} disabled={zoomLevel <= ZOOM_MIN} class="rounded px-1 py-0.5 text-fg-dim hover:text-fg disabled:opacity-40">−</button>
      <span class="min-w-[3ch] text-center font-mono text-fg-dim">{Math.round(zoomLevel * 100)}%</span>
      <button type="button" on:click={zoomIn} disabled={zoomLevel >= ZOOM_MAX} class="rounded px-1 py-0.5 text-fg-dim hover:text-fg disabled:opacity-40">+</button>
    </div>
  </footer>
</div>

{#if showNew}
  <NewWorkspaceDialog onClose={closeNew} on:created={onNewCreated} />
{/if}

{#if selectedEditorName}
  <WorkspaceEditor
    name={selectedEditorName}
    plugins={$pluginsStore}
    onclose={onEditorClose}
    onsaved={onEditorSaved}
    ondeleted={onEditorDeleted}
  />
{/if}
