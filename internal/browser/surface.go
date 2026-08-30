// Package browser owns the "open some URLs as a unit dia can later
// close" problem. A plain process launch cannot solve it: modern
// browsers forward a second `browser <url>` invocation into the
// already-running instance and the launched process exits, so dia is
// left holding a dead PID with no idea which window appeared, and no
// safe way to close only that window.
//
// The Surface interface is the seam that hides this. The one strategy
// is dedicated-profile: dia keeps a managed profile per browser (seeded
// once from the user's real profile), clones it into a throwaway
// runtime profile for each launch, and launches the real browser
// against that clone. Because the clone is a unique profile directory
// the launch is a fresh instance dia owns outright -- a real PID it can
// kill without disturbing the user's own windows -- while the profile
// preserves their logins. On close the runtime profile is written back
// to the managed seed, so anything done in the dia window persists to
// the next launch. The seed is refreshed from the real profile only on
// an explicit `dia browser refresh`, never silently, because merging a
// live browser profile at rest cannot be done safely.
package browser

import (
	"errors"

	"github.com/DerekCorniello/dia/internal/state"
)

// OpenOpts describes one URL group to open under the managed profile.
type OpenOpts struct {
	// Bin is the browser binary (e.g. "zen-browser", "firefox",
	// "chromium"). Its basename selects the family adapter.
	Bin string
	// URLs are opened as tabs in a single new window, in order.
	URLs []string
	// NewWindow is honored by the adapter; for an owned fresh
	// instance the launch is effectively always its own window.
	NewWindow bool
	// Env is appended to the browser process environment ("KEY=VALUE").
	Env []string
	// Instance is the runtime instance ID, used only to label the
	// ephemeral profile directory for debuggability.
	Instance string
}

// Surface opens, checks, and closes owned browser URL groups.
type Surface interface {
	// Open shows opts.URLs as one owned unit and returns a handle
	// that identifies it for a later Close.
	Open(opts OpenOpts) (state.BrowserHandle, error)
	// Alive reports whether the handle's browser process is still
	// running. A gone process is (false, nil), not an error.
	Alive(h state.BrowserHandle) (bool, error)
	// Close terminates the owned process, writes the runtime profile
	// back to the managed seed, and removes the runtime profile. A
	// handle whose process is already gone is a no-op success.
	Close(h state.BrowserHandle) error
	// Name is the strategy's stable identifier, matching the value
	// stored in state.BrowserHandle.Strategy.
	Name() string
}

// StrategyDedicatedProfile is the Strategy tag stored in handles minted
// by the dedicated-profile surface.
const StrategyDedicatedProfile = "dedicated-profile"

// ErrNoSeedProfile is returned by Open when the user's real profile for
// the requested browser cannot be located, so the caller can fall back
// to a plain launch instead of failing the app outright.
var ErrNoSeedProfile = errors.New("browser: no seed profile found")

// ErrUnsupportedBrowser is returned by Open when Bin is not a browser
// this package has an adapter for.
var ErrUnsupportedBrowser = errors.New("browser: unsupported browser binary")

// ErrInsufficientSpace is returned when a plain (non-reflink) profile
// copy would not fit in the free space available.
var ErrInsufficientSpace = errors.New("browser: insufficient disk space to clone profile")
