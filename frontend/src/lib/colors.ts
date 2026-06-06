// Hex <-> OKLCH conversion and daisyUI v4 theme CSS generation.
//
// daisyUI v4 stores colors as OKLCH CSS variables in the format
// `lightness% chroma hue` (e.g. `49.12% 0.3096 275.75`). The
// Tailwind classes use `oklch(var(--p)/<alpha-value>)` so the
// variable must be in OKLCH for the alpha channel to work.
//
// Custom themes are stored in state as hex strings. At apply time
// the frontend converts them and injects a [data-theme="..."] CSS
// block. The same conversion backs the live preview swatches in
// the picker.

export interface Oklch {
  l: number;
  c: number;
  h: number;
}

export function hexToOklch(hex: string): Oklch {
  const m = hex.replace(/^#/, '');
  if (m.length !== 6) return { l: 0, c: 0, h: 0 };
  const r = parseInt(m.slice(0, 2), 16) / 255;
  const g = parseInt(m.slice(2, 4), 16) / 255;
  const b = parseInt(m.slice(4, 6), 16) / 255;

  const lin = (c: number) =>
    c <= 0.04045 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
  const lr = lin(r);
  const lg = lin(g);
  const lb = lin(b);

  const lp = 0.4122214708 * lr + 0.5363325363 * lg + 0.0514459929 * lb;
  const mp = 0.2119034982 * lr + 0.6806995451 * lg + 0.1073969566 * lb;
  const sp = 0.0883024619 * lr + 0.2817188376 * lg + 0.6299787005 * lb;

  const lpc = Math.cbrt(lp);
  const mpc = Math.cbrt(mp);
  const spc = Math.cbrt(sp);

  const L = 0.2104542553 * lpc + 0.793617785 * mpc - 0.0040720468 * spc;
  const a = 1.9779984951 * lpc - 2.428592205 * mpc + 0.4505937099 * spc;
  const bb = 0.0259040371 * lpc + 0.7827717662 * mpc - 0.808675766 * spc;

  const C = Math.sqrt(a * a + bb * bb);
  let H = (Math.atan2(bb, a) * 180) / Math.PI;
  if (H < 0) H += 360;
  if (Number.isNaN(H)) H = 0;

  return { l: L, c: C, h: H };
}

export function formatOklch(o: Oklch): string {
  const l = (o.l * 100).toFixed(2);
  const c = o.c.toFixed(4);
  const h = o.h.toFixed(2);
  return `${l}% ${c} ${h}`;
}

// daisyUI v4 maps object-theme keys (primary, base-100, etc.) onto
// CSS variables (--p, --b1, etc.). The picker uses snake_case in
// state; this converts to the CSS var name and tracks the default
// fallback color used for "auto" picker (daisyUI uses the same).
const slotToVar: Record<string, { var: string; defaultHex: string; base: boolean }> = {
  primary: { var: '--p', defaultHex: '#491eff', base: false },
  primary_content: { var: '--pc', defaultHex: '#ffffff', base: false },
  secondary: { var: '--s', defaultHex: '#0165e1', base: false },
  secondary_content: { var: '--sc', defaultHex: '#ffffff', base: false },
  accent: { var: '--a', defaultHex: '#00d3a7', base: false },
  accent_content: { var: '--ac', defaultHex: '#000000', base: false },
  neutral: { var: '--n', defaultHex: '#1f2937', base: false },
  neutral_content: { var: '--nc', defaultHex: '#ffffff', base: false },
  base_100: { var: '--b1', defaultHex: '#ffffff', base: true },
  base_200: { var: '--b2', defaultHex: '#f3f4f6', base: true },
  base_300: { var: '--b3', defaultHex: '#e5e7eb', base: true },
  base_content: { var: '--bc', defaultHex: '#1f2937', base: true },
  info: { var: '--in', defaultHex: '#00b5ff', base: false },
  success: { var: '--su', defaultHex: '#22c55e', base: false },
  warning: { var: '--wa', defaultHex: '#ffb700', base: false },
  error: { var: '--er', defaultHex: '#ff5874', base: false },
};

export const editableSlots: { key: string; label: string; group: 'brand' | 'base' | 'state' }[] = [
  { key: 'primary', label: 'Primary', group: 'brand' },
  { key: 'primary_content', label: 'Primary Content', group: 'brand' },
  { key: 'secondary', label: 'Secondary', group: 'brand' },
  { key: 'secondary_content', label: 'Secondary Content', group: 'brand' },
  { key: 'accent', label: 'Accent', group: 'brand' },
  { key: 'accent_content', label: 'Accent Content', group: 'brand' },
  { key: 'neutral', label: 'Neutral', group: 'brand' },
  { key: 'neutral_content', label: 'Neutral Content', group: 'brand' },
  { key: 'base_100', label: 'Base 100', group: 'base' },
  { key: 'base_200', label: 'Base 200', group: 'base' },
  { key: 'base_300', label: 'Base 300', group: 'base' },
  { key: 'base_content', label: 'Base Content', group: 'base' },
  { key: 'info', label: 'Info', group: 'state' },
  { key: 'success', label: 'Success', group: 'state' },
  { key: 'warning', label: 'Warning', group: 'state' },
  { key: 'error', label: 'Error', group: 'state' },
];

export interface CustomThemePayload {
  name: string;
  colorScheme: 'light' | 'dark';
  colors: Record<string, string>;
}

export function buildCustomThemeCss(theme: CustomThemePayload): string {
  const lines: string[] = [`[data-theme="${theme.name}"] {`];
  lines.push(`  color-scheme: ${theme.colorScheme};`);
  for (const slot of editableSlots) {
    const mapping = slotToVar[slot.key];
    if (!mapping) continue;
    const hex = theme.colors[slot.key] || mapping.defaultHex;
    const oklch = hexToOklch(hex);
    lines.push(`  ${mapping.var}: ${formatOklch(oklch)};`);
  }
  lines.push('}');
  return lines.join('\n');
}

export function buildAllCustomThemesCss(themes: CustomThemePayload[]): string {
  return themes.map(buildCustomThemeCss).join('\n');
}
