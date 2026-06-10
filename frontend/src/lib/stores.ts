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
export const lastError: Writable<string | null> = writable(null);
export const theme: Writable<string> = writable('dia');
export const customThemes: Writable<CustomThemeInfo[]> = writable([]);
export const plugins: Writable<PluginInfo[]> = writable([]);
export const pluginPaths: Writable<PluginPathsInfo | null> = writable(null);
export const keybinds: Writable<Record<string, string>> = writable({});
export const projectDir: Writable<string> = writable('');
