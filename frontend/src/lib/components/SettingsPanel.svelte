<script lang="ts">
  import { createEventDispatcher, onDestroy } from 'svelte';
  import { api, describeError, type CustomThemeInfo } from '../api';
  import { lastError, customThemes, theme as themeStore } from '../stores';
  import ThemePicker from './ThemePicker.svelte';
  import CustomThemeEditor from './CustomThemeEditor.svelte';
  import type { CheckInfo, PathsInfo } from '../api';

  export let doctor: CheckInfo[];
  export let paths: PathsInfo | null;

  const dispatch = createEventDispatcher<{
    close: void;
    themeChange: { id: string };
  }>();

  type Tab = 'theme' | 'paths' | 'doctor' | 'about';
  let activeTab: Tab = 'theme';
  let editorOpen = false;
  let editorInitial: CustomThemeInfo | null = null;
  let editorBase: string = 'dia';

  let busy = false;
  let toast: { kind: 'ok' | 'err'; text: string } | null = null;
  let toastTimer: ReturnType<typeof setTimeout> | null = null;

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
    const name = e.detail.name;
    if (!confirm(`Delete custom theme "${name}"?`)) return;
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
    if (s === 'ok') return 'bg-accent/20 text-accent';
    if (s === 'warn') return 'bg-accent-warn/20 text-accent-warn';
    return 'bg-accent-err/20 text-accent-err';
  }

  $: doctorSummary = summarize(doctor);

  const tabs: Array<{ id: Tab; label: string }> = [
    { id: 'about', label: 'About' },
    { id: 'theme', label: 'Theme' },
    { id: 'paths', label: 'Paths' },
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
  class="fixed inset-0 z-50 flex items-center justify-center bg-bg-900/70 p-4"
  on:click|self={close}
  on:keydown|self={(e) => e.key === 'Escape' && close()}
  role="presentation"
>
  <div
    class="flex max-h-[calc(100vh-2rem)] w-[min(64rem,calc(100vw-2rem))] flex-col overflow-hidden rounded-lg border border-bg-600 bg-bg-700 shadow-lg"
    role="dialog"
    aria-modal="true"
    aria-label="settings"
  >
    <div class="flex items-center justify-between border-b border-bg-600 px-4 py-3">
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
                  ? 'bg-bg-600 text-fg'
                  : 'text-fg-dim hover:bg-bg-600/50 hover:text-fg'}"
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
                  <button
                    type="button"
                    on:click={() => copy(paths.state_file)}
                    class="shrink-0 rounded bg-bg-600 px-2 py-0.5 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg"
                  >
                    copy
                  </button>
                </div>
              </dl>
            {:else}
              <p class="text-xs text-fg-mute">loading...</p>
            {/if}
          </section>
        {:else if activeTab === 'doctor'}
          <section>
            <h3 class="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-fg-mute">
              Doctor
              {#if doctor.length > 0}
                <span class="rounded bg-accent/20 px-1.5 text-[10px] text-accent">
                  {doctorSummary.ok} ok
                </span>
                {#if doctorSummary.warn > 0}
                  <span class="rounded bg-accent-warn/20 px-1.5 text-[10px] text-accent-warn">
                    {doctorSummary.warn} warn
                  </span>
                {/if}
                {#if doctorSummary.err > 0}
                  <span class="rounded bg-accent-err/20 px-1.5 text-[10px] text-accent-err">
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
      ? 'bg-accent/20 text-accent'
      : 'bg-accent-err/20 text-accent-err'}"
  >
    {toast.text}
  </div>
{/if}

{#if editorOpen}
  <div
    class="fixed inset-0 z-[55] flex items-center justify-center bg-bg-900/70 p-4"
    on:click|self={() => (editorOpen = false)}
    role="presentation"
  >
    <div
      class="flex max-h-[calc(100vh-2rem)] w-[min(48rem,calc(100vw-2rem))] flex-col overflow-y-auto rounded-lg border border-bg-600 bg-bg-700 p-4 shadow-lg"
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
