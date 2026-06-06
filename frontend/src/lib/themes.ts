// Theme metadata for the picker. The picker groups themes by family
// and shows a small color swatch so users can pick by feel rather
// than by name. The actual theme definitions live in tailwind.config.js
// (built-ins) and in the state store (custom themes); this file is
// pure presentation data.
//
// Swatches use the same hex values daisyUI ends up with, so the
// swatch on the card matches the live preview at runtime.

export type ThemeGroup =
  | 'dia'
  | 'light'
  | 'dark'
  | 'catppuccin'
  | 'editor'
  | 'classic';

export interface ThemeSwatch {
  base: string;
  baseContent: string;
  primary: string;
  secondary: string;
  accent: string;
}

export interface ThemeMeta {
  id: string;
  label: string;
  group: ThemeGroup;
  scheme: 'light' | 'dark';
  swatch: ThemeSwatch;
}

export const builtInThemes: ThemeMeta[] = [
  {
    id: 'dia',
    label: 'Dia',
    group: 'dia',
    scheme: 'dark',
    swatch: {
      base: '#1b2636',
      baseContent: '#d6deeb',
      primary: '#7fdbca',
      secondary: '#ecc48d',
      accent: '#ff5874',
    },
  },
  {
    id: 'dia-light',
    label: 'Dia Light',
    group: 'dia',
    scheme: 'light',
    swatch: {
      base: '#fffcf0',
      baseContent: '#292524',
      primary: '#e07c1f',
      secondary: '#7c3aed',
      accent: '#db2777',
    },
  },
  {
    id: 'light',
    label: 'Light',
    group: 'light',
    scheme: 'light',
    swatch: {
      base: '#ffffff',
      baseContent: '#1f2937',
      primary: '#491eff',
      secondary: '#0165e1',
      accent: '#16a34a',
    },
  },
  {
    id: 'latte',
    label: 'Catppuccin Latte',
    group: 'catppuccin',
    scheme: 'light',
    swatch: {
      base: '#eff1f5',
      baseContent: '#4c4f69',
      primary: '#8839ef',
      secondary: '#7c7f93',
      accent: '#dc8a78',
    },
  },
  {
    id: 'frappe',
    label: 'Catppuccin Frappe',
    group: 'catppuccin',
    scheme: 'dark',
    swatch: {
      base: '#303446',
      baseContent: '#c6d0f5',
      primary: '#ca9ee6',
      secondary: '#949cbb',
      accent: '#f2d5cf',
    },
  },
  {
    id: 'macchiato',
    label: 'Catppuccin Macchiato',
    group: 'catppuccin',
    scheme: 'dark',
    swatch: {
      base: '#24273a',
      baseContent: '#cad3f5',
      primary: '#c6a0f6',
      secondary: '#8087a2',
      accent: '#f4dbd6',
    },
  },
  {
    id: 'mocha',
    label: 'Catppuccin Mocha',
    group: 'catppuccin',
    scheme: 'dark',
    swatch: {
      base: '#1e1e2e',
      baseContent: '#cdd6f4',
      primary: '#cba6f7',
      secondary: '#7f849c',
      accent: '#f5e0dc',
    },
  },
  {
    id: 'dracula',
    label: 'Dracula',
    group: 'editor',
    scheme: 'dark',
    swatch: {
      base: '#282a36',
      baseContent: '#f8f8f2',
      primary: '#bd93f9',
      secondary: '#ff79c6',
      accent: '#8be9fd',
    },
  },
  {
    id: 'github-light',
    label: 'GitHub Light',
    group: 'editor',
    scheme: 'light',
    swatch: {
      base: '#ffffff',
      baseContent: '#1f2328',
      primary: '#0969da',
      secondary: '#1f883d',
      accent: '#cf222e',
    },
  },
  {
    id: 'github-dark',
    label: 'GitHub Dark',
    group: 'editor',
    scheme: 'dark',
    swatch: {
      base: '#0d1117',
      baseContent: '#e6edf3',
      primary: '#2f81f7',
      secondary: '#3fb950',
      accent: '#f85149',
    },
  },
  {
    id: 'one-dark',
    label: 'One Dark',
    group: 'editor',
    scheme: 'dark',
    swatch: {
      base: '#282c34',
      baseContent: '#abb2bf',
      primary: '#61afef',
      secondary: '#c678dd',
      accent: '#98c379',
    },
  },
  {
    id: 'tokyo-night',
    label: 'Tokyo Night',
    group: 'editor',
    scheme: 'dark',
    swatch: {
      base: '#1a1b26',
      baseContent: '#c0caf5',
      primary: '#7aa2f7',
      secondary: '#bb9af7',
      accent: '#7dcfff',
    },
  },
  {
    id: 'tokyo-night-light',
    label: 'Tokyo Night Light',
    group: 'editor',
    scheme: 'light',
    swatch: {
      base: '#d5d6db',
      baseContent: '#1a1b26',
      primary: '#2e7de9',
      secondary: '#7847bd',
      accent: '#0db4d2',
    },
  },
  {
    id: 'nord',
    label: 'Nord',
    group: 'editor',
    scheme: 'dark',
    swatch: {
      base: '#2e3440',
      baseContent: '#eceff4',
      primary: '#88c0d0',
      secondary: '#81a1c1',
      accent: '#5e81ac',
    },
  },
  {
    id: 'solarized-dark',
    label: 'Solarized Dark',
    group: 'editor',
    scheme: 'dark',
    swatch: {
      base: '#002b36',
      baseContent: '#93a1a1',
      primary: '#268bd2',
      secondary: '#6c71c4',
      accent: '#2aa198',
    },
  },
  {
    id: 'solarized-light',
    label: 'Solarized Light',
    group: 'editor',
    scheme: 'light',
    swatch: {
      base: '#fdf6e3',
      baseContent: '#586e75',
      primary: '#268bd2',
      secondary: '#6c71c4',
      accent: '#2aa198',
    },
  },
  {
    id: 'monokai',
    label: 'Monokai',
    group: 'classic',
    scheme: 'dark',
    swatch: {
      base: '#272822',
      baseContent: '#f8f8f2',
      primary: '#66d9ef',
      secondary: '#ae81ff',
      accent: '#a6e22e',
    },
  },
  {
    id: 'gruvbox-dark',
    label: 'Gruvbox Dark',
    group: 'classic',
    scheme: 'dark',
    swatch: {
      base: '#282828',
      baseContent: '#ebdbb2',
      primary: '#83a598',
      secondary: '#b16286',
      accent: '#b8bb26',
    },
  },
  {
    id: 'gruvbox-light',
    label: 'Gruvbox Light',
    group: 'classic',
    scheme: 'light',
    swatch: {
      base: '#fbf1c7',
      baseContent: '#3c3836',
      primary: '#076678',
      secondary: '#8f3f71',
      accent: '#79740e',
    },
  },
  {
    id: 'dark',
    label: 'Dark',
    group: 'dark',
    scheme: 'dark',
    swatch: {
      base: '#1d232a',
      baseContent: '#a6adba',
      primary: '#7fdbca',
      secondary: '#ecc48d',
      accent: '#ff5874',
    },
  },
];

export const groupOrder: ThemeGroup[] = [
  'dia',
  'catppuccin',
  'editor',
  'classic',
  'light',
  'dark',
];

export const groupLabels: Record<ThemeGroup, string> = {
  dia: 'Dia',
  catppuccin: 'Catppuccin',
  editor: 'Code',
  classic: 'Classic',
  light: 'Light',
  dark: 'Dark',
};

export function findTheme(id: string): ThemeMeta | undefined {
  return builtInThemes.find((t) => t.id === id);
}
