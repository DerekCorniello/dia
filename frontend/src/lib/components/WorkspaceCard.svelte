<script lang="ts">
  import { api, describeError } from '../api';
  import { pushToast, loading, plugins as pluginsStore } from '../stores';
  import type { WorkspaceInfo, PluginInfo } from '../api';
  import WorkspaceEditor from './WorkspaceEditor.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import Spinner from './Spinner.svelte';
  import { slide } from 'svelte/transition';

  export let workspace: WorkspaceInfo;
  export let onChanged: () => void = () => {};

  let busyAction: string | null = null;
  $: busy = busyAction !== null;
  let expanded = false;
  import type { AppInfo } from '../api';

  let detail: {
    description?: string;
    plugins?: string[];
    app_details: AppInfo[];
  } | null = null;

  function typeIcon(t: string): string {
    if (t === 'browser') return 'B';
    if (t === 'terminal') return 'T';
    if (t === 'open') return '^';
    if (t === 'editor') return 'E';
    return '*';
  }
  function typeBadgeClass(t: string): string {
    if (t === 'browser') return 'bg-info/15 text-info border-info/20';
    if (t === 'terminal') return 'bg-primary/15 text-primary border-primary/20';
    if (t === 'open') return 'bg-success/15 text-success border-success/20';
    return 'bg-bg-600 text-fg-mute border-bg-600';
  }
  let showEditor = false;
  let showDeleteConfirm = false;
  let showStopConfirm = false;

  // The workspace summary omits plugin IDs; expanded detail has the full list.
  $: pluginIds = (detail?.plugins ?? workspace.plugins ?? []) as string[];
  $: workspacePlugins = pluginIds
    .map((id) => $pluginsStore.find((p) => p.id === id))
    .filter((p): p is PluginInfo => p != null);

  async function start() {
    busyAction = 'start';
    try {
      await api.startWorkspace(workspace.name);
      pushToast('ok', `started ${workspace.name}`);
      onChanged();
    } catch (e) {
      pushToast('err', `start ${workspace.name}: ${describeError(e)}`);
    } finally {
      busyAction = null;
    }
  }

  async function stop() {
    busyAction = 'stop';
    try {
      await api.stopWorkspace(workspace.name);
      pushToast('ok', `stopped ${workspace.name}`);
      onChanged();
    } catch (e) {
      pushToast('err', `stop ${workspace.name}: ${describeError(e)}`);
    } finally {
      busyAction = null;
    }
  }

  async function restart() {
    busyAction = 'restart';
    try {
      await api.restartWorkspace(workspace.name);
      pushToast('ok', `restarted ${workspace.name}`);
      onChanged();
    } catch (e) {
      pushToast('err', `restart ${workspace.name}: ${describeError(e)}`);
    } finally {
      busyAction = null;
    }
  }

  async function deleteWorkspace() {
    showDeleteConfirm = true;
  }

  async function openPlugin(p: PluginInfo) {
    busyAction = 'plugin';
    try {
      await api.openWorkspacePluginWindow(workspace.name, p.id);
      pushToast('ok', `opened ${p.name || p.id}`);
    } catch (e) {
      pushToast('err', `open ${p.name || p.id}: ${describeError(e)}`);
    } finally {
      busyAction = null;
    }
  }

  async function confirmDelete() {
    showDeleteConfirm = false;
    busyAction = 'delete';
    try {
      await api.deleteWorkspace(workspace.name);
      pushToast('ok', `deleted ${workspace.name}`);
      onChanged();
    } catch (e) {
      pushToast('err', `delete workspace: ${describeError(e)}`);
    } finally {
      busyAction = null;
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
          <span
            class="inline-block h-2 w-2 shrink-0 rounded-full bg-primary"
            aria-label="running"
            role="img"
          ></span>
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
        {workspace.pluginCount && workspace.pluginCount > 0
          ? ` and ${workspace.pluginCount} plugin${workspace.pluginCount === 1 ? '' : 's'}`
          : ''}
      </p>
      {#if workspace.lastError}
        <p class="mt-1 truncate text-[10px] text-error" title={workspace.lastError}>
          {workspace.lastStatus === 'degraded'
            ? 'degraded'
            : workspace.running
              ? 'degraded'
              : 'last failure'}: {workspace.lastError}
        </p>
      {:else if workspace.lastStatus === 'degraded'}
        <p class="mt-1 text-[10px] text-warning">degraded — some apps failed</p>
      {/if}
    </button>
    <div class="flex items-center gap-1 shrink-0 pt-0.5">
      {#if workspace.running}
        <button
          type="button"
          on:click={() => (showStopConfirm = true)}
          disabled={busy || $loading}
          class="inline-flex items-center gap-1 rounded bg-error/20 px-2 py-1 text-[10px] font-medium text-error hover:bg-error/30 disabled:opacity-50"
        >
          {#if busyAction === 'stop'}<Spinner size={10} />{:else}stop{/if}
        </button>
        <button
          type="button"
          on:click={restart}
          disabled={busy || $loading}
          class="inline-flex items-center gap-1 rounded bg-bg-600 px-2 py-1 text-[10px] font-medium text-fg-dim hover:bg-primary/20 hover:text-primary disabled:opacity-50"
        >
          {#if busyAction === 'restart'}<Spinner size={10} />{:else}restart{/if}
        </button>
      {:else}
        <button
          type="button"
          on:click={start}
          disabled={busy || $loading}
          class="inline-flex items-center gap-1 rounded bg-primary/20 px-2 py-1 text-[10px] font-medium text-primary hover:bg-primary/30 disabled:opacity-50"
        >
          {#if busyAction === 'start'}<Spinner size={10} />{:else}start{/if}
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
    <div class="mt-3 border-t border-bg-600 pt-3 space-y-3" transition:slide={{ duration: 200 }}>
      <div class="flex items-center gap-2 text-[10px] text-fg-mute/60">
        <span class="shrink-0">path</span>
        <span class="flex-1 break-all font-mono text-fg-mute">{workspace.path}</span>
      </div>
      <div class="space-y-2">
        {#each detail.app_details as app, i (i)}
          <div class="rounded border border-bg-600/60 bg-bg-800/50 p-2.5">
            <div class="flex items-start justify-between gap-2">
              <div class="flex items-center gap-1.5 min-w-0">
                <span
                  class="inline-flex h-5 items-center rounded border px-1.5 text-[10px] font-medium {typeBadgeClass(
                    app.type,
                  )}"
                >
                  <span class="mr-1">{typeIcon(app.type)}</span>{app.type || 'app'}
                </span>
                {#if app.label}
                  <span class="text-xs font-medium text-fg truncate">{app.label}</span>
                {/if}
              </div>
              {#if app.type === 'browser' && app.newWindow}
                <span
                  class="shrink-0 rounded bg-info/10 px-1.5 py-0.5 text-[9px] uppercase tracking-wide text-info"
                  >new window</span
                >
              {/if}
            </div>
            {#if app.type === 'browser'}
              <div class="mt-1.5 space-y-1">
                <div class="text-xs font-mono text-fg-dim">
                  {app.browser || 'default browser'}{#if app.browser}<span class="text-fg-mute">
                      - dedicated profile</span
                    >{/if} - {app.urls?.length ?? (app.url ? 1 : 0)} url{(app.urls?.length ??
                    (app.url ? 1 : 0)) === 1
                    ? ''
                    : 's'}
                </div>
                {#if app.urls && app.urls.length > 0}
                  <ul class="space-y-0.5">
                    {#each app.urls as u, urlIndex (urlIndex)}
                      <li class="break-all font-mono text-[11px] text-fg-mute">&gt; {u}</li>
                    {/each}
                  </ul>
                {:else if app.url}
                  <div class="break-all font-mono text-[11px] text-fg-mute">&gt; {app.url}</div>
                {/if}
              </div>
            {:else if app.type === 'terminal' || app.cmd}
              <div class="mt-1.5 font-mono text-xs text-fg-dim break-all">
                {app.cmd}{app.args ? ' ' + app.args : ''}
              </div>
              {#if app.cwd}
                <div class="mt-1 font-mono text-[11px] text-fg-mute">cwd: {app.cwd}</div>
              {/if}
              {#if app.env && Object.keys(app.env).length > 0}
                <div class="mt-1 font-mono text-[11px] text-fg-mute">
                  env: {Object.entries(app.env)
                    .map(([k, v]) => k + '=' + v)
                    .join(' ')}
                </div>
              {/if}
            {:else if app.url}
              <div class="mt-1.5 break-all font-mono text-xs text-info">{app.url}</div>
            {/if}
          </div>
        {/each}
      </div>
      {#if workspacePlugins.length > 0}
        <div class="mt-2 border-t border-bg-600 pt-2">
          <div class="text-[10px] font-semibold uppercase tracking-wide text-fg-mute mb-1">
            Plugins
          </div>
          <div class="flex flex-wrap gap-1">
            {#each workspacePlugins as p (p.id)}
              <div class="flex items-center gap-1">
                <span class="text-xs text-fg-mute">{p.name || p.id}</span>
                {#if p.ui.type === 'window'}
                  <button
                    type="button"
                    on:click={() => openPlugin(p)}
                    disabled={busy || $loading}
                    class="rounded bg-bg-600 px-1.5 py-0.5 text-[10px] text-fg-dim hover:bg-primary/20 hover:text-primary disabled:opacity-50"
                  >
                    open
                  </button>
                {/if}
              </div>
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
