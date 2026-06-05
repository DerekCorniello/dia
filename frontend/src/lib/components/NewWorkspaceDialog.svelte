<script lang="ts">
  import { api, describeError } from '../api';
  import { lastError, workspaces } from '../stores';
  import { createEventDispatcher } from 'svelte';

  export let onClose: () => void = () => {};

  let name = '';
  let busy = false;
  const dispatch = createEventDispatcher<{ created: void }>();

  async function submit() {
    if (!name.trim()) return;
    busy = true;
    lastError.set(null);
    try {
      await api.newWorkspace(name.trim());
      const list = await api.listWorkspaces();
      workspaces.set(list);
      dispatch('created');
      onClose();
    } catch (e) {
      lastError.set(`new workspace: ${describeError(e)}`);
    } finally {
      busy = false;
    }
  }
</script>

<div class="fixed inset-0 z-10 flex items-center justify-center bg-bg-900/70">
  <div class="w-96 rounded-lg border border-bg-600 bg-bg-700 p-5 shadow-lg">
    <h2 class="text-base font-medium mb-3">New workspace</h2>
    <form on:submit|preventDefault={submit} class="space-y-3">
      <label class="block text-sm">
        <span class="text-fg-dim">Name</span>
        <input
          type="text"
          bind:value={name}
          disabled={busy}
          placeholder="my-project"
          class="mt-1 block w-full rounded border border-bg-600 bg-bg-800 px-2 py-1.5 text-sm font-mono focus:border-accent focus:outline-none"
        />
        <span class="block text-xs text-fg-mute mt-1">
          lowercase a-z, 0-9, internal hyphens
        </span>
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
          disabled={busy || !name.trim()}
          class="rounded bg-accent/20 px-3 py-1.5 text-sm font-medium text-accent hover:bg-accent/30 disabled:opacity-50"
        >
          {busy ? '...' : 'Create'}
        </button>
      </div>
    </form>
  </div>
</div>
