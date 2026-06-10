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
  EnablePlugin as _EnablePlugin,
  DisablePlugin as _DisablePlugin,
  PluginCall as _PluginCall,
  PluginPaths as _PluginPaths,
  OpenPluginFolder as _OpenPluginFolder,
  SetPluginEnabled as _SetPluginEnabled,
  GetRecent as _GetRecent,
  GetKeybindings as _GetKeybindings,
  SetKeybinding as _SetKeybinding,
  ResetKeybindings as _ResetKeybindings,
  SelectProjectDir as _SelectProjectDir,
  GetProjectDir as _GetProjectDir,
  ClearProjectDir as _ClearProjectDir,
} from '../../wailsjs/go/wailsapp/App';
import { wailsapp } from '../../wailsjs/go/models';

export type WorkspaceInfo = wailsapp.WorkspaceInfo;
export type WorkspaceDetail = wailsapp.WorkspaceDetail;
export type ReconcileInfo = wailsapp.ReconcileInfo;
export type CheckInfo = wailsapp.CheckInfo;
export type PathsInfo = wailsapp.PathsInfo;
export type CustomThemeInfo = wailsapp.CustomThemeInfo;
export type PluginInfo = wailsapp.PluginInfo;
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
  name: string;
  description: string;
  defaultCwd: string;
  apps: AppEditor[];
  plugins: PluginRefEditor[];
  convertValues?: (a: any, classs: any, asMap?: boolean) => any;
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
  enablePlugin: (id: string): Promise<void> => _EnablePlugin(id),
  disablePlugin: (id: string): Promise<void> => _DisablePlugin(id),
  pluginCall: (id: string, method: string, argsJSON: string): Promise<string> =>
    _PluginCall(id, method, argsJSON),
  pluginPaths: (): Promise<PluginPathsInfo> => _PluginPaths(),
  openPluginFolder: (): Promise<void> => _OpenPluginFolder(),
  revealPath: (path: string): Promise<void> => _RevealPath(path),
  getWorkspaceEditor: (name: string): Promise<WorkspaceEditor> => _GetWorkspaceEditor(name) as Promise<WorkspaceEditor>,
  saveWorkspaceEditor: (editor: WorkspaceEditor): Promise<void> => _SaveWorkspaceEditor(editor as any),
  deleteWorkspace: (name: string): Promise<void> => _DeleteWorkspace(name),
  detectTools: (): Promise<ToolCategory[]> => array(_DetectTools()),
  getRecent: (): Promise<RecentEntry[]> => array(_GetRecent()) as Promise<RecentEntry[]>,
  getKeybindings: (): Promise<Record<string, string>> => _GetKeybindings(),
  setKeybinding: (action: string, keys: string): Promise<void> => _SetKeybinding(action, keys),
  resetKeybindings: (): Promise<void> => _ResetKeybindings(),
  setPluginEnabled: (id: string, enabled: boolean, granted: string[]): Promise<void> =>
    _SetPluginEnabled(id, enabled, granted),
  selectProjectDir: (): Promise<string> => _SelectProjectDir(),
  getProjectDir: (): Promise<string> => _GetProjectDir(),
  clearProjectDir: (): Promise<void> => _ClearProjectDir(),
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
