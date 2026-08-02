<script lang="ts">
  import { api, describeError } from '../api';
  import { pushToast, loading, plugins as pluginsStore } from '../stores';
  import type { WorkspaceInfo, PluginInfo } from '../api';
  import WorkspaceEditor from './WorkspaceEditor.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import { slide } from 'svelte/transition';

  export let workspace: WorkspaceInfo;
  export let onChanged: () => void = () => {};

  let busy = false;
  let expanded = false;
  let detail: { app_details: { type: string; cmd: string; args: string; url?: string }[] } | null =
    null;
  let showEditor = false;
  let showDeleteConfirm = false;
  let showStopConfirm = false;

  $: workspacePlugins = (workspace.plugins ?? [])
    .map((id) => $pluginsStore.find((p) => p.id === id))
    .filter((p): p is PluginInfo => p != null);

  async function start() {
    busy = true;
    try {
      await api.startWorkspace(workspace.name);
      pushToast('ok', `started ${workspace.name}`);
      onChanged();
    } catch (e) {
      pushToast('err', `start ${workspace.name}: ${describeError(e)}`);
    } finally {
      busy = false;
    }
  }

  async function stop() {
    busy = true;
    try {
      await api.stopWorkspace(workspace.name);
      pushToast('ok', `stopped ${workspace.name}`);
      onChanged();
    } catch (e) {
      pushToast('err', `stop ${workspace.name}: ${describeError(e)}`);
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
    try {
      await api.deleteWorkspace(workspace.name);
      pushToast('ok', `deleted ${workspace.name}`);
      onChanged();
    } catch (e) {
      pushToast('err', `delete workspace: ${describeError(e)}`);
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
        pushToast('err', `load ${workspace.name}: ${describeError(e)}`);
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
  class="relative rounded-lg border {workspace.running
    ? 'border-primary/40'
    : 'border-bg-600'} bg-bg-700 p-3 shadow-sm hover:shadow-md transition-shadow duration-150"
>
  <div class="flex items-start justify-between gap-3">
    <button
      type="button"
      on:click={toggleExpand}
      class="min-w-0 flex-1 text-left"
      aria-expanded={expanded}
    >
      <div class="flex items-center gap-2">
        {#if workspace.running}
          <span class="inline-block h-2 w-2 shrink-0 rounded-full bg-primary"></span>
        {/if}
        <span class="text-sm font-semibold text-fg">{workspace.name}</span>
      </div>
      {#if workspace.description}
        <p class="mt-0.5 text-xs text-fg-dim line-clamp-2">{workspace.description}</p>
      {:else}
        <p class="mt-0.5 text-xs italic text-fg-mute/50">No description</p>
      {/if}
      <p class="mt-1 text-[10px] text-fg-mute">
        {workspace.apps} app{workspace.apps === 1 ? '' : 's'}
        {#if workspace.plugins && workspace.plugins.length > 0}
          {' '}and {workspace.plugins.length} plugin{workspace.plugins.length === 1 ? '' : 's'}
        {/if}
      </p>
    </button>
    <div class="flex items-center gap-1 shrink-0 pt-0.5">
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
      <button
        type="button"
        on:click={() => (showEditor = true)}
        disabled={busy || $loading}
        class="rounded bg-bg-600 px-2 py-1 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg disabled:opacity-50"
      >
        edit
      </button>
      <button
        type="button"
        on:click={deleteWorkspace}
        disabled={workspace.running || busy || $loading}
        class="rounded px-2 py-1 text-[10px] text-fg-dim hover:bg-error/10 hover:text-error disabled:opacity-30"
      >
        del
      </button>
    </div>
  </div>

  {#if expanded && detail}
    <div class="mt-3 border-t border-bg-600 pt-3" transition:slide={{ duration: 200 }}>
      <div class="text-xs text-fg-mute/60 break-all font-mono">{workspace.path}</div>
      <ul class="mt-2 space-y-0.5 text-xs">
        {#each detail.app_details as app, i (i)}
          <li class="flex items-baseline gap-2 font-mono">
            <span class="w-16 text-fg-mute shrink-0">{app.type}</span>
            <span class="flex-1 break-all text-fg-dim"
              >{app.url || app.cmd}{app.args ? ' ' + app.args : ''}</span
            >
          </li>
        {/each}
      </ul>
      {#if workspacePlugins.length > 0}
        <div class="mt-2 border-t border-bg-600 pt-2">
          <div class="text-[10px] font-semibold uppercase tracking-wide text-fg-mute mb-1">
            Plugins
          </div>
          <div class="flex flex-wrap gap-1">
            {#each workspacePlugins as p (p.id)}
              <span class="text-xs text-fg-mute">{p.name || p.id}</span>
            {/each}
          </div>
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
    on:confirm={() => {
      showStopConfirm = false;
      stop();
    }}
    on:cancel={() => (showStopConfirm = false)}
  />
{/if}
