package browser

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/DerekCorniello/dia/internal/platform"
	"github.com/DerekCorniello/dia/internal/state"
)

// closeGrace is how long Close waits after SIGTERM before escalating to
// a forced kill of the owned browser process.
const closeGrace = 5 * time.Second

// DedicatedProfile launches the real browser against an ephemeral clone
// of a dia-managed seed profile. The seed is created once from the
// user's real profile and thereafter is dia's own, evolving copy:
// changes made in the dia window are written back to it on close, so
// they persist. Because each launch uses a unique clone directory, it
// is a fresh instance dia owns outright -- a real PID it can kill
// without disturbing the user's own windows -- while the clone carries
// their logins. The seed is re-pulled from the real profile only by an
// explicit Refresh, never silently.
type DedicatedProfile struct {
	pf     platform.Platform
	log    *slog.Logger
	cloner cloner

	home        string // user home, for real-profile discovery
	profilesDir string // ephemeral clones live here
	seedsDir    string // managed per-browser seeds live here

	// newAdapter is overridable in tests; defaults to the package
	// adapterFor.
	newAdapter func(bin, home string) (adapter, error)
}

// Options configures a DedicatedProfile.
type Options struct {
	Platform platform.Platform
	Logger   *slog.Logger
	// StateDir is dia's state directory; the browser package keeps its
	// clones and seeds under StateDir/browser.
	StateDir string
	// Home overrides the user home used for real-profile discovery.
	// Empty means os.UserHomeDir.
	Home string
}

// NewDedicatedProfile constructs the strategy. Platform and StateDir are
// required.
func NewDedicatedProfile(opts Options) (*DedicatedProfile, error) {
	if opts.Platform == nil {
		return nil, errors.New("browser: nil platform")
	}
	if opts.StateDir == "" {
		return nil, errors.New("browser: empty state dir")
	}
	home := opts.Home
	if home == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("browser: resolve home: %w", err)
		}
		home = h
	}
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	base := filepath.Join(opts.StateDir, "browser")
	return &DedicatedProfile{
		pf:          opts.Platform,
		log:         log,
		cloner:      newCloner(),
		home:        home,
		profilesDir: filepath.Join(base, "profiles"),
		seedsDir:    filepath.Join(base, "seeds"),
		newAdapter:  adapterFor,
	}, nil
}

func (d *DedicatedProfile) Name() string { return StrategyDedicatedProfile }

// Open clones the managed seed into a fresh ephemeral profile and
// launches the browser against it. The seed is created from the user's
// real profile on first use.
func (d *DedicatedProfile) Open(opts OpenOpts) (state.BrowserHandle, error) {
	var zero state.BrowserHandle
	if len(opts.URLs) == 0 {
		return zero, errors.New("browser: no urls to open")
	}
	ad, err := d.newAdapter(opts.Bin, d.home)
	if err != nil {
		return zero, err
	}
	excludes := ad.cacheExcludes()

	seedDir, err := d.ensureSeed(ad, opts.Bin, excludes)
	if err != nil {
		return zero, err
	}

	if err := os.MkdirAll(d.profilesDir, 0o700); err != nil {
		return zero, fmt.Errorf("browser: create profiles dir: %w", err)
	}
	clone, err := os.MkdirTemp(d.profilesDir, sanitizeName(opts.Instance)+"-")
	if err != nil {
		return zero, fmt.Errorf("browser: create clone dir: %w", err)
	}
	// MkdirTemp made the dir; CloneTree requires the target not exist.
	if err := os.Remove(clone); err != nil {
		return zero, fmt.Errorf("browser: prepare clone dir: %w", err)
	}

	if err := d.guardSpace(seedDir, excludes); err != nil {
		return zero, err
	}

	reflink, err := d.cloner.CloneTree(seedDir, clone, excludes)
	if err != nil {
		return zero, err
	}
	d.log.Info("cloned browser profile",
		"bin", opts.Bin, "reflink", reflink, "clone", clone)

	cmd, args := ad.launchArgs(clone, opts.URLs, opts.NewWindow)
	handle, err := d.pf.Launch(platform.LaunchOpts{Cmd: cmd, Args: args, Env: opts.Env})
	if err != nil {
		_ = os.RemoveAll(clone)
		return zero, fmt.Errorf("browser: launch %s: %w", cmd, err)
	}

	return state.BrowserHandle{
		Strategy:   StrategyDedicatedProfile,
		PID:        handle.PID(),
		ProfileDir: clone,
		SeedDir:    seedDir,
	}, nil
}

// ensureSeed returns the managed seed directory for a browser, creating
// it by cloning the user's real profile the first time it is needed.
func (d *DedicatedProfile) ensureSeed(ad adapter, bin string, excludes []string) (string, error) {
	seedDir := d.seedPath(bin)
	if _, err := os.Stat(seedDir); err == nil {
		return seedDir, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	real, err := ad.seedProfile()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(d.seedsDir, 0o700); err != nil {
		return "", fmt.Errorf("browser: create seeds dir: %w", err)
	}
	if _, err := d.cloner.CloneTree(real, seedDir, excludes); err != nil {
		return "", fmt.Errorf("browser: seed profile: %w", err)
	}
	d.log.Info("initialized browser seed", "bin", bin, "seed", seedDir)
	return seedDir, nil
}

// seedPath is the managed seed directory for a browser binary.
func (d *DedicatedProfile) seedPath(bin string) string {
	return filepath.Join(d.seedsDir, sanitizeName(filepath.Base(bin)))
}

// Refresh re-pulls a browser's managed seed from the user's current real
// profile, discarding the seed's accumulated dia-side state. This is the
// deliberate, predictable alternative to an unsafe automatic merge: the
// user runs it when they want the dia profile to match their real
// browser's current logins again.
func (d *DedicatedProfile) Refresh(bin string) error {
	ad, err := d.newAdapter(bin, d.home)
	if err != nil {
		return err
	}
	real, err := ad.seedProfile()
	if err != nil {
		return err
	}
	seedDir := d.seedPath(bin)
	tmp := seedDir + ".refresh"
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(d.seedsDir, 0o700); err != nil {
		return fmt.Errorf("browser: create seeds dir: %w", err)
	}
	if _, err := d.cloner.CloneTree(real, tmp, ad.cacheExcludes()); err != nil {
		_ = os.RemoveAll(tmp)
		return fmt.Errorf("browser: refresh seed: %w", err)
	}
	old := seedDir + ".old"
	_ = os.RemoveAll(old)
	if _, err := os.Stat(seedDir); err == nil {
		if err := os.Rename(seedDir, old); err != nil {
			_ = os.RemoveAll(tmp)
			return err
		}
	}
	if err := os.Rename(tmp, seedDir); err != nil {
		_ = os.Rename(old, seedDir)
		_ = os.RemoveAll(tmp)
		return err
	}
	_ = os.RemoveAll(old)
	d.log.Info("refreshed browser seed", "bin", bin, "seed", seedDir)
	return nil
}

// SeededBrowsers lists the browser binaries that currently have a
// managed seed, for `dia browser refresh` with no argument.
func (d *DedicatedProfile) SeededBrowsers() ([]string, error) {
	entries, err := os.ReadDir(d.seedsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		// Skip the transient .old/.refresh/.new/.lock siblings.
		name := e.Name()
		if !e.IsDir() || filepath.Ext(name) != "" {
			continue
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

// guardSpace refuses a launch that would run the disk out. It is a no-op
// when the filesystem supports reflinks (clones cost ~no space) or when
// the plain copy comfortably fits.
func (d *DedicatedProfile) guardSpace(seed string, excludes []string) error {
	size, err := dirSize(seed, excludes)
	if err != nil {
		return fmt.Errorf("browser: measure seed: %w", err)
	}
	free, err := freeBytes(d.profilesDir)
	if err != nil {
		// Cannot measure; let the copy proceed and fail loudly if it
		// actually runs out.
		return nil
	}
	if free >= uint64(size)+uint64(size)/10 {
		return nil // plain copy fits with 10% headroom
	}
	if reflinkSupported(d.profilesDir) {
		return nil // clones are near-zero space
	}
	return fmt.Errorf("%w: need ~%d bytes, %d free at %s",
		ErrInsufficientSpace, size, free, d.profilesDir)
}

// Alive reports whether the owned browser process is still running.
func (d *DedicatedProfile) Alive(h state.BrowserHandle) (bool, error) {
	if h.PID <= 0 {
		return false, nil
	}
	return d.pf.IsRunning(h.PID)
}

// Close terminates the owned browser, writes the runtime profile back to
// the managed seed, and removes the runtime profile. A process that is
// already gone is a no-op success -- the case that must never fall
// through to closing something else.
func (d *DedicatedProfile) Close(h state.BrowserHandle) error {
	if h.PID > 0 {
		running, err := d.pf.IsRunning(h.PID)
		if err == nil && running {
			_ = d.pf.Kill(h.PID, false)
			deadline := time.Now().Add(closeGrace)
			for {
				alive, _ := d.pf.IsRunning(h.PID)
				if !alive {
					break
				}
				if time.Now().After(deadline) {
					_ = d.pf.Kill(h.PID, true)
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	if h.SeedDir != "" && h.ProfileDir != "" {
		if err := d.writeback(h.ProfileDir, h.SeedDir); err != nil {
			// A failed writeback loses this session's changes but must
			// not block teardown; log and continue to cleanup.
			d.log.Warn("browser seed writeback failed",
				"seed", h.SeedDir, "clone", h.ProfileDir, "error", err)
		}
	}

	if h.ProfileDir != "" {
		if err := os.RemoveAll(h.ProfileDir); err != nil {
			return fmt.Errorf("browser: remove clone %s: %w", h.ProfileDir, err)
		}
	}
	return nil
}

// writeback replaces the managed seed with the contents of the runtime
// profile, so changes made inside the dia window persist. It is
// serialized by a lock file: if another instance holds the seed, this
// writeback is skipped (last-writer-wins) rather than corrupting a
// concurrent one.
func (d *DedicatedProfile) writeback(clone, seed string) error {
	lock := seed + ".lock"
	lf, err := os.OpenFile(lock, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			d.log.Warn("browser seed busy, skipping writeback", "seed", seed)
			return nil
		}
		return err
	}
	lf.Close()
	defer os.Remove(lock)

	// Build the new seed beside the old one, then swap: a crash mid-way
	// leaves the previous seed intact.
	tmp := seed + ".new"
	_ = os.RemoveAll(tmp)
	if _, err := d.cloner.CloneTree(clone, tmp, seedWritebackExcludes); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	old := seed + ".old"
	_ = os.RemoveAll(old)
	if err := os.Rename(seed, old); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	if err := os.Rename(tmp, seed); err != nil {
		// Best-effort restore of the previous seed.
		_ = os.Rename(old, seed)
		_ = os.RemoveAll(tmp)
		return err
	}
	_ = os.RemoveAll(old)
	return nil
}

// seedWritebackExcludes are dirs skipped when copying a runtime profile
// back to the seed: the browser's own lock files must never be written
// back, or the next launch would see a stale lock.
var seedWritebackExcludes = []string{
	"lock", ".parentlock", "cache2", "startupCache", "shader-cache",
	"minidumps", "Crash Reports",
	"Default/Cache", "Default/Code Cache", "Default/GPUCache", "ShaderCache",
	"SingletonLock", "SingletonSocket", "SingletonCookie",
}

// reflinkSupported probes whether dir's filesystem can copy-on-write
// clone, by cloning a throwaway file. Any failure means "no".
func reflinkSupported(dir string) bool {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false
	}
	src, err := os.CreateTemp(dir, "reflink-src-")
	if err != nil {
		return false
	}
	srcName := src.Name()
	_, _ = src.WriteString("x")
	src.Close()
	defer os.Remove(srcName)

	dstName := srcName + ".clone"
	ok, err := cloneFile(srcName, dstName)
	_ = os.Remove(dstName)
	return err == nil && ok
}

// sanitizeName makes an instance/binary label safe as a path component.
func sanitizeName(s string) string {
	if s == "" {
		return "inst"
	}
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}
