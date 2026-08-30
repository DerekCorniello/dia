package browser

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/DerekCorniello/dia/internal/platform"
)

// fakePlatform records launches and kills without ever exec'ing a real
// process, so tests never open a browser window.
type fakePlatform struct {
	mu       sync.Mutex
	launches []platform.LaunchOpts
	killed   map[int]bool
	nextPID  int
	// alive maps pid -> running; unset pids are treated as not running.
	alive map[int]bool
}

func newFakePlatform() *fakePlatform {
	return &fakePlatform{killed: map[int]bool{}, alive: map[int]bool{}, nextPID: 1000}
}

type fakeHandle struct{ pid int }

func (h fakeHandle) PID() int              { return h.pid }
func (h fakeHandle) Done() <-chan struct{} { return nil }

func (f *fakePlatform) Launch(opts platform.LaunchOpts) (platform.ProcessHandle, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.launches = append(f.launches, opts)
	f.nextPID++
	f.alive[f.nextPID] = true
	return fakeHandle{pid: f.nextPID}, nil
}

func (f *fakePlatform) IsRunning(pid int) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.alive[pid], nil
}

func (f *fakePlatform) Kill(pid int, force bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.killed[pid] = true
	f.alive[pid] = false
	return nil
}

func (f *fakePlatform) OpenURL(string) error             { return nil }
func (f *fakePlatform) RevealInFileManager(string) error { return nil }
func (f *fakePlatform) OpenFile(string) error            { return nil }
func (f *fakePlatform) Run(platform.LaunchOpts, time.Duration) (string, error) {
	return "", nil
}

func (f *fakePlatform) lastLaunch(t *testing.T) platform.LaunchOpts {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.launches) == 0 {
		t.Fatal("no launch recorded")
	}
	return f.launches[len(f.launches)-1]
}

// writeZenSeed builds a minimal but realistic Zen profile tree under
// home and returns the home dir. It includes a cache dir that must be
// excluded and a login-critical file that must survive.
func writeZenSeed(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	prof := filepath.Join(home, ".zen", "abcd1234.Default (release)")
	mustMkdir(t, prof)
	mustWrite(t, filepath.Join(home, ".zen", "profiles.ini"), ""+
		"[Profile0]\nName=Default (release)\nIsRelative=1\nPath=abcd1234.Default (release)\nDefault=1\n\n"+
		"[General]\nStartWithLastProfile=1\nVersion=2\n\n"+
		"[Install12345]\nDefault=abcd1234.Default (release)\nLocked=1\n")
	mustWrite(t, filepath.Join(prof, "key4.db"), "SECRET-KEYS")
	mustWrite(t, filepath.Join(prof, "cookies.sqlite"), "COOKIES")
	mustWrite(t, filepath.Join(prof, "cookies.sqlite-wal"), "COOKIES-WAL")
	mustMkdir(t, filepath.Join(prof, "cache2", "entries"))
	mustWrite(t, filepath.Join(prof, "cache2", "entries", "junk"), "CACHE")
	mustWrite(t, filepath.Join(prof, "lock"), "LOCK")
	return home
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, p, content string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newSurface(t *testing.T, home string, pf platform.Platform) *DedicatedProfile {
	t.Helper()
	d, err := NewDedicatedProfile(Options{
		Platform: pf,
		StateDir: t.TempDir(),
		Home:     home,
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestOpenClonesAndLaunches(t *testing.T) {
	home := writeZenSeed(t)
	pf := newFakePlatform()
	d := newSurface(t, home, pf)

	h, err := d.Open(OpenOpts{
		Bin:       "zen-browser",
		URLs:      []string{"https://a.example.com", "https://b.example.com"},
		NewWindow: true,
		Instance:  "inst-xyz",
	})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if h.Strategy != StrategyDedicatedProfile {
		t.Fatalf("unexpected handle: %+v", h)
	}
	if h.SeedDir == "" {
		t.Error("handle must record the seed dir for writeback")
	}
	if h.PID <= 0 || h.ProfileDir == "" {
		t.Fatalf("handle missing pid/profile: %+v", h)
	}

	// Login-critical files copied; cache and lock excluded.
	if got := readFile(t, filepath.Join(h.ProfileDir, "key4.db")); got != "SECRET-KEYS" {
		t.Errorf("key4.db not cloned: %q", got)
	}
	if _, err := os.Stat(filepath.Join(h.ProfileDir, "cookies.sqlite-wal")); err != nil {
		t.Errorf("cookies wal sibling not cloned: %v", err)
	}
	if _, err := os.Stat(filepath.Join(h.ProfileDir, "cache2")); !os.IsNotExist(err) {
		t.Errorf("cache2 should have been excluded, stat err=%v", err)
	}

	// Launch used the clone dir and both URLs.
	l := pf.lastLaunch(t)
	if l.Cmd != "zen-browser" {
		t.Errorf("cmd = %q", l.Cmd)
	}
	assertArgHasValue(t, l.Args, "--profile", h.ProfileDir)
	assertArgContains(t, l.Args, "https://a.example.com")
	assertArgContains(t, l.Args, "https://b.example.com")
}

func TestCloseKillsAndRemovesClone(t *testing.T) {
	home := writeZenSeed(t)
	pf := newFakePlatform()
	d := newSurface(t, home, pf)

	h, err := d.Open(OpenOpts{Bin: "zen-browser", URLs: []string{"https://x"}, Instance: "i"})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Close(h); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !pf.killed[h.PID] {
		t.Errorf("pid %d was not killed", h.PID)
	}
	if _, err := os.Stat(h.ProfileDir); !os.IsNotExist(err) {
		t.Errorf("clone dir should be removed, stat err=%v", err)
	}
}

func TestCloseAlreadyGoneIsNoop(t *testing.T) {
	home := writeZenSeed(t)
	pf := newFakePlatform()
	d := newSurface(t, home, pf)

	h, err := d.Open(OpenOpts{Bin: "zen-browser", URLs: []string{"https://x"}, Instance: "i"})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate the user having already closed the window.
	pf.alive[h.PID] = false

	if err := d.Close(h); err != nil {
		t.Fatalf("Close on gone process must be nil, got %v", err)
	}
	if pf.killed[h.PID] {
		t.Errorf("must not kill an already-gone process (risk: closing the wrong thing)")
	}
}

func TestAlive(t *testing.T) {
	home := writeZenSeed(t)
	pf := newFakePlatform()
	d := newSurface(t, home, pf)
	h, _ := d.Open(OpenOpts{Bin: "zen-browser", URLs: []string{"https://x"}, Instance: "i"})

	if ok, _ := d.Alive(h); !ok {
		t.Error("expected alive")
	}
	pf.alive[h.PID] = false
	if ok, _ := d.Alive(h); ok {
		t.Error("expected not alive")
	}
}

func TestSeedsThenWritesBack(t *testing.T) {
	home := writeZenSeed(t)
	pf := newFakePlatform()
	d := newSurface(t, home, pf)

	h, err := d.Open(OpenOpts{Bin: "zen-browser", URLs: []string{"https://x"}, Instance: "i"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if h.SeedDir == "" {
		t.Fatal("Open must set SeedDir for writeback")
	}
	// Seed was created from the real profile.
	if got := readFile(t, filepath.Join(h.SeedDir, "key4.db")); got != "SECRET-KEYS" {
		t.Errorf("seed not initialized from real profile: %q", got)
	}
	// Simulate a login made inside the dia window.
	mustWrite(t, filepath.Join(h.ProfileDir, "logins.json"), "NEW-LOGIN")

	if err := d.Close(h); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Writeback carried the new login into the seed; lock did not travel.
	if got := readFile(t, filepath.Join(h.SeedDir, "logins.json")); got != "NEW-LOGIN" {
		t.Errorf("writeback lost login: %q", got)
	}
	if _, err := os.Stat(filepath.Join(h.SeedDir, "lock")); !os.IsNotExist(err) {
		t.Errorf("browser lock must not be written back to the seed")
	}
}

func TestSeedPersistsAcrossLaunches(t *testing.T) {
	home := writeZenSeed(t)
	pf := newFakePlatform()
	d := newSurface(t, home, pf)

	// First launch seeds and records a dia-side login.
	h1, err := d.Open(OpenOpts{Bin: "zen-browser", URLs: []string{"https://x"}, Instance: "a"})
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(h1.ProfileDir, "logins.json"), "DIA-LOGIN")
	if err := d.Close(h1); err != nil {
		t.Fatal(err)
	}

	// Second launch must inherit the dia-side login from the seed, not
	// re-clone the real profile (which has no logins.json).
	h2, err := d.Open(OpenOpts{Bin: "zen-browser", URLs: []string{"https://x"}, Instance: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(h2.ProfileDir, "logins.json")); got != "DIA-LOGIN" {
		t.Errorf("second launch lost persisted login: %q", got)
	}
}

func TestRefreshRepullsRealProfile(t *testing.T) {
	home := writeZenSeed(t)
	pf := newFakePlatform()
	d := newSurface(t, home, pf)

	// Seed and add a dia-side change.
	h, err := d.Open(OpenOpts{Bin: "zen-browser", URLs: []string{"https://x"}, Instance: "a"})
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(h.ProfileDir, "logins.json"), "STALE-DIA-LOGIN")
	if err := d.Close(h); err != nil {
		t.Fatal(err)
	}

	// Refresh discards the dia-side state and re-pulls the real profile.
	if err := d.Refresh("zen-browser"); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	seed := d.seedPath("zen-browser")
	if _, err := os.Stat(filepath.Join(seed, "logins.json")); !os.IsNotExist(err) {
		t.Error("refresh should have discarded the dia-side login")
	}
	if got := readFile(t, filepath.Join(seed, "key4.db")); got != "SECRET-KEYS" {
		t.Errorf("refresh should re-pull the real profile: %q", got)
	}

	seeded, err := d.SeededBrowsers()
	if err != nil {
		t.Fatal(err)
	}
	if len(seeded) != 1 || seeded[0] != "zen-browser" {
		t.Errorf("SeededBrowsers = %v, want [zen-browser]", seeded)
	}
}

func TestUnsupportedBrowser(t *testing.T) {
	pf := newFakePlatform()
	d := newSurface(t, t.TempDir(), pf)
	_, err := d.Open(OpenOpts{Bin: "netscape", URLs: []string{"https://x"}})
	if err == nil {
		t.Fatal("expected error for unsupported browser")
	}
}

func TestMissingSeedProfile(t *testing.T) {
	pf := newFakePlatform()
	d := newSurface(t, t.TempDir(), pf) // empty home, no ~/.zen
	_, err := d.Open(OpenOpts{Bin: "zen-browser", URLs: []string{"https://x"}})
	if err == nil {
		t.Fatal("expected ErrNoSeedProfile")
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(b)
}

func assertArgHasValue(t *testing.T, args []string, flag, val string) {
	t.Helper()
	for i, a := range args {
		if a == flag && i+1 < len(args) && args[i+1] == val {
			return
		}
	}
	t.Errorf("expected %s %q in args %v", flag, val, args)
}

func assertArgContains(t *testing.T, args []string, val string) {
	t.Helper()
	for _, a := range args {
		if a == val {
			return
		}
	}
	t.Errorf("expected %q in args %v", val, args)
}

// TestCloneUsesReflinkWhereSupported verifies that on a copy-on-write
// filesystem the clone takes the reflink fast path rather than a byte
// copy. It works in a directory on the same filesystem as the user's
// home (btrfs/APFS in dev and CI), and self-skips on filesystems without
// reflink support (e.g. tmpfs) so it never flakes.
func TestCloneUsesReflinkWhereSupported(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	base := filepath.Join(home, ".cache", "dia-reflink-test")
	if err := os.MkdirAll(base, 0o700); err != nil {
		t.Skipf("cannot write under home: %v", err)
	}
	defer os.RemoveAll(base)

	if !reflinkSupported(base) {
		t.Skip("filesystem under home does not support reflinks")
	}

	src := filepath.Join(base, "src")
	mustMkdir(t, filepath.Join(src, "sub"))
	mustWrite(t, filepath.Join(src, "a.bin"), "hello")
	mustWrite(t, filepath.Join(src, "sub", "b.bin"), "world")

	dst := filepath.Join(base, "dst")
	reflink, err := newCloner().CloneTree(src, dst, nil)
	if err != nil {
		t.Fatalf("CloneTree: %v", err)
	}
	if !reflink {
		t.Error("expected reflink fast path on a CoW filesystem")
	}
	if got := readFile(t, filepath.Join(dst, "sub", "b.bin")); got != "world" {
		t.Errorf("cloned content wrong: %q", got)
	}
}
