package runtime

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/DerekCorniello/dia/internal/browser"
	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/state"
)

// fakeSurface records Open/Close calls so the runtime wiring can be
// tested without cloning a profile or launching a browser.
type fakeSurface struct {
	mu     sync.Mutex
	opened []browser.OpenOpts
	closed []state.BrowserHandle
	nextID int
}

func (s *fakeSurface) Open(opts browser.OpenOpts) (state.BrowserHandle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opened = append(s.opened, opts)
	s.nextID++
	return state.BrowserHandle{
		Strategy:   browser.StrategyDedicatedProfile,
		PID:        5000 + s.nextID,
		ProfileDir: "/tmp/fake-clone",
		SeedDir:    "/tmp/fake-seed",
	}, nil
}

func (s *fakeSurface) Alive(h state.BrowserHandle) (bool, error) { return true, nil }

func (s *fakeSurface) Close(h state.BrowserHandle) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = append(s.closed, h)
	return nil
}

func (s *fakeSurface) Name() string { return browser.StrategyDedicatedProfile }

func newBrowserRuntime(t *testing.T, surf browser.Surface) (*Runtime, *mockPlatform, *state.Store) {
	t.Helper()
	st, err := state.OpenAt(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("state.OpenAt: %v", err)
	}
	pf := newMock()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return New(Options{Platform: pf, Store: st, Logger: log, Browser: surf}), pf, st
}

func TestStartRoutesBrowserAppThroughSurface(t *testing.T) {
	surf := &fakeSurface{}
	rt, _, st := newBrowserRuntime(t, surf)

	ws := &config.Workspace{
		Name: "w",
		Apps: []config.App{{
			Type: "browser", Browser: "zen-browser",
			Urls: []string{"https://a", "https://b"}, NewWindow: true,
		}},
	}
	inst, err := rt.Start(ws, config.Source{Path: "/tmp/w.yaml"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	if len(surf.opened) != 1 {
		t.Fatalf("Open called %d times, want 1", len(surf.opened))
	}
	got := surf.opened[0]
	if got.Bin != "zen-browser" || !got.NewWindow {
		t.Errorf("unexpected OpenOpts: %+v", got)
	}

	// The handle is persisted on the app so a later Stop (even after a
	// restart) can close it.
	persisted := st.Snapshot().Instances[inst.ID]
	if len(persisted.Apps) != 1 || persisted.Apps[0].Browser == nil {
		t.Fatalf("browser handle not persisted: %+v", persisted.Apps)
	}
	if persisted.Apps[0].Status != state.StatusRunning {
		t.Errorf("app status = %v, want running", persisted.Apps[0].Status)
	}

	// Stop closes exactly that handle through the Surface, not a raw kill.
	if err := rt.Stop(inst.ID, false); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if len(surf.closed) != 1 {
		t.Fatalf("Close called %d times, want 1", len(surf.closed))
	}
	if surf.closed[0].PID != persisted.Apps[0].Browser.PID {
		t.Errorf("closed wrong handle: %+v vs %+v", surf.closed[0], persisted.Apps[0].Browser)
	}
}
