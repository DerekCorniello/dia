<script lang="ts">
  import { api, describeError } from '../api';
  import { lastError } from '../stores';
  import type { CheckInfo, PathsInfo } from '../api';

  export let doctor: CheckInfo[];
  export let paths: PathsInfo | null;
  export let plugins: string[];
  export let onRefresh: () => void = () => {};
  export let onClose: () => void = () => {};

  let busy = false;

  async function openConfig() {
    busy = true;
    lastError.set(null);
    try {
      await api.openConfigFolder();
    } catch (e) {
      lastError.set(`open config: ${describeError(e)}`);
    } finally {
      busy = false;
    }
  }

  async function openState() {
    busy = true;
    lastError.set(null);
    try {
      await api.openStateFolder();
    } catch (e) {
      lastError.set(`open state: ${describeError(e)}`);
    } finally {
      busy = false;
    }
  }

  async function reconcile() {
    busy = true;
    lastError.set(null);
    try {
      await api.reconcile();
      onRefresh();
    } catch (e) {
      lastError.set(`reconcile: ${describeError(e)}`);
    } finally {
      busy = false;
    }
  }
</script>

<div class="rounded-lg border border-bg-600 bg-bg-700/40 p-4">
  <div class="flex items-center justify-between mb-3">
    <h2 class="text-sm font-semibold uppercase tracking-wide text-fg-dim">Settings</h2>
    <button
      type="button"
      on:click={onClose}
      class="text-fg-mute hover:text-fg"
      aria-label="close settings"
    >
      x
    </button>
  </div>

  <section class="mb-4">
    <h3 class="text-xs font-semibold uppercase tracking-wide text-fg-mute mb-1">Paths</h3>
    {#if paths}
      <dl class="text-xs font-mono space-y-1">
        <div class="flex gap-2">
          <dt class="w-32 text-fg-mute shrink-0">global config</dt>
          <dd class="break-all text-fg-dim">{paths.global_config_dir}</dd>
        </div>
        <div class="flex gap-2">
          <dt class="w-32 text-fg-mute shrink-0">state dir</dt>
          <dd class="break-all text-fg-dim">{paths.state_dir}</dd>
        </div>
        <div class="flex gap-2">
          <dt class="w-32 text-fg-mute shrink-0">state file</dt>
          <dd class="break-all text-fg-dim">{paths.state_file}</dd>
        </div>
      </dl>
    {:else}
      <p class="text-xs text-fg-mute">loading...</p>
    {/if}
  </section>

  <section class="mb-4">
    <h3 class="text-xs font-semibold uppercase tracking-wide text-fg-mute mb-1">Doctor</h3>
    <ul class="text-xs space-y-0.5">
      {#each doctor as c}
        <li class="flex gap-2">
          <span
            class="w-10 shrink-0 text-center rounded px-1 text-[10px] font-semibold uppercase {c.status === 'ok' ? 'bg-accent/20 text-accent' : c.status === 'warn' ? 'bg-accent-warn/20 text-accent-warn' : 'bg-accent-err/20 text-accent-err'}"
          >
            {c.status}
          </span>
          <span class="w-24 shrink-0 text-fg">{c.name}</span>
          <span class="flex-1 break-all font-mono text-fg-dim">{c.detail || ''}</span>
        </li>
      {/each}
    </ul>
  </section>

  <section class="mb-4">
    <h3 class="text-xs font-semibold uppercase tracking-wide text-fg-mute mb-1">
      Plugins ({plugins.length})
    </h3>
    {#if plugins.length === 0}
      <p class="text-xs text-fg-mute">no dia-* on PATH</p>
    {:else}
      <ul class="text-xs font-mono space-y-0.5">
        {#each plugins as p}
          <li class="break-all text-fg-dim">{p}</li>
        {/each}
      </ul>
    {/if}
  </section>

  <div class="flex flex-wrap gap-2">
    <button
      type="button"
      on:click={openConfig}
      disabled={busy}
      class="rounded bg-bg-600 px-3 py-1.5 text-xs hover:bg-bg-600/70 disabled:opacity-50"
    >
      Open config folder
    </button>
    <button
      type="button"
      on:click={openState}
      disabled={busy}
      class="rounded bg-bg-600 px-3 py-1.5 text-xs hover:bg-bg-600/70 disabled:opacity-50"
    >
      Open state folder
    </button>
    <button
      type="button"
      on:click={reconcile}
      disabled={busy}
      class="rounded bg-bg-600 px-3 py-1.5 text-xs hover:bg-bg-600/70 disabled:opacity-50"
    >
      Reconcile
    </button>
  </div>
</div>
