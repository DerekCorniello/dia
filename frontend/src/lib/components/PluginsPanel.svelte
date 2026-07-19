<script lang="ts" context="module">
  import type { PluginInfo, CapabilityInfo } from '../api';
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

  // Capability state per plugin, loaded on demand. Which capabilities
  // exist and which are mutating is decided by the host, not
  // duplicated here, so the two can never disagree.
  let caps: Record<string, CapabilityInfo[]> = {};
  let pending: Record<string, string[]> = {};
  let saving: string | null = null;

  async function installFromFolder() {
    error = null;
    try {
      await api.installPluginFromFolder();
      dispatch('refresh');
    } catch (e) {
      error = describeError(e);
    }
  }

  async function loadCaps(id: string) {
    if (caps[id]) return;
    try {
      const info = await api.getPluginCapabilities(id);
      caps = { ...caps, [id]: info.requested ?? [] };
      pending = { ...pending, [id]: info.granted ?? [] };
    } catch (e) {
      error = describeError(e);
    }
  }

  function toggle(id: string, cap: string) {
    const current = pending[id] ?? [];
    pending = {
      ...pending,
      [id]: current.includes(cap) ? current.filter((c) => c !== cap) : [...current, cap],
    };
  }

  // Which plugins have unsaved capability edits.
  //
  // This has to be a reactive statement naming `pending` and `caps`,
  // not a function called from the markup. A template expression only
  // re-evaluates when an identifier it *names* changes, so
  // `disabled={!isDirty(p.id)}` would name `p` and never `pending` --
  // toggling a checkbox would leave Save disabled forever.
  $: dirtyIds = computeDirty(pending, caps);

  function computeDirty(
    edits: Record<string, string[]>,
    loaded: Record<string, CapabilityInfo[]>,
  ): Record<string, boolean> {
    const out: Record<string, boolean> = {};
    for (const [id, next] of Object.entries(edits)) {
      const list = loaded[id];
      if (!list) continue;
      const prev = list.filter((c) => c.granted).map((c) => c.name);
      out[id] = next.length !== prev.length || next.some((c) => !prev.includes(c));
    }
    return out;
  }

  async function save(id: string) {
    error = null;
    saving = id;
    try {
      await api.setPluginCapabilities(id, pending[id] ?? []);
      // Drop the cache so the next open reflects what the host applied
      // (it may drop capabilities the manifest never requested).
      const { [id]: _c, ...restCaps } = caps;
      caps = restCaps;
      const { [id]: _p, ...restPending } = pending;
      pending = restPending;
      dispatch('refresh');
    } catch (e) {
      error = describeError(e);
    } finally {
      saving = null;
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

          <details class="mt-2" on:toggle={() => loadCaps(p.id)}>
            <summary class="cursor-pointer text-[10px] text-primary hover:text-primary/80">
              capabilities
            </summary>
            {#if caps[p.id]}
              {#if caps[p.id].length === 0}
                <p class="mt-1 text-xs text-fg-mute">this plugin requests no capabilities</p>
              {:else}
                <ul class="mt-1 space-y-1">
                  {#each caps[p.id] as c (c.name)}
                    <li class="flex items-center gap-2">
                      <input
                        type="checkbox"
                        id="{p.id}-{c.name}"
                        checked={(pending[p.id] ?? []).includes(c.name)}
                        on:change={() => toggle(p.id, c.name)}
                        class="h-3 w-3 accent-primary"
                      />
                      <label for="{p.id}-{c.name}" class="font-mono text-[11px] text-fg-dim">
                        {c.name}
                      </label>
                      {#if c.mutating}
                        <span
                          class="inline-flex rounded-full bg-error/15 px-1.5 py-0.5 text-[9px] text-error"
                          title="Lets this plugin change your system"
                        >
                          mutating
                        </span>
                      {/if}
                    </li>
                  {/each}
                </ul>
                <button
                  type="button"
                  disabled={!dirtyIds[p.id] || saving === p.id}
                  on:click={() => save(p.id)}
                  class="mt-2 rounded bg-bg-600 px-2 py-0.5 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg disabled:opacity-40 disabled:hover:bg-bg-600"
                >
                  {saving === p.id ? 'saving...' : 'Save capabilities'}
                </button>
              {/if}
            {:else}
              <p class="mt-1 text-xs text-fg-mute">loading...</p>
            {/if}
          </details>
        </li>
      {/each}
    </ul>
  {/if}
</section>
