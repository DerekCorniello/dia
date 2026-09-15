<script lang="ts">
  import {
    api,
    describeError,
    type WorkspaceEditor,
    type AppEditor,
    type PluginRefEditor,
    type PluginInfo,
    type ToolCategory,
    type DetectedTool,
  } from '../api';
  import ConfirmDialog from './ConfirmDialog.svelte';
  import { focusTrap } from '../focusTrap';

  export let name: string;
  export let plugins: PluginInfo[] = [];
  export let onclose: () => void = () => {};
  export let onsaved: () => void = () => {};
  export let ondeleted: () => void = () => {};

  let editor: WorkspaceEditor = {
    name,
    originalName: name,
    originalPath: '',
    description: '',
    defaultCwd: '',
    apps: [],
    plugins: [],
    hooks: undefined,
  };
  let busy = false;
  let error: string | null = null;
  let toolCats: ToolCategory[] = [];
  let openCat: string | null = null;
  let catSearch = '';
  let pluginSearch = '';
  let openConfigPlugin: string | null = null;
  let showDeleteConfirm = false;

  type SchemaField = {
    type?: string;
    options?: unknown[];
    default?: unknown;
    label?: string;
  };

  function asSchemaField(value: unknown): SchemaField {
    return value && typeof value === 'object' ? (value as SchemaField) : {};
  }

  let appTypeDescriptors: import('../api').AppTypeDescriptor[] = [];
  const fallbackAppTypes = [
    'terminal',
    'editor',
    'service',
    'browser',
    'open',
    'local',
    'custom',
    'ai',
    'gh',
    'gh:pr',
    'gh:issue',
    'gh:checkout',
    'gh:repo-clone',
  ];
  $: builtinAppTypes =
    appTypeDescriptors.length > 0 ? appTypeDescriptors.map((d) => d.type) : fallbackAppTypes;

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
    try {
      appTypeDescriptors = await api.listAppTypes();
    } catch {
      // non-fatal: fallback list remains
    }
  }

  function addApp() {
    editor = {
      ...editor,
      apps: [
        ...editor.apps,
        {
          label: '',
          type: 'terminal',
          cmd: '',
          cwd: editor.defaultCwd || '',
          url: '',
          browser: '',
          urls: [],
          newWindow: false,
          env: {},
          args: [],
          termCmd: '',
          _showUrl: false,
        },
      ],
    };
  }

  function addTool(tool: DetectedTool, cat: string) {
    const isBrowser = cat === 'Browsers';
    editor = {
      ...editor,
      apps: [
        ...editor.apps,
        {
          label: tool.label,
          type: isBrowser ? 'browser' : tool.url && !tool.command ? 'open' : 'terminal',
          cmd: tool.command,
          cwd: editor.defaultCwd || '',
          url: tool.url || '',
          browser: isBrowser ? tool.command : '',
          urls: isBrowser && tool.url ? [tool.url] : [],
          newWindow: false,
          env: {},
          args: [],
          termCmd: '',
          _cat: cat,
          _showUrl: isBrowser,
        },
      ],
    };
    openCat = null;
  }

  function removeApp(i: number) {
    editor = { ...editor, apps: editor.apps.filter((_, idx) => idx !== i) };
  }

  function parseArgs(value: string): string[] {
    return value === '' ? [] : value.split('\n');
  }

  let pluginKeyCounter = 0;

  function addPlugin(p: PluginInfo) {
    pluginKeyCounter++;
    const config: Record<string, unknown> = {};
    const ps = p.configSchema;
    if (ps && typeof ps === 'object') {
      for (const [key, field] of Object.entries(ps)) {
        if (field && typeof field === 'object' && 'default' in field) {
          config[key] = field.default;
        }
      }
    }
    editor = {
      ...editor,
      plugins: [...editor.plugins, { _key: `plugin-${pluginKeyCounter}`, id: p.id, config }],
    };
    openConfigPlugin = null;
  }

  function removePlugin(key: string) {
    editor = { ...editor, plugins: editor.plugins.filter((r) => r._key !== key) };
  }

  function pluginConfig(ref: PluginRefEditor): Record<string, unknown> {
    return ref?.config || {};
  }

  function setPluginConfig(id: string, key: string, value: unknown) {
    editor = {
      ...editor,
      plugins: editor.plugins.map((r) =>
        r.id === id ? { ...r, config: { ...r.config, [key]: value } } : r,
      ),
    };
  }

  function configFieldDefault(schema: unknown): unknown {
    return asSchemaField(schema).default ?? '';
  }

  function configFieldType(schema: unknown): string {
    const field = asSchemaField(schema);
    if (field.type === 'number' || field.type === 'integer') return 'number';
    if (field.type === 'boolean') return 'checkbox';
    if (field.type === 'select' && field.options) return 'select';
    return 'text';
  }

  function pluginConfigSchema(p: PluginInfo): Record<string, unknown> {
    return p.configSchema || {};
  }

  function schemaEntries(s: unknown): Array<[string, SchemaField]> {
    if (!s || typeof s !== 'object') return [];
    return Object.entries(s).map(([key, value]) => [key, asSchemaField(value)]);
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
  <div
    class="flex max-h-[calc(100vh-2rem)] w-[min(56rem,calc(100vw-2rem))] flex-col overflow-hidden rounded-lg border border-primary/15 bg-bg-700 shadow-lg"
    role="dialog"
    aria-modal="true"
    aria-label="Edit workspace"
    use:focusTrap
  >
    <div class="flex items-center justify-between border-b border-primary/15 px-4 py-3">
      <h2 class="text-sm font-semibold uppercase tracking-wide text-fg-dim">Edit workspace</h2>
      <button
        type="button"
        on:click={onclose}
        class="rounded p-1 text-fg-mute hover:bg-bg-600 hover:text-fg"
        aria-label="close editor"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          ><line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" /></svg
        >
      </button>
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto p-4 space-y-4">
      {#if error}
        <p class="text-xs text-accent-err">{error}</p>
      {/if}

      <label class="block text-sm">
        <span class="text-fg-dim">Name</span>
        <input
          type="text"
          bind:value={editor.name}
          disabled={busy}
          placeholder="my-project"
          class="mt-1 block w-full rounded border border-bg-600 bg-bg-800 px-2 py-1.5 text-sm font-mono focus:border-accent focus:outline-none"
        />
        <span class="block text-xs text-fg-mute mt-1">alphanumeric with hyphens or underscores</span
        >
      </label>

      <label class="block text-sm">
        <span class="text-fg-dim">Description</span>
        <textarea
          bind:value={editor.description}
          disabled={busy}
          rows="2"
          placeholder="What this workspace is for"
          class="mt-1 block w-full rounded border border-bg-600 bg-bg-800 px-2 py-1.5 text-sm focus:border-accent focus:outline-none"
        ></textarea>
      </label>

      {#if toolCats.length > 0}
        <div>
          <span class="text-xs font-semibold uppercase tracking-wide text-fg-mute mb-2 block"
            >Quick add</span
          >
          <div class="flex flex-wrap gap-1.5">
            {#each toolCats as cat (cat.name)}
              <div class="relative">
                <button
                  type="button"
                  on:click={() => (openCat = openCat === cat.name ? null : cat.name)}
                  class="rounded bg-info/15 px-2 py-1 text-[10px] text-info hover:bg-info/25"
                >
                  {cat.name} ({cat.tools.length})
                </button>
                {#if openCat === cat.name}
                  <div
                    class="absolute left-0 top-full z-20 mt-1 w-52 rounded border border-bg-600/80 bg-bg-800 shadow-2xl"
                  >
                    <div class="p-1.5">
                      <input
                        type="text"
                        placeholder="filter..."
                        bind:value={catSearch}
                        on:click|stopPropagation
                        class="block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-[10px] text-fg-dim placeholder:text-fg-mute focus:border-accent focus:outline-none"
                      />
                    </div>
                    <div class="max-h-48 overflow-y-auto">
                      <!-- Position-keyed: a detected tool and an $EDITOR
                           style default can share a command. -->
                      {#each cat.tools.filter((t) => !catSearch || t.label
                            .toLowerCase()
                            .includes(catSearch.toLowerCase())) as tool, i (i)}
                        <button
                          type="button"
                          on:click={() => addTool(tool, cat.name)}
                          class="block w-full text-left px-3 py-1.5 text-xs text-fg-dim hover:bg-accent/15 hover:text-fg"
                        >
                          {tool.label}
                        </button>
                      {/each}
                      {#if cat.tools.filter((t) => !catSearch || t.label
                            .toLowerCase()
                            .includes(catSearch.toLowerCase())).length === 0}
                        <p class="px-3 py-2 text-[10px] text-fg-mute">No tools match</p>
                      {/if}
                    </div>
                  </div>
                {/if}
              </div>
            {/each}
          </div>
        </div>
      {/if}

      <p class="text-[11px] text-fg-mute">
        Saving rewrites the YAML file: comments and formatting are not preserved. A backup of the
        previous version is kept next to the file with a .bak extension.
      </p>

      <div>
        <div class="flex items-center justify-between mb-2">
          <span class="text-xs font-semibold uppercase tracking-wide text-fg-mute"
            >Apps ({editor.apps.length})</span
          >
        </div>
        <button
          type="button"
          on:click={addApp}
          class="mb-2 rounded bg-bg-600 px-2 py-0.5 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg"
          >+ Add app</button
        >
        {#if editor.apps.length === 0}
          <p class="text-xs text-fg-mute">No apps configured. Use Quick add or + Add app.</p>
        {:else}
          <div class="space-y-3">
            {#each editor.apps as app, i (i)}
              <div class="rounded border border-bg-600 bg-bg-800 p-3 space-y-2">
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <select
                      bind:value={app.type}
                      disabled={busy}
                      class="rounded border border-bg-600 bg-bg-700 px-1.5 py-1 text-[11px] font-medium focus:border-accent focus:outline-none"
                    >
                      <option value="">default</option>
                      {#if app.type && !builtinAppTypes.includes(app.type)}
                        <option value={app.type}>{app.type}</option>
                      {/if}
                      {#each builtinAppTypes as type (type)}
                        <option value={type}>{type}</option>
                      {/each}
                    </select>
                    <span class="text-xs text-fg-mute font-mono">{app.label || `app [${i}]`}</span>
                  </div>
                  <button
                    type="button"
                    on:click={() => removeApp(i)}
                    class="text-accent-err hover:underline text-[10px]">remove</button
                  >
                </div>
                <label class="block text-xs">
                  <span class="text-fg-mute">label - display name</span>
                  <input
                    type="text"
                    bind:value={app.label}
                    disabled={busy}
                    placeholder="e.g. mux dashboards"
                    class="mt-0.5 block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-xs font-mono focus:border-accent focus:outline-none"
                  />
                </label>

                {#if app.type === 'browser'}
                  <div class="grid grid-cols-2 gap-2">
                    <label class="block text-xs">
                      <span class="text-fg-mute">browser binary</span>
                      <input
                        type="text"
                        bind:value={app.browser}
                        disabled={busy}
                        placeholder="zen-browser, firefox, google-chrome"
                        class="mt-0.5 block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-xs font-mono focus:border-accent focus:outline-none"
                      />
                      <span class="text-[10px] text-fg-mute/60">empty = OS default handler</span>
                    </label>
                    <label class="flex items-center gap-1.5 text-xs mt-5">
                      <input
                        type="checkbox"
                        bind:checked={app.newWindow}
                        disabled={busy}
                        class="accent-primary"
                      />
                      <span class="text-fg-mute">new window</span>
                    </label>
                  </div>
                  <label class="block text-xs">
                    <span class="text-fg-mute">urls - one per line (tabs in one window)</span>
                    <textarea
                      value={(app.urls || []).join('\n')}
                      on:input={(e) =>
                        (app.urls = e.currentTarget.value
                          .split('\n')
                          .map((s) => s.trim())
                          .filter(Boolean))}
                      disabled={busy}
                      rows={Math.max(2, app.urls?.length || 1)}
                      placeholder="dc/gh/mux/prs&#10;https://app.greptile.com/..."
                      class="mt-0.5 block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-xs font-mono focus:border-accent focus:outline-none"
                    ></textarea>
                    <span class="text-[10px] text-fg-mute/60"
                      >{app.urls?.length || 0} url{(app.urls?.length || 0) === 1 ? '' : 's'} - dedicated
                      profile when browser is set</span
                    >
                  </label>
                {:else if app.type === 'open' || app.type === 'gh:repo-clone'}
                  <label class="block text-xs">
                    <span class="text-fg-mute">url - opened with OS handler</span>
                    <input
                      type="text"
                      bind:value={app.url}
                      disabled={busy}
                      placeholder="https://..."
                      class="mt-0.5 block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-xs font-mono focus:border-accent focus:outline-none"
                    />
                  </label>
                {:else}
                  <div class="grid grid-cols-2 gap-2">
                    <label class="block text-xs">
                      <span class="text-fg-mute">command</span>
                      <input
                        type="text"
                        bind:value={app.cmd}
                        disabled={busy}
                        placeholder="kitty, code, etc."
                        class="mt-0.5 block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-xs font-mono focus:border-accent focus:outline-none"
                      />
                    </label>
                    <label class="block text-xs">
                      <span class="text-fg-mute">cwd</span>
                      <input
                        type="text"
                        bind:value={app.cwd}
                        disabled={busy}
                        placeholder="working directory"
                        class="mt-0.5 block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-xs font-mono focus:border-accent focus:outline-none"
                      />
                    </label>
                  </div>
                  <label class="block text-xs">
                    <span class="text-fg-mute">arguments - one per line</span>
                    <textarea
                      value={(app.args || []).join('\n')}
                      on:input={(e) => (app.args = parseArgs(e.currentTarget.value))}
                      disabled={busy}
                      rows={Math.max(2, app.args?.length || 1)}
                      class="mt-0.5 block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-xs font-mono focus:border-accent focus:outline-none"
                    ></textarea>
                  </label>
                  {#if app.type === 'terminal'}
                    <label class="block text-xs">
                      <span class="text-fg-mute">terminal command - appended after -e</span>
                      <input
                        type="text"
                        bind:value={app.termCmd}
                        disabled={busy}
                        placeholder="e.g. claude or bash -c 'make; exec bash'"
                        class="mt-0.5 block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-xs font-mono focus:border-accent focus:outline-none"
                      />
                    </label>
                  {/if}
                {/if}
              </div>
            {/each}
          </div>
        {/if}
      </div>

      <div>
        <div class="flex items-center justify-between mb-2">
          <span class="text-xs font-semibold uppercase tracking-wide text-fg-mute"
            >Plugins ({editor.plugins.length})</span
          >
        </div>
        {#if plugins.length > 0}
          <div class="relative mb-2">
            <button
              type="button"
              on:click={() =>
                (openConfigPlugin = openConfigPlugin === '__picker__' ? null : '__picker__')}
              class="rounded bg-primary/15 px-2 py-0.5 text-[10px] text-primary hover:bg-primary/25"
              >+ Add plugin</button
            >
            {#if openConfigPlugin === '__picker__'}
              <div
                class="absolute left-0 top-full z-20 mt-1 w-52 rounded border border-bg-600/80 bg-bg-800 shadow-2xl"
              >
                <div class="p-1.5">
                  <input
                    type="text"
                    placeholder="filter..."
                    bind:value={pluginSearch}
                    on:click|stopPropagation
                    class="block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-[10px] text-fg-dim placeholder:text-fg-mute focus:border-accent focus:outline-none"
                  />
                </div>
                <div class="max-h-48 overflow-y-auto">
                  {#each plugins.filter((p) => !pluginSearch || (p.name || p.id)
                        .toLowerCase()
                        .includes(pluginSearch.toLowerCase())) as pl (pl.id)}
                    <button
                      type="button"
                      on:click={() => addPlugin(pl)}
                      class="block w-full text-left px-3 py-1.5 text-xs text-fg-dim hover:bg-accent/15 hover:text-fg"
                    >
                      {pl.name || pl.id} <span class="text-fg-mute">{pl.version}</span>
                    </button>
                  {/each}
                  {#if plugins.filter((p) => !pluginSearch || (p.name || p.id)
                        .toLowerCase()
                        .includes(pluginSearch.toLowerCase())).length === 0}
                    <p class="px-3 py-2 text-[10px] text-fg-mute">No plugins match</p>
                  {/if}
                </div>
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
              <div
                class="rounded border border-bg-600 bg-bg-800 {p &&
                pluginConfigSchema(p) &&
                Object.keys(pluginConfigSchema(p)).length > 0
                  ? 'border-primary/20'
                  : ''}"
              >
                <div class="flex items-center gap-2 px-3 py-2">
                  <span class="text-xs text-fg-dim flex-1">{p?.name || ref.id}</span>
                  <span class="text-[10px] text-fg-mute">{p?.version || ''}</span>
                  <button
                    type="button"
                    on:click={() => removePlugin(ref._key ?? '')}
                    class="text-accent-err hover:underline text-[10px]">remove</button
                  >
                  {#if p && pluginConfigSchema(p) && Object.keys(pluginConfigSchema(p)).length > 0}
                    <button
                      type="button"
                      on:click={() =>
                        (openConfigPlugin =
                          openConfigPlugin === (ref._key ?? '') ? null : (ref._key ?? ''))}
                      class="text-[10px] text-primary hover:underline"
                    >
                      {openConfigPlugin === ref._key ? 'hide config' : 'config'}
                    </button>
                  {/if}
                </div>
                {#if p && openConfigPlugin === ref._key && pluginConfigSchema(p) && Object.keys(pluginConfigSchema(p)).length > 0}
                  <div class="border-t border-bg-600 px-3 py-2 space-y-2">
                    {#each schemaEntries(pluginConfigSchema(p)) as [key, field] (key)}
                      <label class="block text-xs">
                        <span class="text-fg-mute">{field?.label || key}</span>
                        {#if configFieldType(field) === 'checkbox'}
                          <input
                            type="checkbox"
                            checked={pluginConfig(ref)[key] === true}
                            on:change={(e) => setPluginConfig(ref.id, key, e.currentTarget.checked)}
                            class="block accent-primary"
                          />
                        {:else if configFieldType(field) === 'number'}
                          <input
                            type="number"
                            value={pluginConfig(ref)[key] ?? configFieldDefault(field)}
                            on:input={(e) =>
                              setPluginConfig(ref.id, key, parseFloat(e.currentTarget.value) || 0)}
                            class="mt-0.5 block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-xs font-mono focus:border-accent focus:outline-none"
                          />
                        {:else if configFieldType(field) === 'select' && field.options}
                          <select
                            value={pluginConfig(ref)[key] ?? configFieldDefault(field)}
                            on:change={(e) => setPluginConfig(ref.id, key, e.currentTarget.value)}
                            class="mt-0.5 block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-xs font-mono focus:border-accent focus:outline-none"
                          >
                            {#each field.options as opt, i (i)}
                              <option value={opt}>{opt}</option>
                            {/each}
                          </select>
                        {:else}
                          <input
                            type="text"
                            value={pluginConfig(ref)[key] ?? configFieldDefault(field)}
                            on:input={(e) => setPluginConfig(ref.id, key, e.currentTarget.value)}
                            class="mt-0.5 block w-full rounded border border-bg-600 bg-bg-700 px-2 py-1 text-xs font-mono focus:border-accent focus:outline-none"
                          />
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
      <button
        type="button"
        on:click={del}
        disabled={busy}
        class="rounded bg-error/20 px-3 py-1.5 text-xs font-medium text-error hover:bg-error/30 disabled:opacity-50"
        >Delete</button
      >
      <div class="flex items-center gap-2">
        <button
          type="button"
          on:click={onclose}
          disabled={busy}
          class="rounded px-3 py-1.5 text-xs text-fg-dim hover:text-fg disabled:opacity-50"
          >Cancel</button
        >
        <button
          type="button"
          on:click={save}
          disabled={busy || !editor.name.trim()}
          class="rounded bg-primary px-3 py-1.5 text-xs font-medium text-primary-content hover:bg-primary/90 disabled:opacity-50"
          >{busy ? '...' : 'Save'}</button
        >
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
