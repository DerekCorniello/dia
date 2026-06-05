<script lang="ts">
  import { api, describeError } from '../api';
  import { instances, lastError } from '../stores';
  import type { InstanceInfo } from '../api';

  export let inst: InstanceInfo;
  export let onChanged: () => void = () => {};

  let busy = false;

  async function stop() {
    busy = true;
    lastError.set(null);
    try {
      await api.stopInstance(inst.id);
      instances.update((arr) => arr.filter((i) => i.id !== inst.id));
      onChanged();
    } catch (e) {
      lastError.set(`stop ${inst.id}: ${describeError(e)}`);
    } finally {
      busy = false;
    }
  }

  function fmtTime(iso: string): string {
    if (!iso) return '';
    try {
      return new Date(iso).toLocaleTimeString();
    } catch {
      return iso;
    }
  }
</script>

<div class="rounded border border-bg-600 bg-bg-700/30 px-3 py-2 text-sm">
  <div class="flex items-center gap-2">
    <span class="inline-block h-2 w-2 rounded-full {inst.status === 'running' ? 'bg-accent' : 'bg-fg-mute'}"></span>
    <span class="font-medium">{inst.workspace}</span>
    <span class="text-fg-mute text-xs">{inst.status}</span>
    <span class="text-fg-mute text-xs ml-auto">{fmtTime(inst.started_at)}</span>
    {#if inst.status === 'running'}
      <button
        type="button"
        on:click={stop}
        disabled={busy}
        class="rounded bg-accent-err/20 px-2 py-0.5 text-xs text-accent-err hover:bg-accent-err/30 disabled:opacity-50"
      >
        {busy ? '...' : 'Stop'}
      </button>
    {/if}
  </div>
  {#if inst.apps && inst.apps.length > 0}
    <ul class="mt-1 ml-4 space-y-0.5 text-xs font-mono text-fg-dim">
      {#each inst.apps as a}
        <li>
          <span class="text-fg-mute">{a.type}</span>
          <span class="ml-2">{a.pid > 0 ? `pid=${a.pid}` : a.cmd}</span>
          <span class="ml-2 text-fg-mute">{a.status}</span>
          {#if a.err}
            <span class="ml-2 text-accent-err" title={a.err}>!</span>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}
</div>
