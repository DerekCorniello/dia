// Typed wrappers around the wails-generated bindings under
// wailsjs/go/wailsapp/App. The generated module is regenerated on
// every `wails build` / `wails dev`; this file is the stable
// surface the UI components import from.
import {
  ListWorkspaces as _ListWorkspaces,
  GetWorkspace as _GetWorkspace,
  StartWorkspace as _StartWorkspace,
  StopWorkspace as _StopWorkspace,
  Reconcile as _Reconcile,
  Doctor as _Doctor,
  Paths as _Paths,
  OpenConfigFolder as _OpenConfigFolder,
  OpenStateFolder as _OpenStateFolder,
  OpenStateFile as _OpenStateFile,
  RevealPath as _RevealPath,
  NewWorkspace as _NewWorkspace,
  GetWorkspaceEditor as _GetWorkspaceEditor,
  SaveWorkspaceEditor as _SaveWorkspaceEditor,
  DeleteWorkspace as _DeleteWorkspace,
  DetectTools as _DetectTools,
  GetTheme as _GetTheme,
  SetTheme as _SetTheme,
  ListCustomThemes as _ListCustomThemes,
  SetCustomTheme as _SetCustomTheme,
  DeleteCustomTheme as _DeleteCustomTheme,
  ListPlugins as _ListPlugins,
  GetPluginCapabilities as _GetPluginCapabilities,
  InspectPluginSource as _InspectPluginSource,
  InstallPluginFromSource as _InstallPluginFromSource,
  PluginIsUpdatable as _PluginIsUpdatable,
  UpdatePlugin as _UpdatePlugin,
  UninstallPlugin as _UninstallPlugin,
  SetPluginCapabilities as _SetPluginCapabilities,
  PluginCall as _PluginCall,
  PluginPaths as _PluginPaths,
  OpenPluginFolder as _OpenPluginFolder,
  InstallPluginFromFolder as _InstallPluginFromFolder,
  GetRecent as _GetRecent,
  GetKeybindings as _GetKeybindings,
  SetKeybinding as _SetKeybinding,
  ResetKeybindings as _ResetKeybindings,
  AddRoot as _AddRoot,
  RemoveRoot as _RemoveRoot,
  ListRoots as _ListRoots,
} from '../../wailsjs/go/wailsapp/App';
import { wailsapp } from '../../wailsjs/go/models';

export type WorkspaceInfo = wailsapp.WorkspaceInfo;
export type WorkspaceDetail = wailsapp.WorkspaceDetail;
export type ReconcileInfo = wailsapp.ReconcileInfo;
export type CheckInfo = wailsapp.CheckInfo;
export type PathsInfo = wailsapp.PathsInfo;
export type CustomThemeInfo = wailsapp.CustomThemeInfo;
export type PluginInfo = wailsapp.PluginInfo;
export type PluginCapabilityInfo = wailsapp.PluginCapabilityInfo;
export type PluginSourceInfo = wailsapp.PluginSourceInfo;
export type CapabilityInfo = wailsapp.CapabilityInfo;
export type PluginUIInfo = wailsapp.PluginUIInfo;
export type PluginUIColumn = wailsapp.PluginUIColumn;
export type PluginActionDef = wailsapp.PluginActionDef;
export type PluginPathsInfo = wailsapp.PluginPathsInfo;

export type AppEditor = {
  label: string;
  cmd: string;
  cwd: string;
  url: string;
  termCmd: string;
  _cat?: string;
  _showUrl?: boolean;
};

export type PluginRefEditor = {
  _key?: string;
  id: string;
  config: Record<string, any>;
};

export type WorkspaceEditor = {
  originalName: string;
  originalPath: string;
  name: string;
  description: string;
  defaultCwd: string;
  apps: AppEditor[];
  plugins: PluginRefEditor[];
};

export type RecentEntry = {
  name: string;
  count: number;
};

export type ToolCategory = {
  name: string;
  tools: DetectedTool[];
};

export type DetectedTool = {
  label: string;
  command: string;
  url: string;
};

const array = <T>(p: Promise<T[]>): Promise<T[]> => p.then((v) => v ?? []);

export const api = {
  listWorkspaces: (): Promise<WorkspaceInfo[]> => array(_ListWorkspaces()),
  getWorkspace: (name: string): Promise<WorkspaceDetail> => _GetWorkspace(name),
  startWorkspace: (name: string): Promise<void> => _StartWorkspace(name),
  stopWorkspace: (name: string): Promise<void> => _StopWorkspace(name),
  reconcile: (): Promise<ReconcileInfo> => _Reconcile(),
  doctor: (): Promise<CheckInfo[]> => array(_Doctor()),
  paths: (): Promise<PathsInfo> => _Paths(),
  openConfigFolder: (): Promise<void> => _OpenConfigFolder(),
  openStateFolder: (): Promise<void> => _OpenStateFolder(),
  openStateFile: (): Promise<void> => _OpenStateFile(),
  newWorkspace: (name: string, local: boolean): Promise<string> => _NewWorkspace(name, local),
  getTheme: (): Promise<string> => _GetTheme(),
  setTheme: (theme: string): Promise<void> => _SetTheme(theme),
  listCustomThemes: (): Promise<CustomThemeInfo[]> => array(_ListCustomThemes()),
  setCustomTheme: (info: CustomThemeInfo): Promise<void> => _SetCustomTheme(info),
  deleteCustomTheme: (name: string): Promise<void> => _DeleteCustomTheme(name),
  listPlugins: (): Promise<PluginInfo[]> => array(_ListPlugins()),
  pluginCall: (id: string, method: string, argsJSON: string): Promise<string> =>
    _PluginCall(id, method, argsJSON),
  getPluginCapabilities: (id: string): Promise<PluginCapabilityInfo> => _GetPluginCapabilities(id),
  setPluginCapabilities: (id: string, caps: string[]): Promise<void> =>
    _SetPluginCapabilities(id, caps),
  inspectPluginSource: (source: string, ref: string): Promise<PluginSourceInfo> =>
    _InspectPluginSource(source, ref),
  installPluginFromSource: (source: string, ref: string): Promise<string> =>
    _InstallPluginFromSource(source, ref),
  pluginIsUpdatable: (id: string): Promise<boolean> => _PluginIsUpdatable(id),
  updatePlugin: (id: string): Promise<string> => _UpdatePlugin(id),
  uninstallPlugin: (id: string): Promise<void> => _UninstallPlugin(id),
  pluginPaths: (): Promise<PluginPathsInfo> => _PluginPaths(),
  openPluginFolder: (): Promise<void> => _OpenPluginFolder(),
  installPluginFromFolder: (): Promise<string> => _InstallPluginFromFolder(),
  revealPath: (path: string): Promise<void> => _RevealPath(path),
  getWorkspaceEditor: (name: string): Promise<WorkspaceEditor> =>
    _GetWorkspaceEditor(name) as Promise<WorkspaceEditor>,
  saveWorkspaceEditor: (editor: WorkspaceEditor): Promise<void> =>
    _SaveWorkspaceEditor(editor as any),
  deleteWorkspace: (name: string): Promise<void> => _DeleteWorkspace(name),
  detectTools: (): Promise<ToolCategory[]> => array(_DetectTools()),
  getRecent: (): Promise<RecentEntry[]> => array(_GetRecent()) as Promise<RecentEntry[]>,
  getKeybindings: (): Promise<Record<string, string>> => _GetKeybindings(),
  setKeybinding: (action: string, keys: string): Promise<void> => _SetKeybinding(action, keys),
  resetKeybindings: (): Promise<void> => _ResetKeybindings(),
  addRoot: (dir: string): Promise<void> => _AddRoot(dir),
  removeRoot: (dir: string): Promise<void> => _RemoveRoot(dir),
  listRoots: (): Promise<string[]> => _ListRoots(),
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
