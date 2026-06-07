<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { api, describeError, type PluginInfo } from '../api';
  import { lastError } from '../stores';

  export let plugins: PluginInfo[];

  const dispatch = createEventDispatcher<{
    refresh: void;
  }>();

  let busy = false;
  let error: string | null = null;

  async function toggle(p: PluginInfo) {
    busy = true;
    error = null;
    try {
      if (p.enabled) {
        await api.disablePlugin(p.id);
      } else {
        await api.setPluginEnabled(p.id, true, p.grantedCapabilities ?? []);
      }
      dispatch('refresh');
    } catch (e) {
      error = describeError(e);
    } finally {
      busy = false;
    }
  }

  function statusClass(s: string): string {
    if (s === 'active' || s === 'loaded') return 'bg-success/20 text-success';
    return 'bg-error/20 text-error';
  }

  function copy(text: string) {
    navigator.clipboard?.writeText(text).then(
      () => {},
      () => lastError.set('copy failed'),
    );
  }
</script>

<section class="space-y-4">
  <h3 class="text-xs font-semibold uppercase tracking-wide text-fg-mute">Plugins</h3>

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
              <span class="inline-flex shrink-0 rounded-full bg-secondary/15 px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide text-secondary">
                {p.source}
              </span>
              <span class="shrink-0 text-[10px] text-fg-mute">v{p.version}</span>
            </div>
            <button
              type="button"
              on:click={() => toggle(p)}
              disabled={busy || p.status === 'errored'}
              class="rounded px-2 py-0.5 text-[10px] disabled:opacity-50 {p.enabled
                ? 'bg-primary/20 text-primary hover:bg-primary/30'
                : 'bg-bg-600 text-fg-dim hover:bg-bg-600/70 hover:text-fg'}"
            >
              {p.enabled ? 'enabled' : 'disabled'}
            </button>
          </div>
          {#if p.description}
            <p class="mt-1 text-xs text-fg-dim">{p.description}</p>
          {/if}
          <div class="mt-1 flex flex-wrap items-center gap-2 text-[10px] text-fg-mute">
            <span class="rounded bg-bg-600 px-1.5 py-0.5 {statusClass(p.status)}">
              {p.status}
            </span>
            {#if p.grantedCapabilities && p.grantedCapabilities.length > 0}
              <span>caps: {p.grantedCapabilities.join(', ')}</span>
            {/if}
          </div>
          {#if p.lastError}
            <p class="mt-1 break-all font-mono text-[10px] text-accent-err">
              {p.lastError}
            </p>
          {/if}
          <p class="mt-1 break-all font-mono text-[10px] text-fg-mute">{p.dir}</p>
        </li>
      {/each}
    </ul>
  {/if}

  <details class="rounded border border-info/20 bg-info/5 p-3 text-xs text-fg-dim">
    <summary class="cursor-pointer text-sm text-fg">Writing a plugin</summary>
    <div class="mt-2 space-y-2 text-xs leading-relaxed text-fg-dim">
      <p>
        A plugin is a folder with a         <code class="rounded bg-base-900 px-1 text-primary/80">plugin.json</code>
        manifest and an <code class="rounded bg-base-900 px-1 text-primary/80">index.js</code> entry. The
        host runs the entry in a sandboxed JavaScript interpreter; the plugin calls back
        into dia through a capability-gated
        <code class="rounded bg-base-900 px-1 text-primary/80">dia.*</code> API.
      </p>
      <p>
        Scaffold one with
        <code class="rounded bg-base-900 px-1 text-primary/80">dia plugin new my-plugin</code> or drop a
        folder into the path above. The host picks it up on next launch.
      </p>
      <p class="font-semibold text-fg">Manifest</p>
      <pre class="overflow-x-auto rounded bg-base-900 p-2 text-[11px] text-fg-dim">{`{
  "id": "my-plugin",
  "name": "My Plugin",
  "version": "0.1.0",
  "description": "What it does",
  "author": "you",
  "entry": "index.js",
  "capabilities": ["workspaces:read"],
  "ui": {
    "type": "list",
    "title": "Things",
    "refreshable": true,
    "actions": [
      { "id": "open", "label": "Open", "capability": "workspaces:start" }
    ]
  }
}`}</pre>
      <p class="font-semibold text-fg">Entry</p>
      <pre class="overflow-x-auto rounded bg-base-900 p-2 text-[11px] text-fg-dim">{`module.exports = {
  getData: function () {
    // return shape depends on ui.type:
    //   list/grid: [{ id, label, detail? }, ...]
    //   table:     [{ col: value, ... }, ...]
    //   kv:        { key: value, ... }
    //   text:      "any string"
    //   canvas:    { strokes: [], color: "#000", width: 2 }
    return dia.listWorkspaces().map(function (w) {
      return { id: w.name, label: w.name };
    });
  },
  onAction: function (id, ctx) {
    // optional; ctx carries the host's view of the current state
    if (id === "open") {
      dia.startWorkspace(ctx.item.id);
    }
  }
};`}</pre>
      <p class="font-semibold text-fg">UI types</p>
      <ul class="ml-4 list-disc space-y-0.5">
        <li><code class="rounded bg-base-900 px-1 text-primary/80">list</code> - vertical list of items</li>
        <li><code class="rounded bg-base-900 px-1 text-primary/80">grid</code> - card grid</li>
        <li>
          <code class="rounded bg-base-900 px-1 text-primary/80">table</code> - requires
          <code class="rounded bg-base-900 px-1 text-primary/80">ui.columns[]</code> with key/label/format
        </li>
        <li><code class="rounded bg-base-900 px-1 text-primary/80">kv</code> - key/value pairs</li>
        <li><code class="rounded bg-base-900 px-1 text-primary/80">text</code> - monospace block</li>
        <li>
          <code class="rounded bg-base-900 px-1 text-primary/80">canvas</code> - free-draw surface. The
          host sends mouse/touch events to
          <code class="rounded bg-base-900 px-1 text-primary/80">ctx.strokes</code> when an action runs
        </li>
      </ul>
      <p class="font-semibold text-fg">Capabilities</p>
      <p>
        Read-only defaults are granted on install:
        <code class="rounded bg-base-900 px-1 text-primary/80">workspaces:read</code>,
        <code class="rounded bg-base-900 px-1 text-primary/80">instances:read</code>,
        <code class="rounded bg-base-900 px-1 text-primary/80">doctor:read</code>,
        <code class="rounded bg-base-900 px-1 text-primary/80">paths:read</code>,
        <code class="rounded bg-base-900 px-1 text-primary/80">themes:read</code>. Mutating ones
        (<code class="rounded bg-base-900 px-1 text-primary/80">workspaces:start</code>,
        <code class="rounded bg-base-900 px-1 text-primary/80">instances:stop</code>,
        <code class="rounded bg-base-900 px-1 text-primary/80">workspaces:create</code>,
        <code class="rounded bg-base-900 px-1 text-primary/80">themes:write</code>) are opt-in. Calling a
        method you don't have throws.
      </p>
      <p class="font-semibold text-fg">Tips</p>
      <ul class="ml-4 list-disc space-y-0.5">
        <li>Use the CLI: <code class="rounded bg-base-900 px-1 text-primary/80">dia plugin list</code>, <code class="rounded bg-base-900 px-1 text-primary/80">dia plugin info my-plugin</code></li>
        <li>Local plugins live in <code class="rounded bg-base-900 px-1 text-primary/80">./.dia/plugins/</code> and shadow global ones on id collision</li>
        <li>The plugin can <code class="rounded bg-base-900 px-1 text-primary/80">require('./lib/foo')</code> relative files inside its own folder</li>
      </ul>
    </div>
  </details>
</section>
