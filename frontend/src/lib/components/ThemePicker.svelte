<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import {
    builtInThemes,
    groupOrder,
    groupLabels,
    type ThemeMeta,
    type ThemeGroup,
  } from '../themes';
  import type { CustomThemeInfo } from '../api';
  import { customThemes } from '../stores';

  export let activeTheme: string;

  const dispatch = createEventDispatcher<{
    select: { id: string };
    editCustom: { name: string };
    newCustom: void;
    deleteCustom: { name: string };
  }>();

  let query = '';

  $: filteredBuiltIn = filterThemes(builtInThemes, query);
  $: filteredCustom = filterCustomThemes($customThemes, query);
  $: groupedBuiltIn = groupThemes(filteredBuiltIn);
  $: hasResults = filteredBuiltIn.length > 0 || filteredCustom.length > 0;

  function filterThemes(themes: ThemeMeta[], q: string): ThemeMeta[] {
    if (!q.trim()) return themes;
    const needle = q.trim().toLowerCase();
    return themes.filter(
      (t) =>
        t.label.toLowerCase().includes(needle) ||
        t.id.toLowerCase().includes(needle) ||
        groupLabels[t.group].toLowerCase().includes(needle),
    );
  }

  function filterCustomThemes(
    themes: CustomThemeInfo[],
    q: string,
  ): CustomThemeInfo[] {
    if (!q.trim()) return themes;
    const needle = q.trim().toLowerCase();
    return themes.filter((t) => t.name.toLowerCase().includes(needle));
  }

  function groupThemes(themes: ThemeMeta[]): Record<ThemeGroup, ThemeMeta[]> {
    const out = {} as Record<ThemeGroup, ThemeMeta[]>;
    for (const t of themes) {
      if (!out[t.group]) out[t.group] = [];
      out[t.group].push(t);
    }
    return out;
  }

  function customSwatch(t: CustomThemeInfo) {
    return {
      base: t.colors.base_100 ?? '#1b2636',
      baseContent: t.colors.base_content ?? '#d6deeb',
      primary: t.colors.primary ?? '#7fdbca',
      secondary: t.colors.secondary ?? '#ecc48d',
      accent: t.colors.accent ?? '#ff5874',
    };
  }

  function selectTheme(id: string) {
    dispatch('select', { id });
  }

  function startEditCustom(name: string) {
    dispatch('editCustom', { name });
  }

  function startNewCustom() {
    dispatch('newCustom');
  }

  function confirmDelete(name: string) {
    dispatch('deleteCustom', { name });
  }
</script>

<div class="flex flex-col gap-3">
  <div class="flex items-center gap-2">
    <div class="relative flex-1">
      <input
        type="text"
        bind:value={query}
        placeholder="filter themes"
        aria-label="filter themes"
        class="w-full rounded border border-bg-600 bg-bg-800 px-3 py-1.5 pr-8 text-sm text-fg placeholder:text-fg-mute focus:border-accent focus:outline-none"
      />
      {#if query}
        <button
          type="button"
          on:click={() => (query = '')}
          class="absolute right-1.5 top-1/2 -translate-y-1/2 rounded p-1 text-fg-mute hover:text-primary"
          aria-label="clear filter"
        >
          x
        </button>
      {/if}
    </div>
    <button
      type="button"
      on:click={startNewCustom}
      class="rounded bg-primary/20 px-3 py-1.5 text-sm font-medium text-primary hover:bg-primary/30"
    >
      + New custom
    </button>
  </div>

  {#if !hasResults}
    <div class="rounded border border-dashed border-bg-600 p-6 text-center text-sm text-fg-mute">
      no themes match
    </div>
  {/if}

  {#each groupOrder as group (group)}
    {#if groupedBuiltIn[group] && groupedBuiltIn[group].length > 0}
      <section>
        <h4 class="mb-2 text-[10px] font-semibold uppercase tracking-widest text-secondary">
          {groupLabels[group]}
        </h4>
        <div class="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
          {#each groupedBuiltIn[group] as t (t.id)}
            {@const selected = t.id === activeTheme}
            <button
              type="button"
              on:click={() => selectTheme(t.id)}
              class="group flex flex-col gap-1.5 rounded-lg border p-2.5 text-left transition focus:outline-none focus:ring-2 focus:ring-primary {selected
                ? 'border-primary bg-primary/5 shadow-glow-sm'
                : 'border-bg-600 bg-bg-800 hover:border-primary/40'}"
              aria-pressed={selected}
            >
              <div
                class="relative h-8 w-full overflow-hidden rounded border border-bg-600"
                style="background-color: {t.swatch.base};"
              >
                <div class="absolute inset-0 flex items-center justify-center gap-1.5">
                  <div class="h-3 w-6 rounded-sm" style="background-color: {t.swatch.primary};"></div>
                  <div class="h-3 w-6 rounded-sm" style="background-color: {t.swatch.secondary};"></div>
                  <div class="h-3 w-6 rounded-sm" style="background-color: {t.swatch.accent};"></div>
                </div>
              </div>
              <div class="flex items-center justify-between gap-1 px-0.5">
                <span class="truncate text-xs font-medium text-fg">{t.label}</span>
                <span
                  class="shrink-0 text-[9px] font-mono uppercase opacity-60"
                  style="color: {t.swatch.baseContent};"
                >
                  {t.scheme}
                </span>
              </div>
            </button>
          {/each}
        </div>
      </section>
    {/if}
  {/each}

  {#if filteredCustom.length > 0}
    <section>
      <h4 class="mb-2 text-[10px] font-semibold uppercase tracking-widest text-secondary">
        Custom
      </h4>
      <div class="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
        {#each filteredCustom as t (t.name)}
          {@const selected = t.name === activeTheme}
          {@const swatch = customSwatch(t)}
          <div
            class="group relative flex flex-col gap-1.5 rounded-lg border p-2.5 text-left transition focus-within:ring-2 focus-within:ring-primary {selected
              ? 'border-primary bg-primary/5 shadow-glow-sm'
              : 'border-bg-600 bg-bg-800 hover:border-primary/40'}"
          >
            <button
              type="button"
              on:click={() => selectTheme(t.name)}
              class="absolute inset-0 z-0 rounded-lg"
              aria-pressed={selected}
              aria-label={`select ${t.name}`}
            ></button>
            <div
              class="pointer-events-none relative h-8 w-full overflow-hidden rounded border border-bg-600"
              style="background-color: {swatch.base};"
            >
              <div class="absolute inset-0 flex items-center justify-center gap-1.5">
                <div class="h-3 w-6 rounded-sm" style="background-color: {swatch.primary};"></div>
                <div class="h-3 w-6 rounded-sm" style="background-color: {swatch.secondary};"></div>
                <div class="h-3 w-6 rounded-sm" style="background-color: {swatch.accent};"></div>
              </div>
            </div>
            <div class="flex items-center justify-between gap-1 px-0.5">
              <span class="truncate text-xs font-medium text-fg">{t.name}</span>
              <div class="relative z-10 flex shrink-0 items-center gap-1 opacity-0 transition group-hover:opacity-100">
                <button
                  type="button"
                  on:click|stopPropagation={() => startEditCustom(t.name)}
                  class="rounded bg-bg-600/80 px-1.5 py-0.5 text-[10px] text-fg-dim hover:bg-bg-600 hover:text-fg"
                  aria-label={`edit ${t.name}`}
                >
                  edit
                </button>
                <button
                  type="button"
                  on:click|stopPropagation={() => confirmDelete(t.name)}
                  class="rounded bg-error/10 px-1.5 py-0.5 text-[10px] text-error hover:bg-error/20"
                  aria-label={`delete ${t.name}`}
                >
                  del
                </button>
              </div>
            </div>
          </div>
        {/each}
      </div>
    </section>
  {/if}
</div>
