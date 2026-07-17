<script lang="ts" context="module">
  import type { PluginInfo } from '../api';
</script>

<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { pushToast } from '../stores';
  import { api, describeError } from '../api';

  export let plugins: PluginInfo[];

  const dispatch = createEventDispatcher<{
    refresh: void;
  }>();

  let error: string | null = null;

  async function installFromFolder() {
    error = null;
    try {
      await api.installPluginFromFolder();
      dispatch('refresh');
    } catch (e) {
      error = describeError(e);
    }
  }

  function statusClass(s: string): string {
    if (s === 'active' || s === 'loaded') return 'bg-success/20 text-success';
    return 'bg-error/20 text-error';
  }

  function copy(text: string) {
    navigator.clipboard?.writeText(text).then(
      () => {},
      () => pushToast('err', 'copy failed'),
    );
  }
</script>

<section class="space-y-4">
  <div class="flex items-center gap-2">
    <h3 class="text-xs font-semibold uppercase tracking-wide text-fg-mute">Plugins</h3>
    <div class="ml-auto flex gap-1.5">
      <button
        type="button"
        on:click={installFromFolder}
        class="rounded bg-bg-600 px-2 py-0.5 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg"
        >Install from folder</button
      >
      <button
        type="button"
        on:click={() => dispatch('refresh')}
        class="rounded bg-bg-600 px-2 py-0.5 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg"
        >Refresh</button
      >
    </div>
  </div>

  {#if error}
    <p class="text-xs text-accent-err">{error}</p>
  {/if}

  {#if plugins.length === 0}
    <p class="text-xs text-fg-mute">no plugins installed</p>
  {:else}
    <ul class="space-y-2">
      {#each plugins as p (p.id + p.source)}
        <li class="rounded border border-bg-600 bg-bg-800 p-3 text-sm">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2 min-w-0">
              <span class="font-semibold text-fg truncate">{p.name || p.id}</span>
              <span
                class="inline-flex shrink-0 rounded-full bg-secondary/15 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-secondary"
              >
                {p.source}
              </span>
              <span class="shrink-0 text-[10px] text-fg-mute">v{p.version}</span>
            </div>
          </div>
          {#if p.description}
            <p class="mt-1 text-xs text-fg-dim">{p.description}</p>
          {/if}
          {#if p.longDescription}
            <details class="mt-1">
              <summary class="cursor-pointer text-[10px] text-primary hover:text-primary/80"
                >more details</summary
              >
              <p class="mt-1 whitespace-pre-wrap text-xs text-fg-dim">{p.longDescription}</p>
            </details>
          {/if}
          <div class="mt-1 flex flex-wrap items-center gap-2">
            <span
              class="inline-flex rounded-full px-1.5 py-0.5 text-[9px] font-medium {statusClass(
                p.status,
              )}">{p.status}</span
            >
            {#if p.lastError}
              <span
                class="inline-flex shrink-0 items-center gap-1 rounded-full bg-error/10 px-1.5 py-0.5 text-[9px] text-error"
                title={p.lastError}
              >
                error
              </span>
            {/if}
            <button
              type="button"
              on:click={() => copy(p.dir)}
              class="text-[10px] text-fg-mute hover:text-fg"
              title="Copy directory"
            >
              {p.dir}
            </button>
          </div>
        </li>
      {/each}
    </ul>
  {/if}
</section>
