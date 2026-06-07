<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher, tick } from 'svelte';
  import { slide } from 'svelte/transition';
  import { api, describeError, type PluginInfo, type PluginActionDef } from '../api';
  import { lastError } from '../stores';
  import ConfirmDialog from './ConfirmDialog.svelte';

  export let plugin: PluginInfo;

  const dispatch = createEventDispatcher<{
    refresh: void;
  }>();

  let data: any = null;
  let loading = false;
  let error: string | null = null;
  let showDetails = false;
  let showActionConfirm = false;
  let pendingAction: PluginActionDef | null = null;
  let mountedAt = 0;
  let canvasEl: HTMLCanvasElement | null = null;
  let strokes: Stroke[] = [];
  let isDrawing = false;
  let lastPt: { x: number; y: number } | null = null;

  type Stroke = { color: string; width: number; points: { x: number; y: number }[] };

  async function fetchData() {
    loading = true;
    error = null;
    try {
      const raw = await api.pluginCall(plugin.id, 'getData', '[]');
      if (raw === '' || raw == null) {
        data = null;
      } else {
        data = JSON.parse(raw);
      }
      if (plugin.ui.type === 'canvas') {
        await renderCanvas();
      }
    } catch (e) {
      error = describeError(e);
    } finally {
      loading = false;
    }
  }

  async function onAction(action: PluginActionDef) {
    if (action.confirm) {
      pendingAction = action;
      showActionConfirm = true;
      return;
    }
    await executeAction(action);
  }

  async function executeAction(action: PluginActionDef) {
    let ctx: any = {};
    if (plugin.ui.type === 'canvas') {
      ctx.strokes = strokes;
    }
    try {
      const raw = await api.pluginCall(
        plugin.id,
        'onAction',
        JSON.stringify([action.id, ctx]),
      );
      if (plugin.ui.type === 'canvas' && raw) {
        try {
          const next = JSON.parse(raw);
          if (Array.isArray(next?.strokes)) {
            strokes = next.strokes;
            await renderCanvas();
          }
        } catch {
          // ignore parse errors; canvas keeps current state
        }
      }
      dispatch('refresh');
    } catch (e) {
      lastError.set(`plugin action: ${describeError(e)}`);
    }
  }

  function onRefreshClick() {
    fetchData();
  }

  function asList(v: any): Array<{ id: any; label: any; [k: string]: any }> {
    if (!Array.isArray(v)) return [];
    return v.map((item, i) => {
      if (item == null) return { id: i, label: '' };
      if (typeof item !== 'object') return { id: i, label: String(item) };
      return { id: item.id ?? i, label: item.label ?? item.name ?? item.title ?? String(item), ...item };
    });
  }

  function asKv(v: any): Array<{ key: string; value: any }> {
    if (v == null || typeof v !== 'object' || Array.isArray(v)) return [];
    return Object.entries(v).map(([k, val]) => ({ key: k, value: val }));
  }

  function asText(v: any): string {
    if (v == null) return '';
    if (typeof v === 'string') return v;
    return JSON.stringify(v, null, 2);
  }

  function asTable(v: any): Array<Record<string, any>> {
    if (!Array.isArray(v)) return [];
    return v.filter((r) => r != null && typeof r === 'object') as Array<Record<string, any>>;
  }

  function formatCell(col: { key: string; label: string; format?: string }, v: any): string {
    if (v == null) return '';
    if (col.format === 'duration' && typeof v === 'number') {
      const sec = Math.floor(v / 1000);
      if (sec < 60) return `${sec}s`;
      if (sec < 3600) return `${Math.floor(sec / 60)}m ${sec % 60}s`;
      return `${Math.floor(sec / 3600)}h ${Math.floor((sec % 3600) / 60)}m`;
    }
    if (col.format === 'badge') return String(v);
    if (typeof v === 'boolean') return v ? 'yes' : 'no';
    return String(v);
  }

  function colorForPlugin(p: PluginInfo): string {
    const s = p.id + p.name;
    let h = 0;
    for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) & 0xffff;
    return `hsl(${h % 360} 60% 50%)`;
  }

  async function renderCanvas() {
    if (!canvasEl) return;
    const ctx = canvasEl.getContext('2d');
    if (!ctx) return;
    const dpr = window.devicePixelRatio || 1;
    const w = canvasEl.clientWidth;
    const h = canvasEl.clientHeight;
    canvasEl.width = Math.floor(w * dpr);
    canvasEl.height = Math.floor(h * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.fillStyle = '#ffffff';
    ctx.fillRect(0, 0, w, h);
    for (const s of strokes) {
      if (s.points.length === 0) continue;
      ctx.strokeStyle = s.color || '#000000';
      ctx.lineWidth = s.width || 2;
      ctx.lineCap = 'round';
      ctx.lineJoin = 'round';
      ctx.beginPath();
      const first = s.points[0];
      ctx.moveTo(first.x, first.y);
      for (let i = 1; i < s.points.length; i++) {
        const p = s.points[i];
        ctx.lineTo(p.x, p.y);
      }
      if (s.points.length === 1) {
        ctx.lineTo(first.x + 0.01, first.y + 0.01);
      }
      ctx.stroke();
    }
  }

  function canvasPt(ev: PointerEvent): { x: number; y: number } {
    if (!canvasEl) return { x: 0, y: 0 };
    const r = canvasEl.getBoundingClientRect();
    return { x: ev.clientX - r.left, y: ev.clientY - r.top };
  }

  function onPointerDown(ev: PointerEvent) {
    if (!canvasEl) return;
    canvasEl.setPointerCapture(ev.pointerId);
    isDrawing = true;
    const p = canvasPt(ev);
    lastPt = p;
    const color = (data && typeof data === 'object' && data.color) || '#000000';
    const width = (data && typeof data === 'object' && data.width) || 2;
    strokes = [...strokes, { color, width, points: [p] }];
  }

  function onPointerMove(ev: PointerEvent) {
    if (!isDrawing) return;
    const p = canvasPt(ev);
    const cur = strokes[strokes.length - 1];
    if (!cur) return;
    cur.points = [...cur.points, p];
    strokes = strokes;
    lastPt = p;
    if (canvasEl) {
      const ctx = canvasEl.getContext('2d');
      if (ctx) {
        ctx.strokeStyle = cur.color;
        ctx.lineWidth = cur.width;
        ctx.lineCap = 'round';
        ctx.lineJoin = 'round';
        ctx.beginPath();
        if (cur.points.length >= 2) {
          const a = cur.points[cur.points.length - 2];
          const b = cur.points[cur.points.length - 1];
          ctx.moveTo(a.x, a.y);
          ctx.lineTo(b.x, b.y);
        } else {
          ctx.moveTo(p.x, p.y);
          ctx.lineTo(p.x + 0.01, p.y + 0.01);
        }
        ctx.stroke();
      }
    }
  }

  function onPointerUp(ev: PointerEvent) {
    if (!isDrawing) return;
    isDrawing = false;
    lastPt = null;
    if (canvasEl) {
      try {
        canvasEl.releasePointerCapture(ev.pointerId);
      } catch {
        // ignore: pointer was already released
      }
    }
  }

  function clearCanvas() {
    strokes = [];
    renderCanvas();
  }

  async function confirmAction() {
    const action = pendingAction;
    showActionConfirm = false;
    pendingAction = null;
    if (action) await executeAction(action);
  }

  function cancelAction() {
    showActionConfirm = false;
    pendingAction = null;
  }

  function colorForAction(p: PluginInfo): string {
    return colorForPlugin(p);
  }

  $: items = asList(data);
  $: kv = asKv(data);
  $: tableRows = asTable(data);
  $: text = asText(data);

  onMount(async () => {
    mountedAt = Date.now();
    await tick();
    await fetchData();
  });

  onDestroy(() => {
    // nothing to clean up; canvas state is host-only
  });
</script>

<section class="relative rounded-lg border border-bg-600 bg-bg-700 p-3 shadow-sm {plugin.enabled ? '' : 'bg-bg-800/50'}">
  <div
    class="absolute left-0 top-2 bottom-2 w-1 rounded-r {plugin.enabled
      ? (plugin.source === 'local' ? 'bg-info' : 'bg-accent-secondary')
      : 'bg-base-300'}"
  ></div>
  <header class="mb-2 flex items-center justify-between gap-2">
    <div class="min-w-0">
      <div class="flex flex-wrap items-center gap-2">
        <h3 class="text-sm font-semibold text-fg truncate">{plugin.ui.title || plugin.name}</h3>
        <span class="inline-flex items-center rounded-full {plugin.source === 'local' ? 'bg-info/15 text-info' : 'bg-accent-secondary/15 text-accent-secondary'} px-1.5 py-0.5 text-[10px] font-medium">
          {plugin.source}
        </span>
        {#if !plugin.enabled}
          <span class="inline-flex items-center rounded-full bg-error/10 px-1.5 py-0.5 text-[10px] font-medium text-error">disabled</span>
        {/if}
      </div>
      {#if plugin.description}
        <p class="mt-0.5 text-xs text-fg-mute">{plugin.description}</p>
      {/if}
      {#if plugin.description || plugin.longDescription}
        <button
          type="button"
          on:click={() => (showDetails = !showDetails)}
          class="mt-0.5 inline-flex items-center gap-1 text-[10px] text-fg-mute hover:text-primary transition-colors"
        >
          {showDetails ? 'less' : 'more details'}
          <svg class="h-3 w-3 transition-transform {showDetails ? 'rotate-180' : ''}" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z" clip-rule="evenodd" />
          </svg>
        </button>
      {/if}
    </div>
    <div class="flex items-center gap-2">
      {#if plugin.ui.refreshable}
        <button
          type="button"
          on:click={onRefreshClick}
          disabled={loading || !plugin.enabled}
          class="rounded bg-bg-600 px-2 py-1 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg disabled:opacity-50"
        >
          {loading ? 'loading...' : 'refresh'}
        </button>
      {/if}
      {#each plugin.actions ?? [] as a (a.id)}
        <button
          type="button"
          on:click={() => onAction(a)}
          disabled={!plugin.enabled}
          class="rounded px-2 py-1 text-[10px] text-fg disabled:opacity-50"
          style="background: {colorForAction(plugin)}33; border: 1px solid {colorForAction(plugin)}66;"
        >
          {a.label}
        </button>
      {/each}
    </div>
  </header>

  {#if !plugin.enabled}
    <p class="text-xs text-fg-mute">Plugin is disabled. Enable it in Settings to interact with it.</p>
  {:else if error}
    <p class="text-xs text-error">{error}</p>
  {:else if plugin.ui.type === 'text'}
    <pre class="overflow-x-auto whitespace-pre-wrap rounded bg-bg-800 p-3 text-xs text-fg-dim">{text}</pre>
  {:else if plugin.ui.type === 'kv'}
    {#if kv.length === 0}
      <p class="text-xs text-fg-mute">no data</p>
    {:else}
      <dl class="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 text-sm">
        {#each kv as row (row.key)}
          <dt class="text-fg-mute">{row.key}</dt>
          <dd class="break-all text-fg-dim">{row.value}</dd>
        {/each}
      </dl>
    {/if}
  {:else if plugin.ui.type === 'list'}
    {#if items.length === 0}
      <p class="text-xs text-fg-mute">empty</p>
    {:else}
      <ul class="space-y-1">
        {#each items as it (it.id)}
          <li class="rounded bg-bg-800 px-2 py-1.5 text-sm text-fg-dim">
            <span class="text-fg">{it.label}</span>
            {#if it.detail}
              <span class="ml-2 text-xs text-fg-mute">{it.detail}</span>
            {/if}
          </li>
        {/each}
      </ul>
    {/if}
  {:else if plugin.ui.type === 'grid'}
    {#if items.length === 0}
      <p class="text-xs text-fg-mute">empty</p>
    {:else}
      <div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
        {#each items as it (it.id)}
          <div class="rounded bg-bg-800 p-2 text-sm">
            <div class="text-fg">{it.label}</div>
            {#if it.detail}
              <div class="text-xs text-fg-mute">{it.detail}</div>
            {/if}
          </div>
        {/each}
      </div>
    {/if}
  {:else if plugin.ui.type === 'table'}
    {#if tableRows.length === 0}
      <p class="text-xs text-fg-mute">empty</p>
    {:else}
      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-left text-[10px] uppercase tracking-wide text-fg-mute">
              {#each plugin.ui.columns ?? [] as col (col.key)}
                <th class="px-2 py-1">{col.label}</th>
              {/each}
            </tr>
          </thead>
          <tbody>
            {#each tableRows as row, i (i)}
              <tr class="border-t border-bg-600">
                {#each plugin.ui.columns ?? [] as col (col.key)}
                  <td class="px-2 py-1 text-fg-dim">{formatCell(col, row[col.key])}</td>
                {/each}
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  {:else if plugin.ui.type === 'canvas'}
    <div class="flex flex-col gap-2">
      <canvas
        bind:this={canvasEl}
        on:pointerdown={onPointerDown}
        on:pointermove={onPointerMove}
        on:pointerup={onPointerUp}
        on:pointercancel={onPointerUp}
        on:pointerleave={onPointerUp}
        class="block w-full touch-none rounded border border-bg-600 bg-white"
        style="height: 50vh; cursor: crosshair;"
      ></canvas>
      <div class="flex items-center justify-between text-[10px] text-fg-mute">
        <span>{strokes.length} stroke{strokes.length === 1 ? '' : 's'}</span>
        <button
          type="button"
          on:click={clearCanvas}
          class="rounded bg-bg-600 px-2 py-0.5 text-[10px] text-fg-dim hover:bg-bg-600/70 hover:text-fg"
        >
          clear
        </button>
      </div>
    </div>
  {/if}

  {#if showDetails}
    <div class="mt-3 border-t border-bg-600 pt-3" transition:slide={{ duration: 200 }}>
      <p class="text-xs text-fg-dim whitespace-pre-line">{plugin.longDescription || plugin.description}</p>
    </div>
  {/if}
</section>

{#if showActionConfirm && pendingAction}
  <ConfirmDialog
    title="Run action"
    message="Run &quot;{pendingAction.label}&quot;?"
    confirmLabel="Run"
    danger={false}
    on:confirm={confirmAction}
    on:cancel={cancelAction}
  />
{/if}
