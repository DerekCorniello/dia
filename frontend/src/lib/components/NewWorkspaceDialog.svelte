<script lang="ts">
  import { api, describeError } from '../api';
  import { createEventDispatcher, onMount } from 'svelte';

  export let onClose: () => void = () => {};

  let name = '';
  let global = false;
  let customDir = '';
  let cwd = '';
  let busy = false;
  let nameError = '';
  const dispatch = createEventDispatcher<{ created: string }>();

  type LocationOption = 'cwd' | 'home' | 'custom';
  let location: LocationOption = 'cwd';

  onMount(async () => {
    try {
      cwd = await api.getCwd();
    } catch {
      cwd = '';
    }
  });

  function validateName(v: string): string {
    if (!v) return '';
    if (!/^[a-zA-Z0-9_-]+$/.test(v)) {
      return 'must be alphanumeric with hyphens or underscores';
    }
    if (v.length > 64) return 'name too long (max 64)';
    return '';
  }

  function onNameInput(e: Event) {
    name = (e.target as HTMLInputElement).value;
    nameError = validateName(name);
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape') onClose();
  }

  function resolveDir(): string {
    if (global) return '';
    if (location === 'cwd') return cwd;
    if (location === 'home') return '';
    return customDir;
  }

  async function submit() {
    if (!name.trim() || nameError) return;
    if (!global && location === 'custom' && !customDir.trim()) {
      nameError = 'Custom directory is required';
      return;
    }
    if (!global && location === 'custom') {
      // resolve ~
      const dir = customDir.trim().replace(/^~/, await api.getHomeDir().catch(() => ''));
      if (!dir) return;
      await api.newWorkspace(name.trim(), dir);
    } else {
      await api.newWorkspace(name.trim(), resolveDir());
    }
    dispatch('created', name.trim());
    onClose();
  }

  async function handleSubmit(e: Event) {
    e.preventDefault();
    busy = true;
    try {
      await submit();
    } catch (e) {
      nameError = describeError(e);
    } finally {
      busy = false;
    }
  }
</script>

<svelte:window on:keydown={onKey} />

<div
  class="fixed inset-0 z-10 flex items-center justify-center bg-bg-900/80"
  on:click|self={onClose}
  on:keydown|self={(e) => e.key === 'Escape' && onClose()}
  role="presentation"
>
  <div
    class="w-96 rounded-lg border border-primary/15 bg-bg-700 p-5 shadow-lg"
    role="dialog"
    aria-modal="true"
    aria-label="new workspace"
  >
    <h2 class="text-base font-medium mb-3">New workspace</h2>
    <form on:submit={handleSubmit} class="space-y-3">
      <label class="block text-sm">
        <span class="text-fg-dim">Name</span>
        <input
          type="text"
          value={name}
          on:input={onNameInput}
          on:blur={() => (nameError = validateName(name))}
          disabled={busy}
          placeholder="my-project"
          class="mt-1 block w-full rounded border {nameError
            ? 'border-error/60'
            : 'border-bg-600'} bg-bg-800 px-2 py-1.5 text-sm font-mono focus:border-accent focus:outline-none"
        />
        {#if nameError}
          <span class="block text-xs text-error mt-1">{nameError}</span>
        {:else}
          <span class="block text-xs text-fg-mute mt-1">
            alphanumeric with hyphens or underscores
          </span>
        {/if}
      </label>

      <div>
        <span class="text-xs font-semibold uppercase tracking-wide text-fg-mute block mb-1.5"
          >Location</span
        >

        {#if !global}
          <label class="flex items-center gap-2 text-sm mb-1.5">
            <input
              type="radio"
              bind:group={location}
              value="cwd"
              disabled={busy}
              class="accent-primary"
            />
            <span class="text-fg-dim">Current directory</span>
            {#if cwd}
              <span class="text-xs text-fg-mute font-mono">({cwd})</span>
            {/if}
          </label>
          <label class="flex items-center gap-2 text-sm mb-1.5">
            <input
              type="radio"
              bind:group={location}
              value="home"
              disabled={busy}
              class="accent-primary"
            />
            <span class="text-fg-dim">Home</span>
            <span class="text-xs text-fg-mute font-mono">(~)</span>
          </label>
          <label class="flex items-center gap-2 text-sm">
            <input
              type="radio"
              bind:group={location}
              value="custom"
              disabled={busy}
              class="accent-primary"
            />
            <span class="text-fg-dim">Custom</span>
          </label>
          {#if location === 'custom'}
            <input
              type="text"
              bind:value={customDir}
              disabled={busy}
              placeholder="/path/to/project"
              class="mt-1 block w-full rounded border border-bg-600 bg-bg-800 px-2 py-1 text-sm font-mono focus:border-accent focus:outline-none"
            />
          {/if}
        {:else}
          <p class="text-sm text-fg-mute">Saved to ~/.config/dia/workspaces/</p>
        {/if}
      </div>

      <label class="flex items-center gap-2 text-sm">
        <input type="checkbox" bind:checked={global} disabled={busy} class="accent-primary" />
        <span class="text-fg-dim">Global</span>
        <span class="text-xs text-fg-mute">(shared across all projects)</span>
      </label>

      <div class="flex justify-end gap-2 pt-2">
        <button
          type="button"
          on:click={onClose}
          disabled={busy}
          class="rounded px-3 py-1.5 text-sm text-fg-dim hover:text-fg disabled:opacity-50"
        >
          Cancel
        </button>
        <button
          type="submit"
          disabled={busy || !name.trim() || !!nameError}
          class="rounded bg-primary px-3 py-1.5 text-sm font-medium text-primary-content hover:bg-primary/90 disabled:opacity-50"
        >
          {busy ? '...' : 'Create'}
        </button>
      </div>
    </form>
  </div>
</div>
