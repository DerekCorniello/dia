<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { editableSlots, buildCustomThemeCss } from '../colors';
  import { findTheme, type ThemeMeta } from '../themes';
  import type { CustomThemeInfo } from '../api';

  // The existing custom theme to edit, or null for new.
  export let initial: CustomThemeInfo | null = null;

  // Pre-selected base theme id for the "start from" picker when
  // creating a new theme. Defaults to the active Dia dark.
  export let baseThemeId: string = 'dia';

  const dispatch = createEventDispatcher<{
    save: CustomThemeInfo;
    cancel: void;
  }>();

  const groupedSlots = groupSlots(editableSlots);

  let name = initial?.name ?? '';
  let colorScheme: 'light' | 'dark' = (initial?.color_scheme as 'light' | 'dark') ?? 'dark';
  let colors: Record<string, string> = { ...(initial?.colors ?? seedFromBase(baseThemeId)) };
  let nameError = '';
  let hexErrors: Record<string, string> = {};

  $: colorScheme = colorScheme;

  function groupSlots(slots: typeof editableSlots) {
    const groups: Record<string, typeof editableSlots> = { brand: [], base: [], state: [] };
    for (const s of slots) groups[s.group].push(s);
    return groups;
  }

  function seedFromBase(id: string): Record<string, string> {
    const meta: ThemeMeta | undefined = findTheme(id);
    if (!meta) return defaultColors();
    return paletteForBase(meta);
  }

  function defaultColors(): Record<string, string> {
    return {
      primary: '#7fdbca',
      primary_content: '#1b2636',
      secondary: '#ecc48d',
      secondary_content: '#1b2636',
      accent: '#ff5874',
      accent_content: '#ffffff',
      neutral: '#2f3d52',
      neutral_content: '#d6deeb',
      base_100: '#1b2636',
      base_200: '#243144',
      base_300: '#2f3d52',
      base_content: '#d6deeb',
      info: '#67b0e8',
      success: '#7fdbca',
      warning: '#ecc48d',
      error: '#ff5874',
    };
  }

  function paletteForBase(meta: ThemeMeta): Record<string, string> {
    return {
      primary: meta.swatch.primary,
      primary_content: meta.swatch.baseContent,
      secondary: meta.swatch.secondary,
      secondary_content: meta.swatch.baseContent,
      accent: meta.swatch.accent,
      accent_content: meta.swatch.base,
      neutral: meta.swatch.base,
      neutral_content: meta.swatch.baseContent,
      base_100: meta.swatch.base,
      base_200: meta.swatch.base,
      base_300: meta.swatch.base,
      base_content: meta.swatch.baseContent,
      info: meta.swatch.primary,
      success: '#22c55e',
      warning: '#eab308',
      error: '#ef4444',
    };
  }

  function changeBase(id: string) {
    colors = seedFromBase(id);
  }

  function isValidHex(v: string): boolean {
    return /^#[0-9a-fA-F]{6}$/.test(v);
  }

  function setColor(key: string, value: string) {
    if (!isValidHex(value)) {
      hexErrors = { ...hexErrors, [key]: 'must be #rrggbb' };
      return;
    }
    if (hexErrors[key]) {
      const next = { ...hexErrors };
      delete next[key];
      hexErrors = next;
    }
    colors = { ...colors, [key]: value };
  }

  function validateName(): string {
    if (!name.trim()) return 'name is required';
    if (name.length > 64) return 'name must be 64 characters or fewer';
    if (!/^[A-Za-z0-9_-]+$/.test(name)) return 'use letters, digits, hyphen, underscore';
    return '';
  }

  function save() {
    const err = validateName();
    if (err) {
      nameError = err;
      return;
    }
    nameError = '';
    if (Object.keys(hexErrors).length > 0) return;
    const cleaned: Record<string, string> = {};
    for (const slot of editableSlots) {
      const v = colors[slot.key];
      if (v) cleaned[slot.key] = v;
    }
    dispatch('save', {
      name: name.trim(),
      color_scheme: colorScheme,
      colors: cleaned,
    });
  }

  function cancel() {
    dispatch('cancel');
  }

  $: previewCss = buildCustomThemeCss({
    name: name.trim() || 'preview-tmp',
    colorScheme,
    colors,
  });

  // Inject a transient preview block while editing. We never
  // change document.documentElement.dataset.theme here so the
  // editor chrome stays in the host theme.
  let previewStyleEl: HTMLStyleElement | null = null;

  $: if (typeof document !== 'undefined') {
    if (!previewStyleEl) {
      previewStyleEl = document.createElement('style');
      previewStyleEl.setAttribute('data-theme-preview', '');
      document.head.appendChild(previewStyleEl);
    }
    previewStyleEl.textContent = previewCss;
  }

  export function destroy() {
    previewStyleEl?.remove();
    previewStyleEl = null;
  }
</script>

<div class="flex flex-col gap-4">
  <div class="grid gap-3 sm:grid-cols-2">
    <label class="block text-sm">
      <span class="text-fg-dim">Theme name</span>
      <input
        type="text"
        bind:value={name}
        placeholder="my-theme"
        class="mt-1 block w-full rounded border bg-bg-800 px-2 py-1.5 text-sm font-mono focus:outline-none {nameError
          ? 'border-accent-err'
          : 'border-bg-600 focus:border-accent'}"
      />
      {#if nameError}
        <span class="mt-1 block text-xs text-accent-err">{nameError}</span>
      {/if}
    </label>
    <div class="block text-sm">
      <span class="text-fg-dim">Color scheme</span>
      <div class="mt-1 flex gap-1 rounded border border-bg-600 bg-bg-800 p-0.5">
        <button
          type="button"
          on:click={() => (colorScheme = 'light')}
          class="flex-1 rounded px-2 py-1 text-xs {colorScheme === 'light'
            ? 'bg-accent/20 text-accent'
            : 'text-fg-dim hover:text-fg'}"
        >
          light
        </button>
        <button
          type="button"
          on:click={() => (colorScheme = 'dark')}
          class="flex-1 rounded px-2 py-1 text-xs {colorScheme === 'dark'
            ? 'bg-accent/20 text-accent'
            : 'text-fg-dim hover:text-fg'}"
        >
          dark
        </button>
      </div>
    </div>
  </div>

  <div class="text-sm">
    <span class="text-fg-dim">Start from</span>
    <div class="mt-1 flex flex-wrap gap-1">
      {#each ['dia', 'dia-light', 'github-dark', 'github-light', 'dracula', 'one-dark', 'tokyo-night', 'nord', 'catppuccin-mocha', 'catppuccin-latte'] as id (id)}
        <button
          type="button"
          on:click={() => changeBase(id)}
          class="rounded border border-bg-600 bg-bg-800 px-2 py-1 text-xs text-fg-dim hover:border-fg-mute hover:text-fg"
        >
          {id}
        </button>
      {/each}
    </div>
  </div>

  <div class="grid gap-4 sm:grid-cols-3">
    {#each Object.entries(groupedSlots) as [groupKey, slots] (groupKey)}
      <div>
        <h5 class="mb-2 text-[10px] font-semibold uppercase tracking-widest text-secondary">
          {groupKey}
        </h5>
        <div class="space-y-2">
          {#each slots as slot (slot.key)}
            <div class="flex items-center gap-2">
              <input
                type="color"
                value={colors[slot.key] ?? '#000000'}
                on:input={(e) => setColor(slot.key, e.currentTarget.value)}
                aria-label={slot.label}
                class="h-7 w-7 cursor-pointer rounded border border-bg-600 bg-transparent"
              />
              <div class="flex-1 min-w-0">
                <div class="truncate text-xs text-fg">{slot.label}</div>
                <input
                  type="text"
                  value={colors[slot.key] ?? ''}
                  on:change={(e) => setColor(slot.key, e.currentTarget.value)}
                  class="w-full rounded border bg-bg-800 px-1.5 py-0.5 font-mono text-[10px] focus:outline-none {hexErrors[
                    slot.key
                  ]
                    ? 'border-accent-err'
                    : 'border-bg-600 focus:border-accent'}"
                />
                {#if hexErrors[slot.key]}
                  <div class="mt-0.5 text-[10px] text-accent-err">
                    {hexErrors[slot.key]}
                  </div>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/each}
  </div>

  <div class="rounded border border-primary/30 bg-bg-800 p-3" data-theme={name.trim() || 'preview-tmp'}>
    <div class="mb-2 text-[10px] font-semibold uppercase tracking-widest text-primary">
      Preview
    </div>
    <div class="flex flex-wrap items-center gap-2 text-xs">
      <button class="btn btn-xs btn-primary">Primary</button>
      <button class="btn btn-xs btn-secondary">Secondary</button>
      <button class="btn btn-xs btn-accent">Accent</button>
      <span class="badge badge-info">info</span>
      <span class="badge badge-success">success</span>
      <span class="badge badge-warning">warning</span>
      <span class="badge badge-error">error</span>
      <span class="rounded bg-base-200 px-2 py-0.5 text-base-content">base 200</span>
      <span class="rounded bg-base-300 px-2 py-0.5 text-base-content">base 300</span>
    </div>
  </div>

  <div class="flex justify-end gap-2 border-t border-bg-600 pt-3">
    <button
      type="button"
      on:click={cancel}
      class="rounded px-3 py-1.5 text-sm text-fg-dim hover:text-fg"
    >
      Cancel
    </button>
    <button
      type="button"
      on:click={save}
      class="rounded bg-primary px-3 py-1.5 text-sm font-medium text-primary-content hover:bg-primary/90"
    >
      {initial ? 'Save' : 'Create'}
    </button>
  </div>
</div>
