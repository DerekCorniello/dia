// Svelte stores: workspaces, instances, and transient UI state.
// All stores are readable + writable; UI subscribes via $-syntax or
// `derived(...)`.
import { writable, type Writable } from 'svelte/store';
import type { WorkspaceInfo, InstanceInfo, CheckInfo, PathsInfo } from './api';

export const workspaces: Writable<WorkspaceInfo[]> = writable([]);
export const instances: Writable<InstanceInfo[]> = writable([]);
export const doctor: Writable<CheckInfo[]> = writable([]);
export const paths: Writable<PathsInfo | null> = writable(null);
export const loading: Writable<boolean> = writable(false);
export const lastError: Writable<string | null> = writable(null);
