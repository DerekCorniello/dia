import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { writable } from 'svelte/store';

vi.mock('../../wailsjs/go/wailsapp/App', () => ({
  ListWorkspaces: vi.fn(),
  GetWorkspace: vi.fn(),
  StartWorkspace: vi.fn(),
  StopWorkspace: vi.fn(),
  Reconcile: vi.fn(),
  Doctor: vi.fn(),
  Paths: vi.fn(),
  OpenConfigFolder: vi.fn(),
  OpenStateFolder: vi.fn(),
  OpenStateFile: vi.fn(),
  RevealPath: vi.fn(),
  NewWorkspace: vi.fn().mockResolvedValue(''),
  GetWorkspaceEditor: vi.fn(),
  SaveWorkspaceEditor: vi.fn(),
  DeleteWorkspace: vi.fn(),
  DetectTools: vi.fn(),
  GetTheme: vi.fn(),
  SetTheme: vi.fn(),
  ListCustomThemes: vi.fn(),
  SetCustomTheme: vi.fn(),
  DeleteCustomTheme: vi.fn(),
  ListPlugins: vi.fn(),
  EnablePlugin: vi.fn(),
  DisablePlugin: vi.fn(),
  PluginCall: vi.fn(),
  PluginPaths: vi.fn(),
  OpenPluginFolder: vi.fn(),
  SetPluginEnabled: vi.fn(),
  GetRecent: vi.fn(),
  GetKeybindings: vi.fn(),
  SetKeybinding: vi.fn(),
  ResetKeybindings: vi.fn(),
  SelectProjectDir: vi.fn(),
  GetProjectDir: vi.fn(),
  ClearProjectDir: vi.fn(),
}));

vi.mock('../stores', () => ({
  workspaces: writable([]),
  doctor: writable([]),
  paths: writable(null),
  loading: writable(false),
  lastError: writable(null),
  theme: writable('dia'),
  customThemes: writable([]),
  plugins: writable([]),
  pluginPaths: writable(null),
  keybinds: writable({}),
  projectDir: writable(''),
}));

import NewWorkspaceDialog from './NewWorkspaceDialog.svelte';
import WorkspaceCard from './WorkspaceCard.svelte';
import SettingsPanel from './SettingsPanel.svelte';

describe('NewWorkspaceDialog', () => {
  it('renders the dialog with name input', () => {
    render(NewWorkspaceDialog, { onClose: vi.fn() });
    expect(screen.getByPlaceholderText('my-project')).toBeTruthy();
    expect(screen.getByText('lowercase a-z, 0-9, internal hyphens')).toBeTruthy();
  });

  it('shows validation error for invalid name', async () => {
    const user = userEvent.setup();
    render(NewWorkspaceDialog, { onClose: vi.fn() });
    const input = screen.getByPlaceholderText('my-project');
    await user.type(input, 'Bad Name');
    await user.tab();
    expect(screen.getByText('must be lowercase a-z, 0-9, internal hyphens')).toBeTruthy();
  });

  it('clears error for valid name', async () => {
    const user = userEvent.setup();
    render(NewWorkspaceDialog, { onClose: vi.fn() });
    const input = screen.getByPlaceholderText('my-project');
    await user.type(input, 'Bad');
    expect(screen.getByText('must be lowercase a-z, 0-9, internal hyphens')).toBeTruthy();
    await user.clear(input);
    await user.type(input, 'good-name');
    expect(screen.queryByText('must be lowercase a-z, 0-9, internal hyphens')).toBeNull();
  });

  it('disables submit when name is invalid', async () => {
    const user = userEvent.setup();
    render(NewWorkspaceDialog, { onClose: vi.fn() });
    const input = screen.getByPlaceholderText('my-project');
    const submit = screen.getByText('Create');
    await user.type(input, 'Bad Name');
    expect((submit as HTMLButtonElement).disabled).toBe(true);
    await user.clear(input);
    await user.type(input, 'good');
    expect((submit as HTMLButtonElement).disabled).toBe(false);
  });
});

describe('WorkspaceCard', () => {
  it('renders workspace name and app count', () => {
    render(WorkspaceCard, {
      workspace: {
        name: 'test-project',
        apps: 3,
        source: 'global',
        running: false,
        path: '/tmp/test-project.yaml',
        plugins: [],
        useCount: 0,
        description: '',
      },
      onChanged: vi.fn(),
    });
    expect(screen.getByText('test-project')).toBeTruthy();
    expect(screen.getByText('3 apps')).toBeTruthy();
  });

  it('renders running badge when running', () => {
    render(WorkspaceCard, {
      workspace: {
        name: 'running-project',
        apps: 1,
        source: 'local',
        running: true,
        path: '/tmp/running-project.yaml',
        plugins: [],
        useCount: 0,
        description: '',
      },
      onChanged: vi.fn(),
    });
    expect(screen.getByText('running')).toBeTruthy();
    expect(screen.getByText('stop')).toBeTruthy();
  });

  it('renders start button when not running', () => {
    render(WorkspaceCard, {
      workspace: {
        name: 'idle-project',
        apps: 2,
        source: 'global',
        running: false,
        path: '/tmp/idle-project.yaml',
        plugins: [],
        useCount: 0,
        description: '',
      },
      onChanged: vi.fn(),
    });
    expect(screen.getByText('start')).toBeTruthy();
  });

  it('renders description when provided', () => {
    render(WorkspaceCard, {
      workspace: {
        name: 'desc-project',
        apps: 1,
        source: 'global',
        running: false,
        path: '/tmp/desc-project.yaml',
        plugins: [],
        useCount: 0,
        description: 'My test project',
      },
      onChanged: vi.fn(),
    });
    expect(screen.getByText('My test project')).toBeTruthy();
  });
});

describe('SettingsPanel', () => {
  it('renders tabs', () => {
    const { container } = render(SettingsPanel, {
      doctor: [],
      paths: null,
      plugins: [],
      pluginPaths: null,
    });
    expect(screen.getByText('About')).toBeTruthy();
    expect(screen.getByText('Theme')).toBeTruthy();
    expect(screen.getByText('Keybinds')).toBeTruthy();
    expect(screen.getByText('Paths')).toBeTruthy();
    expect(screen.getByText('Plugins')).toBeTruthy();
    expect(screen.getByText('Doctor')).toBeTruthy();
  });

  it('renders doctor checks', async () => {
    const user = userEvent.setup();
    render(SettingsPanel, {
      doctor: [
        { name: 'platform', status: 'ok', detail: 'linux/amd64' },
        { name: 'config dir', status: 'warn', detail: '/tmp/config' },
      ],
      paths: null,
      plugins: [],
      pluginPaths: null,
    });
    await user.click(screen.getByText('Doctor'));
    expect(screen.getByText('platform')).toBeTruthy();
    expect(screen.getByText('ok')).toBeTruthy();
    expect(screen.getByText('warn')).toBeTruthy();
  });

  it('renders paths', async () => {
    const user = userEvent.setup();
    render(SettingsPanel, {
      doctor: [],
      paths: {
        global_config_dir: '/home/user/.config/dia/workspaces',
        state_dir: '/home/user/.local/state/dia',
        state_file: '/home/user/.local/state/dia/state.json',
      },
      plugins: [],
      pluginPaths: null,
    });
    await user.click(screen.getByText('Paths'));
    expect(screen.getByText('global config')).toBeTruthy();
    expect(screen.getByText('state dir')).toBeTruthy();
    expect(screen.getByText('state file')).toBeTruthy();
  });
});
