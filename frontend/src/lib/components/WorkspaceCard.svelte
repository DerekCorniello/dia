<script lang="ts">
  import { api, describeError } from '../api';
  import { lastError, loading, plugins as pluginsStore } from '../stores';
  import type { WorkspaceInfo, PluginInfo } from '../api';
  import WorkspaceEditor from './WorkspaceEditor.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import { slide } from 'svelte/transition';

  export let workspace: WorkspaceInfo;
  export let onChanged: () => void = () => {};

  let busy = false;
  let expanded = false;
  let detail: { app_details: { type: string; cmd: string; args: string; url?: string }[] } | null = null;
  let showEditor = false;
  let showDeleteConfirm = false;
  let showStopConfirm = false;

  $: workspacePlugins = (workspace.plugins ?? [])
    .map((id) => $pluginsStore.find((p) => p.id === id))
    .filter((p): p is PluginInfo => p != null);

  async function start() {
    busy = true;
    lastError.set(null);
    try {
      await api.startWorkspace(workspace.name);
      onChanged();
    } catch (e) {
      lastError.set(`start ${workspace.name}: ${describeError(e)}`);
    } finally {
      busy = false;
    }
  }

  async function stop() {
    busy = true;
    lastError.set(null);
    try {
      await api.stopWorkspace(workspace.name);
      onChanged();
    } catch (e) {
      lastError.set(`stop ${workspace.name}: ${describeError(e)}`);
    } finally {
      busy = false;
    }
  }

  async function deleteWorkspace() {
    showDeleteConfirm = true;
  }

  async function confirmDelete() {
    showDeleteConfirm = false;
    busy = true;
    lastError.set(null);
    try {
      await api.deleteWorkspace(workspace.name);
      onChanged();
    } catch (e) {
      lastError.set(`delete workspace: ${describeError(e)}`);
    } finally {
      busy = false;
    }
  }

  async function toggleExpand() {
    expanded = !expanded;
    if (expanded && !detail) {
      try {
        detail = await api.getWorkspace(workspace.name);
      } catch (e) {
        lastError.set(`load ${workspace.name}: ${describeError(e)}`);
      }
    }
  }

  function onEditorSaved() {
    showEditor = false;
    onChanged();
  }

  function onEditorDeleted() {
    showEditor = false;
    onChanged();
  }

</script>

<section
  class="relative rounded-lg border border-bg-600 bg-bg-700 p-3 transition-all duration-150 ease-out hover:-translate-y-0.5 hover:shadow-md shadow-sm {workspace.running ? 'shadow-primary/10' : ''}"
>
  <div
    class="absolute left-0 top-2 bottom-2 w-1 rounded-r {workspace.running
      ? 'bg-primary'
      : workspace.source === 'local'
        ? 'bg-info'
        : 'bg-accent-secondary'}"
  ></div>
  <header class="mb-2 flex items-center justify-between gap-2">
    <button
      type="button"
      on:click={toggleExpand}
      class="min-w-0 flex-1 text-left"
      aria-expanded={expanded}
    >
      <div class="flex flex-wrap items-center gap-2">
        <span class="text-sm font-semibold text-fg truncate">{workspace.name}</span>
        <span class="inline-flex items-center rounded-full {workspace.source === 'local' ? 'bg-info/15 text-info' : 'bg-accent-secondary/15 text-accent-secondary'} px-1.5 py-0.5 text-[10px] font-medium">
          {workspace.source}
        </span>
        {#if workspace.running}
          <span class="inline-flex items-center gap-1 text-[10px] font-medium text-primary">
            <span class="inline-block h-1.5 w-1.5 rounded-full bg-primary"></span>
            running
          </span>
        {/if}
      </div>
      {#if workspace.description}
        <p class="mt-0.5 text-xs text-fg-mute line-clamp-2">{workspace.description}</p>
      {:else}
        <p class="mt-0.5 text-xs text-fg-mute">
          {workspace.apps} app{workspace.apps === 1 ? '' : 's'}
          {#if workspace.plugins && workspace.plugins.length > 0}
            , {workspace.plugins.length} plugin{workspace.plugins.length === 1 ? '' : 's'}
          {/if}
          {#if (workspace.useCount ?? 0) > 0}
            &middot; used {workspace.useCount}x
          {/if}
        </p>
      {/if}
    </button>
    <div class="flex items-center gap-1 shrink-0">
      <button
        type="button"
        on:click={() => (showEditor = true)}
        disabled={busy || $loading}
        class="rounded bg-bg-600 px-2 py-1 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg disabled:opacity-50"
        title="edit workspace"
      >
        edit
      </button>
      <button
        type="button"
        on:click={deleteWorkspace}
        disabled={workspace.running || busy || $loading}
        class="rounded bg-error/10 px-2 py-1 text-[10px] text-error hover:bg-error/20 disabled:opacity-50"
        title="delete workspace"
      >
        delete
      </button>
      {#if workspace.running}
        <button
          type="button"
          on:click={() => (showStopConfirm = true)}
          disabled={busy || $loading}
          class="rounded bg-error/20 px-2 py-1 text-[10px] font-medium text-error hover:bg-error/30 disabled:opacity-50"
        >
          stop
        </button>
      {:else}
        <button
          type="button"
          on:click={start}
          disabled={busy || $loading}
          class="rounded bg-primary/20 px-2 py-1 text-[10px] font-medium text-primary hover:bg-primary/30 disabled:opacity-50"
        >
          {busy ? '...' : 'start'}
        </button>
      {/if}
    </div>
  </header>

  {#if expanded && detail}
    <div class="mt-3 border-t border-primary/15 pt-3" transition:slide={{ duration: 200 }}>
      <div class="text-xs text-fg-mute break-all mb-2 font-mono">{workspace.path}</div>
      <ul class="space-y-1 text-sm">
        {#each detail.app_details as app}
          <li class="flex items-baseline gap-2 font-mono">
            <span class="w-20 text-fg-dim">{app.type}</span>
            <span class="flex-1 break-all">{app.url || app.cmd}{app.args ? ' ' + app.args : ''}</span>
          </li>
        {/each}
      </ul>
      {#if workspacePlugins.length > 0}
        <div class="mt-3 border-t border-primary/15 pt-3">
          <div class="text-xs font-semibold uppercase tracking-wide text-fg-mute mb-1">Plugins</div>
          <ul class="space-y-1">
            {#each workspacePlugins as p (p.id)}
              <li class="flex items-center gap-2 text-sm">
                <span class="text-fg-dim">{p.name || p.id}</span>
                <span class="rounded bg-secondary/15 px-1 py-0.5 text-[10px] font-medium text-secondary">{p.ui.type}</span>
              </li>
            {/each}
          </ul>
        </div>
      {/if}
    </div>
  {/if}
</section>

{#if showEditor}
  <WorkspaceEditor
    name={workspace.name}
    plugins={$pluginsStore}
    onclose={() => (showEditor = false)}
    onsaved={onEditorSaved}
    ondeleted={onEditorDeleted}
  />
{/if}

{#if showDeleteConfirm}
  <ConfirmDialog
    title="Delete workspace"
    message="Delete workspace &quot;{workspace.name}&quot;? This cannot be undone."
    confirmLabel="Delete"
    on:confirm={confirmDelete}
    on:cancel={() => (showDeleteConfirm = false)}
  />
{/if}

{#if showStopConfirm}
  <ConfirmDialog
    title="Stop workspace"
    message="Stop workspace &quot;{workspace.name}&quot;? Running apps will be terminated."
    confirmLabel="Stop"
    on:confirm={() => { showStopConfirm = false; stop(); }}
    on:cancel={() => (showStopConfirm = false)}
  />
{/if}
