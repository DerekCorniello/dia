<script lang="ts">
  import { api, describeError } from '../api';
  import { instances, lastError, loading } from '../stores';
  import type { WorkspaceInfo, InstanceInfo } from '../api';

  export let workspace: WorkspaceInfo;
  export let onChanged: () => void = () => {};

  let busy = false;
  let expanded = false;
  let detail: { app_details: { type: string; cmd: string; args: string; url?: string }[] } | null = null;

  $: instance = $instances.find((i) => i.workspace === workspace.name && i.status === 'running');
  $: hasRunning = !!instance;

  async function start() {
    busy = true;
    lastError.set(null);
    try {
      const inst: InstanceInfo = await api.startWorkspace(workspace.name);
      instances.update((arr) => [inst, ...arr.filter((i) => i.id !== inst.id)]);
      onChanged();
    } catch (e) {
      lastError.set(`start ${workspace.name}: ${describeError(e)}`);
    } finally {
      busy = false;
    }
  }

  async function stop() {
    if (!instance) return;
    busy = true;
    lastError.set(null);
    try {
      await api.stopInstance(instance.id);
      onChanged();
    } catch (e) {
      lastError.set(`stop ${instance.id}: ${describeError(e)}`);
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
</script>

<div class="rounded-lg border border-bg-600 bg-bg-700/40 p-4 shadow-sm transition hover:border-bg-600/80">
  <div class="flex items-center gap-3">
    <button
      type="button"
      on:click={toggleExpand}
      class="flex-1 text-left"
      aria-expanded={expanded}
    >
      <div class="flex items-baseline gap-2">
        <span class="text-lg font-medium text-fg">{workspace.name}</span>
        <span class="text-xs uppercase tracking-wide text-fg-mute">{workspace.source}</span>
        {#if hasRunning}
          <span class="ml-1 inline-block h-2 w-2 rounded-full bg-accent" title="running"></span>
        {/if}
      </div>
      <div class="text-sm text-fg-dim">{workspace.apps} app{workspace.apps === 1 ? '' : 's'}</div>
    </button>

    {#if hasRunning}
      <button
        type="button"
        on:click={stop}
        disabled={busy || $loading}
        class="rounded bg-accent-err/20 px-3 py-1.5 text-sm font-medium text-accent-err hover:bg-accent-err/30 disabled:opacity-50"
      >
        {busy ? '...' : 'Stop'}
      </button>
    {:else}
      <button
        type="button"
        on:click={start}
        disabled={busy || $loading}
        class="rounded bg-accent/20 px-3 py-1.5 text-sm font-medium text-accent hover:bg-accent/30 disabled:opacity-50"
      >
        {busy ? '...' : 'Start'}
      </button>
    {/if}
  </div>

  {#if expanded && detail}
    <div class="mt-3 border-t border-bg-600 pt-3">
      <div class="text-xs text-fg-mute break-all mb-2 font-mono">{workspace.path}</div>
      <ul class="space-y-1 text-sm">
        {#each detail.app_details as app}
          <li class="flex items-baseline gap-2 font-mono">
            <span class="w-20 text-fg-dim">{app.type}</span>
            <span class="flex-1 break-all">{app.url || app.cmd}{app.args ? ' ' + app.args : ''}</span>
          </li>
        {/each}
      </ul>
    </div>
  {/if}
</div>
