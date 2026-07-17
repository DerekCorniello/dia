<script lang="ts">
  import { toasts, dismissToast } from '../stores';
</script>

<div class="pointer-events-none fixed bottom-4 left-1/2 z-[60] flex -translate-x-1/2 flex-col items-center gap-2">
  {#each $toasts as t (t.id)}
    <div
      class="pointer-events-auto flex items-center gap-2 rounded px-3 py-1.5 text-xs shadow {t.kind ===
      'ok'
        ? 'border-l-2 border-success bg-success/15 text-success'
        : 'border-l-2 border-error bg-error/15 text-error'}"
    >
      <span class="break-all">{t.text}</span>
      <button
        class="shrink-0 opacity-60 hover:opacity-100"
        aria-label="dismiss"
        on:click={() => dismissToast(t.id)}>x</button
      >
    </div>
  {/each}
</div>
