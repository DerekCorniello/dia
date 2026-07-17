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
const TOAST_MAX = 4;

// pushToast returns the id of the toast carrying this message, which is
// an existing one when the same message is already on screen.
export function pushToast(kind: ToastKind, text: string): number {
  let id = -1;
  let isNew = false;
  toasts.update((list) => {
    // Errors persist, and a running workspace rewrites the state file
    // every couple of seconds, which refreshes the UI. A refresh that
    // keeps failing would otherwise pile up an identical toast per
    // attempt until they buried the app, so repeat messages reuse the
    // toast that is already showing.
    const existing = list.find((t) => t.kind === kind && t.text === text);
    if (existing) {
      id = existing.id;
      return list;
    }
    id = nextToastId++;
    isNew = true;
    const next = [...list, { id, kind, text }];
    // Backstop for distinct messages arriving faster than they are read.
    return next.length > TOAST_MAX ? next.slice(next.length - TOAST_MAX) : next;
  });
  if (isNew && kind === 'ok') {
    setTimeout(() => dismissToast(id), TOAST_DISMISS_MS);
  }
  return id;
}

export function dismissToast(id: number): void {
  toasts.update((list) => list.filter((t) => t.id !== id));
}
