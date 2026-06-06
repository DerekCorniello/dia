<script lang="ts">
  import { api, describeError } from '../api';
  import { lastError } from '../stores';
  import type { CheckInfo, PathsInfo } from '../api';

  export let doctor: CheckInfo[];
  export let paths: PathsInfo | null;
  export let plugins: string[];
  export let theme: string = 'dia';
  export let onThemeChange: (t: string) => void = () => {};
  export let onRefresh: () => void = () => {};
  export let onClose: () => void = () => {};

  const availableThemes = [
    { id: 'dia', label: 'Dia' },
    { id: 'dia-light', label: 'Dia Light' },
    { id: 'light', label: 'Light' },
    { id: 'dark', label: 'Dark' },
    { id: 'cupcake', label: 'Cupcake' },
    { id: 'bumblebee', label: 'Bumblebee' },
    { id: 'emerald', label: 'Emerald' },
    { id: 'corporate', label: 'Corporate' },
    { id: 'synthwave', label: 'Synthwave' },
    { id: 'retro', label: 'Retro' },
    { id: 'cyberpunk', label: 'Cyberpunk' },
    { id: 'valentine', label: 'Valentine' },
    { id: 'halloween', label: 'Halloween' },
    { id: 'garden', label: 'Garden' },
    { id: 'forest', label: 'Forest' },
    { id: 'aqua', label: 'Aqua' },
    { id: 'lofi', label: 'Lofi' },
    { id: 'pastel', label: 'Pastel' },
    { id: 'fantasy', label: 'Fantasy' },
    { id: 'wireframe', label: 'Wireframe' },
    { id: 'black', label: 'Black' },
    { id: 'luxury', label: 'Luxury' },
    { id: 'dracula', label: 'Dracula' },
    { id: 'cmyk', label: 'CMYK' },
    { id: 'autumn', label: 'Autumn' },
    { id: 'business', label: 'Business' },
    { id: 'acid', label: 'Acid' },
    { id: 'lemonade', label: 'Lemonade' },
    { id: 'night', label: 'Night' },
    { id: 'coffee', label: 'Coffee' },
    { id: 'winter', label: 'Winter' },
    { id: 'dim', label: 'Dim' },
    { id: 'nord', label: 'Nord' },
    { id: 'sunset', label: 'Sunset' },

  ];

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

<div class="fixed inset-0 z-50 flex items-center justify-center bg-bg-900/70 p-4">
  <div class="flex max-h-[calc(100vh-2rem)] w-[min(56rem,calc(100vw-2rem))] flex-col overflow-hidden rounded-lg border border-bg-600 bg-bg-700 p-4 shadow-lg">
    <div class="mb-3 flex items-center justify-between">
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

    <div class="min-h-0 flex-1 overflow-y-auto pr-1">
      <section class="mb-4">
        <h3 class="mb-1 text-xs font-semibold uppercase tracking-wide text-fg-mute">Theme</h3>
        <select
          value={theme}
          on:change={(e) => onThemeChange(e.currentTarget.value)}
          class="select select-bordered select-sm w-full max-w-xs bg-bg-800 text-fg font-mono text-xs"
        >
          {#each availableThemes as t}
            <option value={t.id}>{t.label}</option>
          {/each}
        </select>
      </section>

      <section class="mb-4">
        <h3 class="mb-1 text-xs font-semibold uppercase tracking-wide text-fg-mute">Paths</h3>
        {#if paths}
          <dl class="space-y-1 font-mono text-xs">
            <div class="flex gap-2">
              <dt class="w-32 shrink-0 text-fg-mute">global config</dt>
              <dd class="break-all text-fg-dim">{paths.global_config_dir}</dd>
            </div>
            <div class="flex gap-2">
              <dt class="w-32 shrink-0 text-fg-mute">state dir</dt>
              <dd class="break-all text-fg-dim">{paths.state_dir}</dd>
            </div>
            <div class="flex gap-2">
              <dt class="w-32 shrink-0 text-fg-mute">state file</dt>
              <dd class="break-all text-fg-dim">{paths.state_file}</dd>
            </div>
          </dl>
        {:else}
          <p class="text-xs text-fg-mute">loading...</p>
        {/if}
      </section>

      <section class="mb-4">
        <h3 class="mb-1 text-xs font-semibold uppercase tracking-wide text-fg-mute">Doctor</h3>
        <ul class="space-y-0.5 text-xs">
          {#each doctor as c}
            <li class="flex gap-2">
              <span
                class="w-10 shrink-0 rounded px-1 text-center text-[10px] font-semibold uppercase {c.status === 'ok' ? 'bg-accent/20 text-accent' : c.status === 'warn' ? 'bg-accent-warn/20 text-accent-warn' : 'bg-accent-err/20 text-accent-err'}"
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
        <h3 class="mb-1 text-xs font-semibold uppercase tracking-wide text-fg-mute">
          Plugins ({plugins.length})
        </h3>
        {#if plugins.length === 0}
          <p class="text-xs text-fg-mute">no dia-* on PATH</p>
        {:else}
          <ul class="space-y-0.5 font-mono text-xs">
            {#each plugins as p}
              <li class="break-all text-fg-dim">{p}</li>
            {/each}
          </ul>
        {/if}
      </section>
    </div>

    <div class="mt-3 flex flex-wrap gap-2 border-t border-bg-600 pt-3">
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
</div>
