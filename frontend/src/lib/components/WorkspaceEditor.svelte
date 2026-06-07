<script lang="ts">
  import { api, describeError, type WorkspaceEditor, type AppEditor, type PluginRefEditor, type PluginInfo, type ToolCategory, type DetectedTool } from '../api';
  import ConfirmDialog from './ConfirmDialog.svelte';

  export let name: string;
  export let plugins: PluginInfo[] = [];
  export let onclose: () => void = () => {};
  export let onsaved: () => void = () => {};
  export let ondeleted: () => void = () => {};

  let editor: WorkspaceEditor = { name, originalName: name, description: '', defaultCwd: '', apps: [], plugins: [] };
  let busy = false;
  let error: string | null = null;
  let toolCats: ToolCategory[] = [];
  let openCat: string | null = null;
  let openConfigPlugin: string | null = null;
  let showDeleteConfirm = false;

  async function load() {
    try {
      const ed = await api.getWorkspaceEditor(name);
      // Add unique _key to each plugin ref for duplicate management.
      ed.plugins = ed.plugins.map((r, i) => ({ ...r, _key: `plugin-${i}` }));
      editor = ed;
    } catch (e) {
      error = describeError(e);
    }
    try {
      toolCats = await api.detectTools();
    } catch {
      // non-fatal
    }
  }

  function addApp() {
    editor = { ...editor, apps: [...editor.apps, { label: '', cmd: '', cwd: editor.defaultCwd || '', url: '', termCmd: '', _showUrl: false }] };
  }

  function addTool(tool: DetectedTool, cat: string) {
    editor = { ...editor, apps: [...editor.apps, {
      label: tool.label,
      cmd: tool.command,
      cwd: editor.defaultCwd || '',
      url: tool.url || '',
      termCmd: '',
      _cat: cat,
      _showUrl: cat === 'Browsers',
    }] };
    openCat = null;
  }

  function removeApp(i: number) {
    editor = { ...editor, apps: editor.apps.filter((_, idx) => idx !== i) };
  }

  let pluginKeyCounter = 0;

  function addPlugin(p: PluginInfo) {
    pluginKeyCounter++;
    const config: Record<string, any> = {};
    const ps = (p as any).configSchema;
    if (ps && typeof ps === 'object') {
      for (const [key, field] of Object.entries(ps)) {
        if (field && typeof field === 'object' && 'default' in field) {
          config[key] = field.default;
        }
      }
    }
    editor = { ...editor, plugins: [...editor.plugins, { _key: `plugin-${pluginKeyCounter}`, id: p.id, config }] };
    openConfigPlugin = null;
  }

  function removePlugin(key: string) {
    editor = { ...editor, plugins: editor.plugins.filter((r) => r._key !== key) };
  }

  function pluginConfig(ref: PluginRefEditor): Record<string, any> {
    return ref?.config || {};
  }

  function setPluginConfig(id: string, key: string, value: any) {
    editor = {
      ...editor,
      plugins: editor.plugins.map((r) =>
        r.id === id ? { ...r, config: { ...r.config, [key]: value } } : r
      ),
    };
  }

  function configFieldDefault(schema: any): any {
    if (schema && typeof schema === 'object' && 'default' in schema) return schema.default;
    return '';
  }

  function configFieldType(schema: any): string {
    if (!schema || typeof schema !== 'object') return 'text';
    if (schema.type === 'number' || schema.type === 'integer') return 'number';
    if (schema.type === 'boolean') return 'checkbox';
    if (schema.type === 'select' && schema.options) return 'select';
    return 'text';
  }

  function pluginConfigSchema(p: PluginInfo): Record<string, any> {
    return (p as any).configSchema || {};
  }

  function schemaEntries(s: any): Array<[string, any]> {
    if (!s || typeof s !== 'object') return [];
    return Object.entries(s);
  }

  async function save() {
    busy = true;
    error = null;
    try {
      await api.saveWorkspaceEditor(editor);
      onsaved();
    } catch (e) {
      error = describeError(e);
    } finally {
      busy = false;
    }
  }

  async function del() {
    showDeleteConfirm = true;
  }

  async function confirmDelete() {
    showDeleteConfirm = false;
    busy = true;
    error = null;
    try {
      await api.deleteWorkspace(editor.name);
      ondeleted();
    } catch (e) {
      error = describeError(e);
    } finally {
      busy = false;
    }
  }

  function handleKey(e: KeyboardEvent) {
    if (e.key === 'Escape') onclose();
  }

  load();
</script>

<svelte:window on:keydown={handleKey} />

<div class="fixed inset-0 z-50 flex items-center justify-center bg-bg-900/80 p-4">
  <div class="flex max-h-[calc(100vh-2rem)] w-[min(56rem,calc(100vw-2rem))] flex-col overflow-hidden rounded-lg border border-primary/15 bg-bg-700 shadow-lg">
    <div class="flex items-center justify-between border-b border-primary/15 px-4 py-3">
      <h2 class="text-sm font-semibold uppercase tracking-wide text-fg-dim">Edit workspace</h2>
      <button type="button" on:click={onclose} class="rounded p-1 text-fg-mute hover:bg-bg-600 hover:text-fg" aria-label="close editor">
        <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg>
      </button>
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto p-4 space-y-4">
      {#if error}
        <p class="text-xs text-accent-err">{error}</p>
      {/if}

      <label class="block text-sm">
        <span class="text-fg-dim">Name</span>
        <input type="text" bind:value={editor.name} disabled={busy} placeholder="my-project"
          class="mt-1 block w-full rounded border border-bg-600 bg-bg-800 px-2 py-1.5 text-sm font-mono focus:border-accent focus:outline-none" />
        <span class="block text-xs text-fg-mute mt-1">lowercase a-z, 0-9, internal hyphens</span>
      </label>

      <label class="block text-sm">
        <span class="text-fg-dim">Description</span>
        <textarea bind:value={editor.description} disabled={busy} rows="2" placeholder="What this workspace is for"
          class="mt-1 block w-full rounded border border-bg-600 bg-bg-800 px-2 py-1.5 text-sm focus:border-accent focus:outline-none"></textarea>
      </label>

      {#if toolCats.length > 0}
        <div>
          <span class="text-xs font-semibold uppercase tracking-wide text-fg-mute mb-2 block">Quick add</span>
          <div class="flex flex-wrap gap-1.5">
            {#each toolCats as cat (cat.name)}
              <div class="relative">
                <button type="button" on:click={() => openCat = openCat === cat.name ? null : cat.name}
                  class="rounded bg-info/15 px-2 py-1 text-[10px] text-info hover:bg-info/25">
                  {cat.name} ({cat.tools.length})
                </button>
                {#if openCat === cat.name}
                  <div class="absolute left-0 top-full z-20 mt-1 w-52 rounded border border-bg-600/80 bg-bg-800 shadow-2xl">
                    {#each cat.tools as tool}
                      <button type="button" on:click={() => addTool(tool, cat.name)}
                        class="block w-full text-left px-3 py-1.5 text-xs text-fg-dim hover:bg-accent/15 hover:text-fg">
                        {tool.label}
                      </button>
                    {/each}
                  </div>
                {/if}
              </div>
            {/each}
          </div>
        </div>
      {/if}

      <div>
        <div class="flex items-center justify-between mb-2">
          <span class="text-xs font-semibold uppercase tracking-wide text-fg-mute">Apps ({editor.apps.length})</span>
        </div>
        <button type="button" on:click={addApp} class="mb-2 rounded bg-bg-600 px-2 py-0.5 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg">+ Add app</button>
        {#if editor.apps.length === 0}
          <p class="text-xs text-fg-mute">No apps configured. Use Quick add or + Add app.</p>
        {:else}
          <div class="space-y-2">
            {#each editor.apps as app, i (i)}
              <div class="rounded border border-bg-600 bg-bg-800 p-3">
                <div class="flex items-center justify-between mb-2">
                  <span class="text-xs text-fg-mute font-mono">{app.label || `app [${i}]`}</span>
                  <button type="button" on:click={() => removeApp(i)} class="text-accent-err hover:underline text-[10px]">remove</button>
                </div>
                <div class="grid grid-cols-2 gap-2">
                  <label class="block text-xs">
                    <span class="text-fg-mute">label</span>
                    <input type="text" bind:value={app.label} disabled={busy} placeholder="e.g. API Server"
                      class="mt-0.5 block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-xs font-mono focus:border-accent focus:outline-none" />
                  </label>
                  <label class="block text-xs">
                    <span class="text-fg-mute">command</span>
                    <input type="text" bind:value={app.cmd} disabled={busy} placeholder="executable and args"
                      class="mt-0.5 block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-xs font-mono focus:border-accent focus:outline-none" />
                  </label>
                  <label class="block text-xs">
                    <span class="text-fg-mute">cwd</span>
                    <input type="text" bind:value={app.cwd} disabled={busy} placeholder="working directory"
                      class="mt-0.5 block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-xs font-mono focus:border-accent focus:outline-none" />
                  </label>
                  {#if app._cat === 'Terminals'}
                    <label class="block text-xs">
                      <span class="text-fg-mute">run in terminal</span>
                      <input type="text" bind:value={app.termCmd} disabled={busy} placeholder="e.g. btop or bash -c 'make; exec bash'"
                        class="mt-0.5 block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-xs font-mono focus:border-accent focus:outline-none" />
                    </label>
                  {/if}
                  {#if app._showUrl || app.url}
                    <label class="block text-xs">
                      <span class="text-fg-mute">url</span>
                      <input type="text" bind:value={app.url} disabled={busy} placeholder="https://..."
                        class="mt-0.5 block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-xs font-mono focus:border-accent focus:outline-none" />
                    </label>
                  {/if}
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>

      <div>
        <div class="flex items-center justify-between mb-2">
          <span class="text-xs font-semibold uppercase tracking-wide text-fg-mute">Plugins ({editor.plugins.length})</span>
        </div>
        {#if plugins.length > 0}
          <div class="relative mb-2">
            <button type="button" on:click={() => openConfigPlugin = openConfigPlugin === '__picker__' ? null : '__picker__'}
              class="rounded bg-primary/15 px-2 py-0.5 text-[10px] text-primary hover:bg-primary/25">+ Add plugin</button>
            {#if openConfigPlugin === '__picker__'}
              <div class="absolute left-0 top-full z-20 mt-1 w-52 rounded border border-bg-600/80 bg-bg-800 shadow-2xl max-h-60 overflow-y-auto">
                {#each plugins as p (p.id)}
                  <button type="button" on:click={() => addPlugin(p)}
                    class="block w-full text-left px-3 py-1.5 text-xs text-fg-dim hover:bg-accent/15 hover:text-fg">
                    {p.name || p.id} <span class="text-fg-mute">{p.version}</span>
                  </button>
                {/each}
              </div>
            {/if}
          </div>
        {/if}
        {#if editor.plugins.length === 0}
          <p class="text-xs text-fg-mute">No plugins added. Use + Add plugin.</p>
        {:else}
          <div class="space-y-1.5">
            {#each editor.plugins as ref (ref._key)}
              {@const p = plugins.find((pl) => pl.id === ref.id)}
              <div class="rounded border border-bg-600 bg-bg-800 {p && pluginConfigSchema(p) && Object.keys(pluginConfigSchema(p)).length > 0 ? 'border-primary/20' : ''}">
                <div class="flex items-center gap-2 px-3 py-2">
                  <span class="text-xs text-fg-dim flex-1">{p?.name || ref.id}</span>
                  <span class="text-[10px] text-fg-mute">{p?.version || ''}</span>
                  <button type="button" on:click={() => removePlugin(ref._key ?? '')} class="text-accent-err hover:underline text-[10px]">remove</button>
                  {#if p && pluginConfigSchema(p) && Object.keys(pluginConfigSchema(p)).length > 0}
                    <button type="button" on:click={() => openConfigPlugin = openConfigPlugin === (ref._key ?? '') ? null : (ref._key ?? '')}
                      class="text-[10px] text-primary hover:underline">
                      {openConfigPlugin === ref._key ? 'hide config' : 'config'}
                    </button>
                  {/if}
                </div>
                {#if p && openConfigPlugin === ref._key && pluginConfigSchema(p) && Object.keys(pluginConfigSchema(p)).length > 0}
                  <div class="border-t border-bg-600 px-3 py-2 space-y-2">
                    {#each schemaEntries(pluginConfigSchema(p)) as [key, field]}
                      <label class="block text-xs">
                        <span class="text-fg-mute">{field?.label || key}</span>
                        {#if configFieldType(field) === 'checkbox'}
                          <input type="checkbox" checked={pluginConfig(ref)[key] === true}
                            on:change={(e) => setPluginConfig(ref.id, key, e.currentTarget.checked)}
                            class="block accent-primary" />
                        {:else if configFieldType(field) === 'number'}
                          <input type="number" value={pluginConfig(ref)[key] ?? configFieldDefault(field)}
                            on:input={(e) => setPluginConfig(ref.id, key, parseFloat(e.currentTarget.value) || 0)}
                            class="mt-0.5 block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-xs font-mono focus:border-accent focus:outline-none" />
                        {:else if configFieldType(field) === 'select' && field.options}
                          <select
                            value={pluginConfig(ref)[key] ?? configFieldDefault(field)}
                            on:change={(e) => setPluginConfig(ref.id, key, e.currentTarget.value)}
                            class="mt-0.5 block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-xs font-mono focus:border-accent focus:outline-none">
                            {#each field.options as opt}
                              <option value={opt}>{opt}</option>
                            {/each}
                          </select>
                        {:else}
                          <input type="text" value={pluginConfig(ref)[key] ?? configFieldDefault(field)}
                            on:input={(e) => setPluginConfig(ref.id, key, e.currentTarget.value)}
                            class="mt-0.5 block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-xs font-mono focus:border-accent focus:outline-none" />
                        {/if}
                      </label>
                    {/each}
                  </div>
                {/if}
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>

    <div class="flex items-center justify-between border-t border-primary/15 px-4 py-3">
      <button type="button" on:click={del} disabled={busy} class="rounded bg-error/20 px-3 py-1.5 text-xs font-medium text-error hover:bg-error/30 disabled:opacity-50">Delete</button>
      <div class="flex items-center gap-2">
        <button type="button" on:click={onclose} disabled={busy} class="rounded px-3 py-1.5 text-xs text-fg-dim hover:text-fg disabled:opacity-50">Cancel</button>
        <button type="button" on:click={save} disabled={busy || !editor.name.trim()} class="rounded bg-primary px-3 py-1.5 text-xs font-medium text-primary-content hover:bg-primary/90 disabled:opacity-50">{busy ? '...' : 'Save'}</button>
      </div>
    </div>
  </div>
</div>

{#if showDeleteConfirm}
  <ConfirmDialog
    title="Delete workspace"
    message="Delete workspace &quot;{editor.name}&quot;? This cannot be undone."
    confirmLabel="Delete"
    on:confirm={confirmDelete}
    on:cancel={() => (showDeleteConfirm = false)}
  />
{/if}
