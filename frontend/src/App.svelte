<script lang="ts">
  import { api, describeError } from './lib/api';
  import {
    workspaces,
    instances,
    doctor,
    paths,
    loading,
    lastError,
  } from './lib/stores';
  import WorkspaceCard from './lib/components/WorkspaceCard.svelte';
  import InstanceRow from './lib/components/InstanceRow.svelte';
  import SettingsPanel from './lib/components/SettingsPanel.svelte';
  import NewWorkspaceDialog from './lib/components/NewWorkspaceDialog.svelte';

  let showSettings = false;
  let showNew = false;
  let plugins: string[] = [];

  async function refresh() {
    loading.set(true);
    lastError.set(null);
    try {
      const [ws, inst, doc, p, pl] = await Promise.all([
        api.listWorkspaces(),
        api.listInstances(),
        api.doctor(),
        api.paths(),
        api.plugins(),
      ]);
      workspaces.set(ws);
      instances.set(inst);
      doctor.set(doc);
      paths.set(p);
      plugins = pl;
    } catch (e) {
      lastError.set(`refresh: ${describeError(e)}`);
    } finally {
      loading.set(false);
    }
  }

  async function stopAll() {
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
  }

  $: $lastError;

  import { onMount } from 'svelte';
  onMount(refresh);
</script>

<div class="flex h-screen flex-col">
  <header class="flex items-center justify-between border-b border-bg-600 px-5 py-3">
    <div class="flex items-baseline gap-3">
      <h1 class="text-xl font-semibold">dia</h1>
      <span class="text-xs text-fg-mute uppercase tracking-wide">dev workspace launcher</span>
    </div>
    <div class="flex items-center gap-2">
      <button
        type="button"
        on:click={refresh}
        disabled={$loading}
        class="rounded bg-bg-600 px-3 py-1.5 text-xs hover:bg-bg-600/70 disabled:opacity-50"
        title="refresh"
      >
        {$loading ? '...' : 'Refresh'}
      </button>
      <button
        type="button"
        on:click={() => (showNew = true)}
        class="rounded bg-accent/20 px-3 py-1.5 text-xs font-medium text-accent hover:bg-accent/30"
      >
        + New
      </button>
      <button
        type="button"
        on:click={() => (showSettings = !showSettings)}
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
              on:click={() => (showNew = true)}
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
              on:click={stopAll}
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
            {plugins}
            onRefresh={refresh}
            onClose={() => (showSettings = false)}
          />
        </div>
      {/if}
    </div>
  </div>

  <footer class="flex items-center justify-between border-t border-bg-600 px-5 py-2 text-xs text-fg-mute">
    <span>
      {$instances.filter((i) => i.status === 'running').length} running
      {plugins.length > 0 ? ` | ${plugins.length} plugin${plugins.length === 1 ? '' : 's'}` : ''}
    </span>
    <span class="font-mono">{$paths?.state_file ?? ''}</span>
  </footer>
</div>

{#if showNew}
  <NewWorkspaceDialog onClose={() => (showNew = false)} />
{/if}
