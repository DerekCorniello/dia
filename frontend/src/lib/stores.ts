// Svelte stores: workspaces, instances, and transient UI state.
// All stores are readable + writable; UI subscribes via $-syntax or
// `derived(...)`.
import { writable, type Writable } from 'svelte/store';
import type {
  WorkspaceInfo,
  CheckInfo,
  PathsInfo,
  CustomThemeInfo,
  PluginInfo,
  PluginPathsInfo,
} from './api';

export const workspaces: Writable<WorkspaceInfo[]> = writable([]);
export const doctor: Writable<CheckInfo[]> = writable([]);
export const paths: Writable<PathsInfo | null> = writable(null);
export const loading: Writable<boolean> = writable(false);
export const theme: Writable<string> = writable('dia');
export const customThemes: Writable<CustomThemeInfo[]> = writable([]);
export const plugins: Writable<PluginInfo[]> = writable([]);
export const pluginPaths: Writable<PluginPathsInfo | null> = writable(null);
export const keybinds: Writable<Record<string, string>> = writable({});
export const projectDir: Writable<string> = writable('');

// Unified toast feedback. Success/info toasts auto-dismiss; error
// toasts persist until dismissed so a failure is never missed.
export type ToastKind = 'ok' | 'err';
export type Toast = { id: number; kind: ToastKind; text: string };
export const toasts: Writable<Toast[]> = writable([]);

let nextToastId = 1;
const TOAST_DISMISS_MS = 3000;

export function pushToast(kind: ToastKind, text: string): number {
  const id = nextToastId++;
  toasts.update((list) => [...list, { id, kind, text }]);
  if (kind === 'ok') {
    setTimeout(() => dismissToast(id), TOAST_DISMISS_MS);
  }
  return id;
}

export function dismissToast(id: number): void {
  toasts.update((list) => list.filter((t) => t.id !== id));
}
