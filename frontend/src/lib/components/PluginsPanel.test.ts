import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';

vi.mock('../api', () => ({
  api: {
    getPluginCapabilities: vi.fn(),
    setPluginCapabilities: vi.fn(),
    installPluginFromFolder: vi.fn(),
  },
  describeError: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));

vi.mock('../stores', () => ({
  pushToast: vi.fn(),
}));

import { api } from '../api';
import PluginsPanel from './PluginsPanel.svelte';
import type { PluginInfo, PluginCapabilityInfo } from '../api';

// PluginCapabilityInfo is a wails-generated class with a convertValues
// method; these literals carry everything the component actually reads
// but not the class's structural shape, hence the cast.
function capsInfo(
  requested: { name: string; mutating: boolean; granted: boolean }[],
  granted: string[],
): PluginCapabilityInfo {
  return { requested, granted } as unknown as PluginCapabilityInfo;
}

function plugin(overrides: Record<string, unknown> = {}): PluginInfo {
  return {
    id: 'caps-plug',
    name: 'Caps Plugin',
    version: '0.1.0',
    description: '',
    author: '',
    source: 'global',
    dir: '/tmp/caps-plug',
    enabled: true,
    status: 'active',
    grantedCapabilities: ['workspaces:read'],
    ui: { type: 'kv', title: 'T', refreshable: false },
    actions: [],
    ...overrides,
  } as unknown as PluginInfo;
}

// The capability list lives inside a <details>. jsdom does not fire
// `toggle` from a summary click on its own, so open it the way the
// component listens for: set `open` and dispatch the event.
async function openCapabilities(container: HTMLElement) {
  const details = container.querySelector('details') as HTMLDetailsElement;
  details.open = true;
  await fireEvent(details, new Event('toggle'));
  return details;
}

const readAndMutating = capsInfo(
  [
    { name: 'workspaces:read', mutating: false, granted: true },
    { name: 'cmd:exec', mutating: true, granted: false },
    { name: 'apps:resolve', mutating: true, granted: false },
  ],
  ['workspaces:read'],
);

beforeEach(() => {
  vi.mocked(api.getPluginCapabilities).mockReset();
  vi.mocked(api.setPluginCapabilities).mockReset();
  vi.mocked(api.installPluginFromFolder).mockReset();
});

describe('PluginsPanel rendering', () => {
  it('shows the empty state with no plugins', () => {
    render(PluginsPanel, { plugins: [] });
    expect(screen.getByText('no plugins installed')).toBeTruthy();
  });

  it('renders a plugin with its name, version and source', () => {
    render(PluginsPanel, { plugins: [plugin()] });
    expect(screen.getByText('Caps Plugin')).toBeTruthy();
    expect(screen.getByText('v0.1.0')).toBeTruthy();
    expect(screen.getByText('global')).toBeTruthy();
    expect(screen.getByText('active')).toBeTruthy();
  });
});

describe('PluginsPanel capabilities', () => {
  it('does not fetch capabilities until the section is opened', () => {
    render(PluginsPanel, { plugins: [plugin()] });
    expect(api.getPluginCapabilities).not.toHaveBeenCalled();
  });

  it('loads and lists capabilities when opened, flagging mutating ones', async () => {
    vi.mocked(api.getPluginCapabilities).mockResolvedValue(readAndMutating);
    const { container } = render(PluginsPanel, { plugins: [plugin()] });

    await openCapabilities(container);

    await waitFor(() => {
      expect(screen.getByText('workspaces:read')).toBeTruthy();
      expect(screen.getByText('cmd:exec')).toBeTruthy();
      expect(screen.getByText('apps:resolve')).toBeTruthy();
    });
    expect(api.getPluginCapabilities).toHaveBeenCalledWith('caps-plug');
    // The read-only capability is not flagged; the two mutating ones are.
    expect(screen.getAllByText('mutating')).toHaveLength(2);
  });

  it('checks the boxes that are already granted', async () => {
    vi.mocked(api.getPluginCapabilities).mockResolvedValue(readAndMutating);
    const { container } = render(PluginsPanel, { plugins: [plugin()] });
    await openCapabilities(container);

    await waitFor(() => expect(screen.getByText('cmd:exec')).toBeTruthy());
    const granted = container.querySelector('#caps-plug-workspaces\\:read') as HTMLInputElement;
    const ungranted = container.querySelector('#caps-plug-cmd\\:exec') as HTMLInputElement;
    expect(granted.checked).toBe(true);
    expect(ungranted.checked).toBe(false);
  });

  it('shows a message when the plugin requests nothing', async () => {
    vi.mocked(api.getPluginCapabilities).mockResolvedValue(capsInfo([], []));
    const { container } = render(PluginsPanel, { plugins: [plugin()] });
    await openCapabilities(container);

    await waitFor(() =>
      expect(screen.getByText('this plugin requests no capabilities')).toBeTruthy(),
    );
  });

  it('surfaces a load failure instead of rendering a broken list', async () => {
    vi.mocked(api.getPluginCapabilities).mockRejectedValue(new Error('plugin not found'));
    const { container } = render(PluginsPanel, { plugins: [plugin()] });
    await openCapabilities(container);

    await waitFor(() => expect(screen.getByText('plugin not found')).toBeTruthy());
  });

  it('only fetches once even if the section is opened again', async () => {
    vi.mocked(api.getPluginCapabilities).mockResolvedValue(readAndMutating);
    const { container } = render(PluginsPanel, { plugins: [plugin()] });

    const details = await openCapabilities(container);
    await waitFor(() => expect(screen.getByText('cmd:exec')).toBeTruthy());
    details.open = false;
    await fireEvent(details, new Event('toggle'));
    details.open = true;
    await fireEvent(details, new Event('toggle'));

    expect(api.getPluginCapabilities).toHaveBeenCalledTimes(1);
  });
});

describe('PluginsPanel granting', () => {
  // Save must stay inert until something actually changed, or the user
  // cannot tell whether their edit registered.
  it('disables save until a capability is toggled', async () => {
    vi.mocked(api.getPluginCapabilities).mockResolvedValue(readAndMutating);
    const { container } = render(PluginsPanel, { plugins: [plugin()] });
    await openCapabilities(container);
    await waitFor(() => expect(screen.getByText('cmd:exec')).toBeTruthy());

    const save = screen.getByText('Save capabilities') as HTMLButtonElement;
    expect(save.disabled).toBe(true);

    await userEvent.click(container.querySelector('#caps-plug-cmd\\:exec') as HTMLInputElement);
    expect((screen.getByText('Save capabilities') as HTMLButtonElement).disabled).toBe(false);
  });

  it('sends the granted set, including the previously granted ones', async () => {
    vi.mocked(api.getPluginCapabilities).mockResolvedValue(readAndMutating);
    vi.mocked(api.setPluginCapabilities).mockResolvedValue(undefined);
    const { container } = render(PluginsPanel, { plugins: [plugin()] });
    await openCapabilities(container);
    await waitFor(() => expect(screen.getByText('apps:resolve')).toBeTruthy());

    await userEvent.click(container.querySelector('#caps-plug-apps\\:resolve') as HTMLInputElement);
    await userEvent.click(screen.getByText('Save capabilities'));

    await waitFor(() => {
      expect(api.setPluginCapabilities).toHaveBeenCalledWith('caps-plug', [
        'workspaces:read',
        'apps:resolve',
      ]);
    });
  });

  // Revoking is the half that used to silently keep working, so it
  // gets its own case.
  it('sends a shortened set when a granted capability is unchecked', async () => {
    vi.mocked(api.getPluginCapabilities).mockResolvedValue(
      capsInfo(
        [
          { name: 'workspaces:read', mutating: false, granted: true },
          { name: 'apps:resolve', mutating: true, granted: true },
        ],
        ['workspaces:read', 'apps:resolve'],
      ),
    );
    vi.mocked(api.setPluginCapabilities).mockResolvedValue(undefined);
    const { container } = render(PluginsPanel, { plugins: [plugin()] });
    await openCapabilities(container);
    await waitFor(() => expect(screen.getByText('apps:resolve')).toBeTruthy());

    await userEvent.click(container.querySelector('#caps-plug-apps\\:resolve') as HTMLInputElement);
    await userEvent.click(screen.getByText('Save capabilities'));

    await waitFor(() => {
      expect(api.setPluginCapabilities).toHaveBeenCalledWith('caps-plug', ['workspaces:read']);
    });
  });

  it('toggling twice returns to the original set and re-disables save', async () => {
    vi.mocked(api.getPluginCapabilities).mockResolvedValue(readAndMutating);
    const { container } = render(PluginsPanel, { plugins: [plugin()] });
    await openCapabilities(container);
    await waitFor(() => expect(screen.getByText('cmd:exec')).toBeTruthy());

    const box = container.querySelector('#caps-plug-cmd\\:exec') as HTMLInputElement;
    await userEvent.click(box);
    await userEvent.click(box);

    expect((screen.getByText('Save capabilities') as HTMLButtonElement).disabled).toBe(true);
  });

  it('asks the parent to refresh after a successful save', async () => {
    vi.mocked(api.getPluginCapabilities).mockResolvedValue(readAndMutating);
    vi.mocked(api.setPluginCapabilities).mockResolvedValue(undefined);
    const onRefresh = vi.fn();
    const { container } = render(PluginsPanel, {
      props: { plugins: [plugin()] },
      events: { refresh: onRefresh },
    });

    await openCapabilities(container);
    await waitFor(() => expect(screen.getByText('cmd:exec')).toBeTruthy());
    await userEvent.click(container.querySelector('#caps-plug-cmd\\:exec') as HTMLInputElement);
    await userEvent.click(screen.getByText('Save capabilities'));

    await waitFor(() => expect(onRefresh).toHaveBeenCalled());
  });

  it('shows the error and does not refresh when the save is rejected', async () => {
    vi.mocked(api.getPluginCapabilities).mockResolvedValue(readAndMutating);
    vi.mocked(api.setPluginCapabilities).mockRejectedValue(new Error('apply failed'));
    const onRefresh = vi.fn();
    const { container } = render(PluginsPanel, {
      props: { plugins: [plugin()] },
      events: { refresh: onRefresh },
    });

    await openCapabilities(container);
    await waitFor(() => expect(screen.getByText('cmd:exec')).toBeTruthy());
    await userEvent.click(container.querySelector('#caps-plug-cmd\\:exec') as HTMLInputElement);
    await userEvent.click(screen.getByText('Save capabilities'));

    await waitFor(() => expect(screen.getByText('apply failed')).toBeTruthy());
    expect(onRefresh).not.toHaveBeenCalled();
  });

  // Two plugins share one component; edits to one must not leak.
  it('keeps pending edits separate per plugin', async () => {
    vi.mocked(api.getPluginCapabilities).mockImplementation(async (id: string) => {
      if (id === 'caps-plug') return readAndMutating;
      return capsInfo([{ name: 'cmd:exec', mutating: true, granted: false }], []);
    });
    vi.mocked(api.setPluginCapabilities).mockResolvedValue(undefined);

    const { container } = render(PluginsPanel, {
      plugins: [plugin(), plugin({ id: 'other-plug', name: 'Other' })],
    });

    const all = Array.from(container.querySelectorAll('details')) as HTMLDetailsElement[];
    for (const d of all) {
      d.open = true;
      await fireEvent(d, new Event('toggle'));
    }
    await waitFor(() => expect(container.querySelector('#other-plug-cmd\\:exec')).toBeTruthy());

    // Edit only the second plugin.
    await userEvent.click(container.querySelector('#other-plug-cmd\\:exec') as HTMLInputElement);
    const saves = screen.getAllByText('Save capabilities') as HTMLButtonElement[];
    expect(saves[0].disabled).toBe(true);
    expect(saves[1].disabled).toBe(false);

    await userEvent.click(saves[1]);
    await waitFor(() => {
      expect(api.setPluginCapabilities).toHaveBeenCalledWith('other-plug', ['cmd:exec']);
    });
  });
});

describe('PluginsPanel toolbar', () => {
  it('refreshes after installing from a folder', async () => {
    vi.mocked(api.installPluginFromFolder).mockResolvedValue('/tmp/new-plug');
    const onRefresh = vi.fn();
    render(PluginsPanel, { props: { plugins: [] }, events: { refresh: onRefresh } });

    await userEvent.click(screen.getByText('Install from folder'));
    await waitFor(() => expect(onRefresh).toHaveBeenCalled());
  });

  it('shows an install failure', async () => {
    vi.mocked(api.installPluginFromFolder).mockRejectedValue(new Error('not a plugin'));
    render(PluginsPanel, { plugins: [] });

    await userEvent.click(screen.getByText('Install from folder'));
    await waitFor(() => expect(screen.getByText('not a plugin')).toBeTruthy());
  });

  it('refreshes on demand', async () => {
    const onRefresh = vi.fn();
    render(PluginsPanel, { props: { plugins: [] }, events: { refresh: onRefresh } });

    await userEvent.click(screen.getByText('Refresh'));
    expect(onRefresh).toHaveBeenCalled();
  });
});
