import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

vi.mock('../../../wailsjs/go/wailsapp/App', () => ({
  PluginCall: vi.fn(),
}));

vi.mock('../stores', () => ({
  pushToast: vi.fn(),
}));

import { PluginCall } from '../../../wailsjs/go/wailsapp/App';
import PluginPanel from './PluginPanel.svelte';
import type { PluginInfo } from '../api';

// PluginUIInfo/PluginActionDef are wails-generated classes with a
// convertValues method; the plain object literals below satisfy what
// PluginPanel actually reads at runtime (plain data fields) but not
// the class's structural shape, hence the cast.
function basePlugin(overrides: Record<string, unknown> = {}): PluginInfo {
  return {
    id: 'p1',
    name: 'Test Plugin',
    version: '0.1.0',
    description: '',
    author: '',
    source: 'global',
    dir: '/tmp/p1',
    enabled: true,
    status: 'ok',
    grantedCapabilities: [],
    ui: { type: 'list', title: 'Test Plugin', refreshable: false },
    actions: [],
    ...overrides,
  } as unknown as PluginInfo;
}

beforeEach(() => {
  vi.mocked(PluginCall).mockReset();
});

describe('PluginPanel list/grid/table/kv/text', () => {
  it('renders list items from getData, including detail text', async () => {
    vi.mocked(PluginCall).mockResolvedValue(
      JSON.stringify([
        { id: 'a', label: 'Alpha' },
        { id: 'b', label: 'Bravo', detail: 'a note' },
      ]),
    );
    render(PluginPanel, {
      plugin: basePlugin({ ui: { type: 'list', title: 'L', refreshable: false } }),
    });
    await waitFor(() => {
      expect(screen.getByText('Alpha')).toBeTruthy();
      expect(screen.getByText('Bravo')).toBeTruthy();
      expect(screen.getByText('a note')).toBeTruthy();
    });
  });

  it('shows the empty state for a list with no items', async () => {
    vi.mocked(PluginCall).mockResolvedValue(JSON.stringify([]));
    render(PluginPanel, {
      plugin: basePlugin({ ui: { type: 'list', title: 'L', refreshable: false } }),
    });
    await waitFor(() => expect(screen.getByText('empty')).toBeTruthy());
  });

  it('renders grid items', async () => {
    vi.mocked(PluginCall).mockResolvedValue(JSON.stringify([{ id: 'x', label: 'Item X' }]));
    render(PluginPanel, {
      plugin: basePlugin({ ui: { type: 'grid', title: 'G', refreshable: false } }),
    });
    await waitFor(() => expect(screen.getByText('Item X')).toBeTruthy());
  });

  it('renders kv pairs', async () => {
    vi.mocked(PluginCall).mockResolvedValue(JSON.stringify({ foo: 'bar', count: 3 }));
    render(PluginPanel, {
      plugin: basePlugin({ ui: { type: 'kv', title: 'KV', refreshable: false } }),
    });
    await waitFor(() => {
      expect(screen.getByText('foo')).toBeTruthy();
      expect(screen.getByText('bar')).toBeTruthy();
    });
  });

  it('renders text as-is', async () => {
    vi.mocked(PluginCall).mockResolvedValue(JSON.stringify('hello world'));
    render(PluginPanel, {
      plugin: basePlugin({ ui: { type: 'text', title: 'T', refreshable: false } }),
    });
    await waitFor(() => expect(screen.getByText('hello world')).toBeTruthy());
  });

  it('renders table rows with duration and boolean formatting', async () => {
    vi.mocked(PluginCall).mockResolvedValue(
      JSON.stringify([{ name: 'job1', done: true, elapsed: 65000 }]),
    );
    render(PluginPanel, {
      plugin: basePlugin({
        ui: {
          type: 'table',
          title: 'Tbl',
          refreshable: false,
          columns: [
            { key: 'name', label: 'Name' },
            { key: 'done', label: 'Done' },
            { key: 'elapsed', label: 'Elapsed', format: 'duration' },
          ],
        },
      }),
    });
    await waitFor(() => {
      expect(screen.getByText('job1')).toBeTruthy();
      expect(screen.getByText('yes')).toBeTruthy();
      expect(screen.getByText('1m 5s')).toBeTruthy();
    });
  });

  it('shows the error inline when getData rejects', async () => {
    vi.mocked(PluginCall).mockRejectedValue(new Error('boom'));
    render(PluginPanel, {
      plugin: basePlugin({ ui: { type: 'list', title: 'L', refreshable: false } }),
    });
    await waitFor(() => expect(screen.getByText('boom')).toBeTruthy());
  });
});

describe('PluginPanel description', () => {
  it('says so when a plugin has no description', async () => {
    vi.mocked(PluginCall).mockResolvedValue(JSON.stringify([]));
    render(PluginPanel, { plugin: basePlugin({ description: '' }) });
    await waitFor(() => expect(screen.getByText('No description')).toBeTruthy());
  });

  it('shows the description without a duplicate expand toggle', async () => {
    vi.mocked(PluginCall).mockResolvedValue(JSON.stringify([]));
    render(PluginPanel, { plugin: basePlugin({ description: 'Does a thing' }) });
    await waitFor(() => expect(screen.getByText('Does a thing')).toBeTruthy());
    // The old toggle expanded to `longDescription || description`, so a
    // plugin without a long description rendered the same text twice
    // and grew the card inside the grid.
    expect(screen.queryByText('more details')).toBeNull();
  });
});

describe('PluginPanel canvas', () => {
  it('still calls getData on mount even though canvas does not draw the value', async () => {
    vi.mocked(PluginCall).mockResolvedValue(JSON.stringify({ color: '#111111', width: 5 }));
    const { container } = render(PluginPanel, {
      plugin: basePlugin({ ui: { type: 'canvas', title: 'Canvas', refreshable: false } }),
    });
    await waitFor(() => {
      expect(PluginCall).toHaveBeenCalledWith('p1', 'getData', '[]');
    });
    expect(container.querySelector('canvas')).toBeTruthy();
  });

  it('starts at zero strokes before any drawing', async () => {
    vi.mocked(PluginCall).mockResolvedValue(JSON.stringify({}));
    render(PluginPanel, {
      plugin: basePlugin({ ui: { type: 'canvas', title: 'Canvas', refreshable: false } }),
    });
    await waitFor(() => expect(screen.getByText('0 strokes')).toBeTruthy());
  });

  it('sends the current strokes to onAction and applies a returned replacement', async () => {
    vi.mocked(PluginCall).mockImplementation(async (_id, method) => {
      if (method === 'getData') return JSON.stringify({});
      if (method === 'onAction') return JSON.stringify({ strokes: [] });
      return '';
    });
    render(PluginPanel, {
      plugin: basePlugin({
        ui: { type: 'canvas', title: 'Canvas', refreshable: false },
        actions: [{ id: 'undo', label: 'Undo', confirm: false, capability: '' }],
      }),
    });
    await waitFor(() => expect(PluginCall).toHaveBeenCalledWith('p1', 'getData', '[]'));

    const undo = await screen.findByText('Undo');
    await fireEvent.click(undo);

    await waitFor(() => {
      expect(PluginCall).toHaveBeenCalledWith(
        'p1',
        'onAction',
        JSON.stringify(['undo', { strokes: [] }]),
      );
    });
  });
});
