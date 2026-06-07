<script lang="ts">
  import { createEventDispatcher, onDestroy, onMount } from 'svelte';
  import { api, describeError, type CustomThemeInfo, type PluginInfo, type PluginPathsInfo } from '../api';
  import { lastError, customThemes, theme as themeStore, plugins as pluginsStore, pluginPaths as pluginPathsStore, keybinds as keybindsStore } from '../stores';
  import ThemePicker from './ThemePicker.svelte';
  import CustomThemeEditor from './CustomThemeEditor.svelte';
  import PluginsPanel from './PluginsPanel.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import type { CheckInfo, PathsInfo } from '../api';

  export let doctor: CheckInfo[];
  export let paths: PathsInfo | null;
  export let plugins: PluginInfo[] = [];
  export let pluginPaths: PluginPathsInfo | null = null;

  const dispatch = createEventDispatcher<{
    close: void;
    themeChange: { id: string };
    refresh: void;
  }>();

  type Tab = 'theme' | 'keybinds' | 'paths' | 'plugins' | 'doctor' | 'about';
  let activeTab: Tab = 'theme';
  let editorOpen = false;
  let editorInitial: CustomThemeInfo | null = null;
  let editorBase: string = 'dia';

  let busy = false;
  let toast: { kind: 'ok' | 'err'; text: string } | null = null;
  let toastTimer: ReturnType<typeof setTimeout> | null = null;
  let showDeleteThemeConfirm = false;
  let deleteThemeName = '';

  // Keybinds state
  let recording: string | null = null;
  let pressedKeys = new Set<string>();

  // Detect platform for modifier display.
  let modKey = 'Ctrl';

  function buildDefaults(mod: string): Record<string, string> {
    return {
      'Focus search': '/',
      'New workspace': `${mod}+N`,
      'Toggle settings': `${mod}+,`,
      'Refresh': `${mod}+R`,
      'Close dialog': 'Escape',
      'Zoom in': `${mod}+=`,
      'Zoom out': `${mod}+-`,
      'Zoom reset': `${mod}+0`,
    };
  }

  $: defaultKeybinds = buildDefaults(modKey);
  $: userOverrides = (() => {
    const out: Record<string, string> = {};
    for (const [action, def] of Object.entries(defaultKeybinds)) {
      const cur = $keybindsStore[action];
      if (cur && cur !== def) out[action] = cur;
    }
    for (const [action, val] of Object.entries($keybindsStore)) {
      if (!(action in defaultKeybinds)) out[action] = val;
    }
    return out;
  })();
  $: keybinds = { ...defaultKeybinds, ...userOverrides };

  onMount(async () => {
    try {
      modKey = navigator.platform.includes('Mac') ? 'Cmd' : 'Ctrl';
    } catch {
      modKey = 'Ctrl';
    }
    try {
      const overrides = await api.getKeybindings();
      keybindsStore.set(overrides);
    } catch {
      keybindsStore.set({});
    }
  });

  function onRecordKeydown(e: KeyboardEvent) {
    if (!recording) return;
    e.preventDefault();
    e.stopPropagation();
    pressedKeys.add(e.key);
  }

  function onRecordKeyup(e: KeyboardEvent) {
    if (!recording) return;
    e.preventDefault();
    e.stopPropagation();
    pressedKeys.delete(e.key);

    const hasNonMod = [...pressedKeys].some((k) => !['Control', 'Meta', 'Alt', 'Shift'].includes(k));
    if (hasNonMod) return;

    const parts: string[] = [];
    if (pressedKeys.has('Control')) parts.push('Ctrl');
    if (pressedKeys.has('Meta')) parts.push('Cmd');
    if (pressedKeys.has('Alt')) parts.push('Alt');
    if (pressedKeys.has('Shift')) parts.push('Shift');
    if (e.key && !['Control', 'Meta', 'Alt', 'Shift'].includes(e.key)) {
      const display = e.key === ',' ? ',' : e.key === '.' ? '.' : e.key.length === 1 ? e.key.toUpperCase() : e.key;
      parts.push(display);
    }
    if (parts.length === 0) return;
    const action = recording;
    recording = null;
    pressedKeys.clear();
    saveKeybinding(action, parts.join('+'));
  }

  async function saveKeybinding(action: string, combo: string) {
    const def = defaultKeybinds[action];
    try {
      const next = { ...$keybindsStore };
      if (combo === def) {
        delete next[action];
        await api.setKeybinding(action, '');
      } else {
        next[action] = combo;
        await api.setKeybinding(action, combo);
      }
      keybindsStore.set(next);
    } catch (e) {
      showToast('err', `save keybinding: ${describeError(e)}`);
    }
  }

  async function resetKeybindings() {
    try {
      await api.resetKeybindings();
      keybindsStore.set({});
      showToast('ok', 'reset keybindings');
    } catch (e) {
      showToast('err', `reset: ${describeError(e)}`);
    }
  }

  onDestroy(() => {
    if (toastTimer) clearTimeout(toastTimer);
  });

  function showToast(kind: 'ok' | 'err', text: string) {
    toast = { kind, text };
    if (toastTimer) clearTimeout(toastTimer);
    toastTimer = setTimeout(() => {
      toast = null;
    }, 2500);
  }

  function close() {
    dispatch('close');
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      if (editorOpen) {
        editorOpen = false;
        return;
      }
      close();
    }
  }

  function selectTheme(e: CustomEvent<{ id: string }>) {
    const id = e.detail.id;
    themeStore.set(id);
    document.documentElement.dataset.theme = id;
    dispatch('themeChange', { id });
    api.setTheme(id).catch((err) => {
      lastError.set(`theme: ${describeError(err)}`);
    });
  }

  function openNewCustom() {
    editorInitial = null;
    editorBase = $themeStore;
    editorOpen = true;
  }

  function openEditCustom(e: CustomEvent<{ name: string }>) {
    const name = e.detail.name;
    const found = $customThemes.find((t) => t.name === name);
    if (!found) return;
    editorInitial = found;
    editorBase = name;
    editorOpen = true;
  }

  async function saveCustom(e: CustomEvent<CustomThemeInfo>) {
    const info = e.detail;
    try {
      await api.setCustomTheme(info);
      const list = await api.listCustomThemes();
      customThemes.set(list);
      themeStore.set(info.name);
      document.documentElement.dataset.theme = info.name;
      dispatch('themeChange', { id: info.name });
      await api.setTheme(info.name);
      editorOpen = false;
      showToast('ok', `saved ${info.name}`);
    } catch (err) {
      showToast('err', `save: ${describeError(err)}`);
    }
  }

  function cancelCustom() {
    editorOpen = false;
  }

  async function deleteCustom(e: CustomEvent<{ name: string }>) {
    deleteThemeName = e.detail.name;
    showDeleteThemeConfirm = true;
  }

  async function confirmDeleteTheme() {
    showDeleteThemeConfirm = false;
    const name = deleteThemeName;
    try {
      await api.deleteCustomTheme(name);
      const list = await api.listCustomThemes();
      customThemes.set(list);
      if ($themeStore === name) {
        themeStore.set('dia');
        document.documentElement.dataset.theme = 'dia';
        dispatch('themeChange', { id: 'dia' });
        await api.setTheme('dia');
      }
      showToast('ok', `deleted ${name}`);
    } catch (err) {
      showToast('err', `delete: ${describeError(err)}`);
    }
  }

  async function openConfig() {
    busy = true;
    lastError.set(null);
    try {
      await api.openConfigFolder();
      showToast('ok', 'opened config folder');
    } catch (e) {
      lastError.set(`open config: ${describeError(e)}`);
      showToast('err', 'open config failed');
    } finally {
      busy = false;
    }
  }

  async function openState() {
    busy = true;
    lastError.set(null);
    try {
      await api.openStateFolder();
      showToast('ok', 'opened state folder');
    } catch (e) {
      lastError.set(`open state: ${describeError(e)}`);
      showToast('err', 'open state failed');
    } finally {
      busy = false;
    }
  }

  function copy(text: string) {
    navigator.clipboard?.writeText(text).then(
      () => showToast('ok', 'copied'),
      () => showToast('err', 'copy failed'),
    );
  }

  function statusClass(s: string): string {
    if (s === 'ok') return 'bg-success/20 text-success';
    if (s === 'warn') return 'bg-warning/20 text-warning';
    return 'bg-error/20 text-error';
  }

  $: doctorSummary = summarize(doctor);

  const tabs: Array<{ id: Tab; label: string }> = [
    { id: 'about', label: 'About' },
    { id: 'theme', label: 'Theme' },
    { id: 'keybinds', label: 'Keybinds' },
    { id: 'paths', label: 'Paths' },
    { id: 'plugins', label: 'Plugins' },
    { id: 'doctor', label: 'Doctor' },
  ];

  function summarize(rows: CheckInfo[]) {
    let ok = 0;
    let warn = 0;
    let err = 0;
    for (const r of rows) {
      if (r.status === 'ok') ok++;
      else if (r.status === 'warn') warn++;
      else err++;
    }
    return { ok, warn, err };
  }
</script>

<svelte:window on:keydown={onKey} />

  <div
    class="fixed inset-0 z-50 flex items-center justify-center bg-bg-900/80 p-4"
    on:click|self={close}
    on:keydown|self={(e) => e.key === 'Escape' && close()}
    role="presentation"
  >
    <div
      class="flex max-h-[calc(100vh-2rem)] w-[min(64rem,calc(100vw-2rem))] flex-col overflow-hidden rounded-lg border border-primary/15 bg-bg-700 shadow-lg"
      role="dialog"
      aria-modal="true"
      aria-label="settings"
    >
      <div class="flex items-center justify-between border-b border-primary/15 px-4 py-3">
        <h2 class="text-sm font-semibold uppercase tracking-wide text-fg-dim">Settings</h2>
      <button
        type="button"
        on:click={close}
        class="rounded p-1 text-fg-mute hover:bg-bg-600 hover:text-fg"
        aria-label="close settings"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <line x1="18" y1="6" x2="6" y2="18" />
          <line x1="6" y1="6" x2="18" y2="18" />
        </svg>
      </button>
    </div>

    <div class="flex min-h-0 flex-1">
      <nav class="w-40 shrink-0 border-r border-bg-600 bg-bg-800 p-2">
        <ul class="space-y-0.5">
          {#each tabs as item (item.id)}
            <li>
              <button
                type="button"
                on:click={() => (activeTab = item.id)}
                class="block w-full rounded px-2 py-1.5 text-left text-sm {activeTab === item.id
                  ? 'border-l-2 border-primary bg-primary/15 text-primary font-medium'
                  : 'text-fg-dim hover:bg-primary/10 hover:text-fg'}"
                aria-current={activeTab === item.id ? 'page' : undefined}
              >
                {item.label}
                {#if item.id === 'doctor' && doctor.length > 0}
                  <span class="ml-1 text-[10px] text-fg-mute">
                    ({doctorSummary.ok}/{doctor.length})
                  </span>
                {/if}
              </button>
            </li>
          {/each}
        </ul>
      </nav>

      <div class="min-h-0 flex-1 overflow-y-auto p-4">
        {#if activeTab === 'theme'}
          <ThemePicker
            activeTheme={$themeStore}
            on:select={selectTheme}
            on:editCustom={openEditCustom}
            on:newCustom={openNewCustom}
            on:deleteCustom={deleteCustom}
          />
        {:else if activeTab === 'paths'}
          <section>
            <h3 class="mb-2 text-xs font-semibold uppercase tracking-wide text-fg-mute">
              Paths
            </h3>
            {#if paths}
              <dl class="space-y-2 text-sm">
                <div class="flex flex-col gap-1 sm:flex-row sm:items-center">
                  <dt class="w-32 shrink-0 text-fg-mute">global config</dt>
                  <dd class="flex-1 break-all font-mono text-xs text-fg-dim">{paths.global_config_dir}</dd>
                  <div class="flex shrink-0 gap-1">
                    <button
                      type="button"
                      on:click={() => openConfig()}
                      disabled={busy}
                      class="rounded bg-bg-600 px-2 py-0.5 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg disabled:opacity-50"
                    >
                      open
                    </button>
                    <button
                      type="button"
                      on:click={() => copy(paths.global_config_dir)}
                      class="rounded bg-bg-600 px-2 py-0.5 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg"
                    >
                      copy
                    </button>
                  </div>
                </div>
                <div class="flex flex-col gap-1 sm:flex-row sm:items-center">
                  <dt class="w-32 shrink-0 text-fg-mute">state dir</dt>
                  <dd class="flex-1 break-all font-mono text-xs text-fg-dim">{paths.state_dir}</dd>
                  <div class="flex shrink-0 gap-1">
                    <button
                      type="button"
                      on:click={() => openState()}
                      disabled={busy}
                      class="rounded bg-bg-600 px-2 py-0.5 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg disabled:opacity-50"
                    >
                      open
                    </button>
                    <button
                      type="button"
                      on:click={() => copy(paths.state_dir)}
                      class="rounded bg-bg-600 px-2 py-0.5 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg"
                    >
                      copy
                    </button>
                  </div>
                </div>
                <div class="flex flex-col gap-1 sm:flex-row sm:items-center">
                  <dt class="w-32 shrink-0 text-fg-mute">state file</dt>
                  <dd class="flex-1 break-all font-mono text-xs text-fg-dim">{paths.state_file}</dd>
                  <div class="flex shrink-0 gap-1">
                    <button
                      type="button"
                      on:click={() => api.openStateFile()}
                      disabled={busy}
                      class="rounded bg-bg-600 px-2 py-0.5 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg disabled:opacity-50"
                    >
                      open
                    </button>
                    <button
                      type="button"
                      on:click={() => copy(paths.state_file)}
                      class="rounded bg-bg-600 px-2 py-0.5 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg"
                    >
                      copy
                    </button>
                  </div>
                </div>
              </dl>
            {:else}
              <p class="text-xs text-fg-mute">loading...</p>
            {/if}
            {#if pluginPaths}
              <div class="mt-4">
                <h4 class="mb-2 text-xs font-semibold uppercase tracking-wide text-fg-mute">Plugin directories</h4>
                <dl class="space-y-2 text-sm">
                  <div class="flex flex-col gap-1 sm:flex-row sm:items-center">
                    <dt class="w-32 shrink-0 text-fg-mute">global</dt>
                    <dd class="flex-1 break-all font-mono text-xs text-fg-dim">{pluginPaths.globalDir}</dd>
                    <div class="flex shrink-0 gap-1">
                      <button
                        type="button"
                        on:click={() => api.openPluginFolder()}
                        class="rounded bg-bg-600 px-2 py-0.5 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg"
                      >
                        open
                      </button>
                      <button
                        type="button"
                        on:click={() => copy(pluginPaths.globalDir)}
                        class="rounded bg-bg-600 px-2 py-0.5 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg"
                      >
                        copy
                      </button>
                    </div>
                  </div>
                  {#if pluginPaths.localDir}
                    {@const local = pluginPaths.localDir}
                    <div class="flex flex-col gap-1 sm:flex-row sm:items-center">
                      <dt class="w-32 shrink-0 text-fg-mute">local</dt>
                      <dd class="flex-1 break-all font-mono text-xs text-fg-dim">{local}</dd>
                      <div class="flex shrink-0 gap-1">
                        <button
                          type="button"
                          on:click={() => api.revealPath(local)}
                          class="rounded bg-bg-600 px-2 py-0.5 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg"
                        >
                          open
                        </button>
                        <button
                          type="button"
                          on:click={() => copy(local)}
                          class="rounded bg-bg-600 px-2 py-0.5 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg"
                        >
                          copy
                        </button>
                      </div>
                    </div>
                  {/if}
                </dl>
              </div>
            {/if}
          </section>
        {:else if activeTab === 'keybinds'}
          <section>
            <h3 class="mb-2 text-xs font-semibold uppercase tracking-wide text-fg-mute">Keybindings</h3>
            <!-- svelte-ignore a11y-no-static-element-interactions -->
            <div class="space-y-1 text-sm" on:keydown={onRecordKeydown} on:keyup={onRecordKeyup}>
              {#each Object.entries(defaultKeybinds) as [action, def]}
                {@const current = keybinds[action] || def}
                <div class="flex items-center gap-2">
                  <span class="w-40 text-fg-dim">{action}</span>
                  <button
                    type="button"
                    on:click={() => recording = recording === action ? null : action}
                    class="flex-1 rounded border border-bg-600 bg-bg-800 px-2 py-1 text-xs font-mono text-left text-fg-dim hover:border-accent/50"
                  >
                    {#if recording === action}
                      <span class="text-accent">press keys...</span>
                    {:else}
                      {current}
{/if}

{#if showDeleteThemeConfirm}
  <ConfirmDialog
    title="Delete theme"
    message="Delete custom theme &quot;{deleteThemeName}&quot;? This cannot be undone."
    confirmLabel="Delete"
    on:confirm={confirmDeleteTheme}
    on:cancel={() => (showDeleteThemeConfirm = false)}
  />
{/if}
                  </button>
                  {#if current !== def}
                    <button
                      type="button"
                      on:click={() => saveKeybinding(action, def)}
                      class="text-[10px] text-fg-mute hover:text-fg"
                    >
                      reset
                    </button>
                  {/if}
                </div>
              {/each}
            </div>
            <button
              type="button"
              on:click={resetKeybindings}
              class="mt-3 rounded bg-bg-600 px-2 py-0.5 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg"
            >
              Reset all to defaults
            </button>
          </section>
        {:else if activeTab === 'doctor'}
          <section>
            <h3 class="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-fg-mute">
              Doctor
              {#if doctor.length > 0}
                <span class="rounded bg-success/20 px-1.5 text-[10px] text-success">
                  {doctorSummary.ok} ok
                </span>
                {#if doctorSummary.warn > 0}
                  <span class="rounded bg-warning/20 px-1.5 text-[10px] text-warning">
                    {doctorSummary.warn} warn
                  </span>
                {/if}
                {#if doctorSummary.err > 0}
                  <span class="rounded bg-error/20 px-1.5 text-[10px] text-error">
                    {doctorSummary.err} err
                  </span>
                {/if}
              {/if}
            </h3>
            {#if doctor.length === 0}
              <p class="text-xs text-fg-mute">loading...</p>
            {:else}
              <ul class="space-y-1 text-sm">
                {#each doctor as c (c.name)}
                  <li class="flex items-start gap-2">
                    <span
                      class="mt-0.5 inline-block w-12 shrink-0 rounded px-1 text-center text-[10px] font-semibold uppercase {statusClass(
                        c.status,
                      )}"
                    >
                      {c.status}
                    </span>
                    <span class="w-28 shrink-0 truncate text-fg" title={c.name}>{c.name}</span>
                    <span class="flex-1 break-all font-mono text-xs text-fg-dim">{c.detail ?? ''}</span>
                  </li>
                {/each}
              </ul>
            {/if}
          </section>
        {:else if activeTab === 'plugins'}
          <PluginsPanel
            plugins={plugins}
            on:refresh={() => dispatch('refresh')}
          />
        {:else if activeTab === 'about'}
          <section>
            <h3 class="mb-2 text-xs font-semibold uppercase tracking-wide text-fg-mute">About</h3>
            <p class="mb-2 text-sm text-fg-dim">
              <span class="font-semibold text-fg">dia</span> is a deterministic workspace
              launcher. Define a workspace in YAML, click start, and dia brings up the
              editor, terminal, browser, and services for that project.
            </p>
            <p class="text-sm text-fg-dim">
              See
              <a
                href="https://github.com/DerekCorniello/dia"
                class="text-accent hover:underline"
                on:click|preventDefault={() => {
                  window.open('https://github.com/DerekCorniello/dia', '_blank');
                }}
              >
                github.com/DerekCorniello/dia
              </a>
              for documentation and the roadmap.
            </p>
          </section>
        {/if}
      </div>
    </div>

    <div class="flex items-center justify-end border-t border-bg-600 px-4 py-2">
      <span class="text-[10px] text-fg-mute">esc to close</span>
    </div>
  </div>
</div>

{#if toast}
  <div
    class="pointer-events-none fixed bottom-4 left-1/2 z-[60] -translate-x-1/2 rounded px-3 py-1.5 text-xs shadow {toast.kind ===
    'ok'
      ? 'border-l-2 border-success bg-success/15 text-success'
      : 'border-l-2 border-error bg-error/15 text-error'}"
  >
    {toast.text}
  </div>
{/if}

{#if editorOpen}
  <div
    class="fixed inset-0 z-[55] flex items-center justify-center bg-bg-900/80 p-4"
    on:click|self={() => (editorOpen = false)}
    role="presentation"
  >
    <div
      class="flex max-h-[calc(100vh-2rem)] w-[min(48rem,calc(100vw-2rem))] flex-col overflow-y-auto rounded-lg border border-primary/15 bg-bg-700 p-4 shadow-lg"
      role="dialog"
      aria-modal="true"
      aria-label="custom theme editor"
    >
      <div class="mb-3 flex items-center justify-between">
        <h3 class="text-sm font-semibold uppercase tracking-wide text-fg-dim">
          {editorInitial ? 'Edit custom theme' : 'New custom theme'}
        </h3>
        <button
          type="button"
          on:click={() => (editorOpen = false)}
          class="rounded p-1 text-fg-mute hover:bg-bg-600 hover:text-fg"
          aria-label="close editor"
        >
          x
        </button>
      </div>
      <CustomThemeEditor
        initial={editorInitial}
        baseThemeId={editorBase}
        on:save={saveCustom}
        on:cancel={cancelCustom}
      />
    </div>
  </div>
{/if}
