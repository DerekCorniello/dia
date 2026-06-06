<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api, describeError } from './lib/api';
  import {
    workspaces,
    instances,
    doctor,
    paths,
    loading,
    lastError,
    theme,
    customThemes,
  } from './lib/stores';
  import { buildAllCustomThemesCss } from './lib/colors';
  import WorkspaceCard from './lib/components/WorkspaceCard.svelte';
  import InstanceRow from './lib/components/InstanceRow.svelte';
  import SettingsPanel from './lib/components/SettingsPanel.svelte';
  import NewWorkspaceDialog from './lib/components/NewWorkspaceDialog.svelte';

  let showSettings = false;
  let showNew = false;

  let customThemeStyle: HTMLStyleElement | null = null;

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

  onDestroy(() => {
    customThemeStyle?.remove();
    customThemeStyle = null;
  });

  async function refresh() {
    loading.set(true);
    lastError.set(null);
    try {
      const [ws, inst, doc, p, ct] = await Promise.all([
        api.listWorkspaces(),
        api.listInstances(),
        api.doctor(),
        api.paths(),
        api.listCustomThemes(),
      ]);
      workspaces.set(ws);
      instances.set(inst);
      doctor.set(doc);
      paths.set(p);
      customThemes.set(ct);
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

  function openNew() {
    showNew = true;
  }

  function closeNew() {
    showNew = false;
  }

  function toggleSettings() {
    showSettings = !showSettings;
  }

  function closeSettings() {
    showSettings = false;
  }

  onMount(async () => {
    try {
      const [t, ct] = await Promise.all([api.getTheme(), api.listCustomThemes()]);
      theme.set(t);
      document.documentElement.dataset.theme = t;
      customThemes.set(ct);
    } catch (e) {
      // fall back to default theme
    }
    await refresh();
  });
</script>

<div class="flex h-screen flex-col">
  <header class="flex items-center border-b border-bg-600 px-5 py-3">
    <div class="flex items-baseline gap-3 flex-1">
      <h1 class="text-xl font-semibold">dia</h1>
      <span class="text-xs text-fg-mute uppercase tracking-wide">the do-it-all app</span>
    </div>
    <div class="flex-1 flex justify-center">
      <button
        type="button"
        on:click={openNew}
        class="rounded bg-accent/20 px-3 py-1.5 text-xs font-medium text-accent hover:bg-accent/30"
      >
        + New
      </button>
    </div>
    <div class="flex items-center gap-2 flex-1 justify-end">
      <button
        type="button"
        on:click={toggleSettings}
        class="rounded bg-bg-600 px-3 py-1.5 text-xs hover:bg-bg-600/70"
      >
        {showSettings ? 'Hide' : 'Settings'}
      </button>
    </div>
  </header>

  {#if $lastError}
    <div class="border-b border-accent-err/30 bg-accent-err/10 px-5 py-2 text-sm text-accent-err">
      {$lastError}
    </div>
  {/if}

  <div class="flex-1 overflow-y-auto px-5 py-4">
    <div class="grid gap-5 {showSettings ? 'lg:grid-cols-2' : ''}">
      <section>
        <div class="flex items-baseline justify-between mb-3">
          <h2 class="text-sm font-semibold uppercase tracking-wide text-fg-dim">
            Workspaces ({$workspaces.length})
          </h2>
        </div>
        {#if $workspaces.length === 0}
          <div class="rounded-lg border border-dashed border-bg-600 p-8 text-center text-sm text-fg-mute">
            No workspaces yet.
            <button
              type="button"
              on:click={openNew}
              class="ml-1 text-accent hover:underline"
            >
              Create one
            </button>
            to get started.
          </div>
        {:else}
          <div class="space-y-2">
            {#each $workspaces as w (w.path)}
              <WorkspaceCard workspace={w} onChanged={refresh} />
            {/each}
          </div>
        {/if}
      </section>

      <section>
        <div class="flex items-baseline justify-between mb-3">
          <h2 class="text-sm font-semibold uppercase tracking-wide text-fg-dim">
            Instances ({$instances.length})
          </h2>
          {#if $instances.some((i) => i.status === 'running')}
            <button
              type="button"
              on:click={async () => {
                loading.set(true);
                lastError.set(null);
                try {
                  await api.stopAll();
                  await refresh();
                } catch (e) {
                  lastError.set(`stop all: ${describeError(e)}`);
                } finally {
                  loading.set(false);
                }
              }}
              disabled={$loading}
              class="text-xs text-accent-err hover:underline disabled:opacity-50"
            >
              Stop all
            </button>
          {/if}
        </div>
        {#if $instances.length === 0}
          <div class="rounded-lg border border-dashed border-bg-600 p-8 text-center text-sm text-fg-mute">
            Nothing started.
          </div>
        {:else}
          <div class="space-y-2">
            {#each $instances as inst (inst.id)}
              <InstanceRow {inst} onChanged={refresh} />
            {/each}
          </div>
        {/if}
      </section>

      {#if showSettings}
        <div class="lg:col-span-2">
          <SettingsPanel
            doctor={$doctor}
            paths={$paths}
            on:close={closeSettings}
            on:themeChange={(e) => changeTheme(e.detail.id)}
          />
        </div>
      {/if}
    </div>
  </div>

  <footer class="flex items-center justify-between border-t border-bg-600 px-5 py-2 text-xs text-fg-mute">
    <span>{$instances.filter((i) => i.status === 'running').length} running</span>
    <span class="font-mono">{$paths?.state_file ?? ''}</span>
  </footer>
</div>

{#if showNew}
  <NewWorkspaceDialog onClose={closeNew} />
{/if}
