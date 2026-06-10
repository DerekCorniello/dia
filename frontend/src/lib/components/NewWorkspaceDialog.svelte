<script lang="ts">
  import { api, describeError } from '../api';
  import { lastError } from '../stores';
  import { createEventDispatcher } from 'svelte';

  export let onClose: () => void = () => {};

  let name = '';
  let local = false;
  let busy = false;
  let nameError = '';
  const dispatch = createEventDispatcher<{ created: string }>();

  function validateName(v: string): string {
    if (!v) return '';
    if (!/^[a-z0-9]([a-z0-9-]*[a-z0-9])?$/.test(v)) {
      return 'must be lowercase a-z, 0-9, internal hyphens';
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

  async function submit() {
    if (!name.trim() || nameError) return;
    busy = true;
    lastError.set(null);
    try {
      await api.newWorkspace(name.trim(), local);
      dispatch('created', name.trim());
      onClose();
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
    <form on:submit|preventDefault={submit} class="space-y-3">
      <label class="block text-sm">
        <span class="text-fg-dim">Name</span>
        <input
          type="text"
          value={name}
          on:input={onNameInput}
          on:blur={() => (nameError = validateName(name))}
          disabled={busy}
          placeholder="my-project"
          class="mt-1 block w-full rounded border {nameError ? 'border-error/60' : 'border-bg-600'} bg-bg-800 px-2 py-1.5 text-sm font-mono focus:border-accent focus:outline-none"
        />
        {#if nameError}
          <span class="block text-xs text-error mt-1">{nameError}</span>
        {:else}
          <span class="block text-xs text-fg-mute mt-1">
            lowercase a-z, 0-9, internal hyphens
          </span>
        {/if}
      </label>

      <div>
        <span class="text-xs font-semibold uppercase tracking-wide text-fg-mute block mb-1.5">Location</span>
        <label class="flex items-center gap-2 text-sm mb-1.5">
          <input
            type="radio"
            bind:group={local}
            value={false}
            disabled={busy}
            class="accent-primary"
          />
          <span class="text-fg-dim">Global</span>
          <span class="text-xs text-fg-mute">(~/.config/dia/workspaces/)</span>
        </label>
        <label class="flex items-center gap-2 text-sm">
          <input
            type="radio"
            bind:group={local}
            value={true}
            disabled={busy}
            class="accent-primary"
          />
          <span class="text-fg-dim">Local</span>
          <span class="text-xs text-fg-mute">(./.dia/)</span>
        </label>
      </div>

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
