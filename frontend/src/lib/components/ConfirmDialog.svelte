<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { fly, fade } from 'svelte/transition';

  export let title = 'Confirm';
  export let message = '';
  export let confirmLabel = 'Delete';
  export let danger = true;

  const dispatch = createEventDispatcher<{ confirm: void; cancel: void }>();

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape') dispatch('cancel');
  }

  function onConfirm() {
    dispatch('confirm');
  }

  function onCancel() {
    dispatch('cancel');
  }

  function onBackdrop(e: MouseEvent) {
    if (e.target === e.currentTarget) dispatch('cancel');
  }
</script>

<svelte:window on:keydown={onKey} />

<div
  class="fixed inset-0 z-50 flex items-center justify-center bg-bg-900/80"
  on:click={onBackdrop}
  role="presentation"
>
  <div
    class="w-96 rounded-lg border border-primary/15 bg-bg-700 p-5 shadow-lg"
    role="alertdialog"
    aria-modal="true"
    aria-label={title}
  >
    <h2 class="text-base font-medium text-fg mb-2">{title}</h2>
    <p class="text-sm text-fg-dim mb-5">{message}</p>
    <div class="flex justify-end gap-2">
      <button
        type="button"
        on:click={onCancel}
        class="rounded px-3 py-1.5 text-sm text-fg-dim hover:text-fg"
      >
        Cancel
      </button>
      <button
        type="button"
        on:click={onConfirm}
        class="rounded px-3 py-1.5 text-sm font-medium {danger ? 'bg-error text-white hover:bg-error/90' : 'bg-primary text-primary-content hover:bg-primary/90'}"
      >
        {confirmLabel}
      </button>
    </div>
  </div>
</div>