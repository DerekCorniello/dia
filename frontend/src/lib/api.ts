// Typed wrappers around the wails-generated bindings under
// wailsjs/go/wailsapp/App. The generated module is regenerated on
// every `wails build` / `wails dev`; this file is the stable
// surface the UI components import from.
import {
  ListWorkspaces as _ListWorkspaces,
  GetWorkspace as _GetWorkspace,
  StartWorkspace as _StartWorkspace,
  StopInstance as _StopInstance,
  StopAll as _StopAll,
  ListInstances as _ListInstances,
  Reconcile as _Reconcile,
  Doctor as _Doctor,
  Paths as _Paths,
  OpenConfigFolder as _OpenConfigFolder,
  OpenStateFolder as _OpenStateFolder,
  NewWorkspace as _NewWorkspace,
  GetTheme as _GetTheme,
  SetTheme as _SetTheme,
  ListCustomThemes as _ListCustomThemes,
  SetCustomTheme as _SetCustomTheme,
  DeleteCustomTheme as _DeleteCustomTheme,
} from '../../wailsjs/go/wailsapp/App';
import { wailsapp } from '../../wailsjs/go/models';

export type WorkspaceInfo = wailsapp.WorkspaceInfo;
export type WorkspaceDetail = wailsapp.WorkspaceDetail;
export type InstanceInfo = wailsapp.InstanceInfo;
export type ReconcileInfo = wailsapp.ReconcileInfo;
export type CheckInfo = wailsapp.CheckInfo;
export type PathsInfo = wailsapp.PathsInfo;
export type CustomThemeInfo = wailsapp.CustomThemeInfo;

const array = <T>(p: Promise<T[]>): Promise<T[]> => p.then((v) => v ?? []);

export const api = {
  listWorkspaces: (): Promise<WorkspaceInfo[]> => array(_ListWorkspaces()),
  getWorkspace: (name: string): Promise<WorkspaceDetail> => _GetWorkspace(name),
  startWorkspace: (name: string): Promise<InstanceInfo> => _StartWorkspace(name),
  stopInstance: (id: string): Promise<void> => _StopInstance(id),
  stopAll: (): Promise<number> => _StopAll(),
  listInstances: (): Promise<InstanceInfo[]> => array(_ListInstances()),
  reconcile: (): Promise<ReconcileInfo> => _Reconcile(),
  doctor: (): Promise<CheckInfo[]> => array(_Doctor()),
  paths: (): Promise<PathsInfo> => _Paths(),
  openConfigFolder: (): Promise<void> => _OpenConfigFolder(),
  openStateFolder: (): Promise<void> => _OpenStateFolder(),
  newWorkspace: (name: string): Promise<string> => _NewWorkspace(name),
  getTheme: (): Promise<string> => _GetTheme(),
  setTheme: (theme: string): Promise<void> => _SetTheme(theme),
  listCustomThemes: (): Promise<CustomThemeInfo[]> => array(_ListCustomThemes()),
  setCustomTheme: (info: CustomThemeInfo): Promise<void> => _SetCustomTheme(info),
  deleteCustomTheme: (name: string): Promise<void> => _DeleteCustomTheme(name),
};

export function describeError(err: unknown): string {
  if (err == null) return 'unknown error';
  if (typeof err === 'string') return err;
  if (err instanceof Error) return err.message;
  if (typeof err === 'object') {
    const e = err as { message?: unknown };
    if (typeof e.message === 'string') return e.message;
  }
  try {
    return JSON.stringify(err);
  } catch {
    return String(err);
  }
}
