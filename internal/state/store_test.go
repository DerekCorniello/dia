package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestOpenAtRejectsFutureSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	data := []byte(`{"version":2,"instances":{}}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := OpenAt(path)
	if err == nil || !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("OpenAt error = %v, want future-schema error", err)
	}
}

func TestOpenAtNormalizesMissingSchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"instances":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Snapshot().Version; got != SchemaVersion {
		t.Fatalf("version = %d, want %d", got, SchemaVersion)
	}
}

func TestOpenAtCreatesEmptyOnMissing(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	s, err := OpenAt(p)
	if err != nil {
		t.Fatal(err)
	}
	if s.Path() != p {
		t.Errorf("Path = %q, want %q", s.Path(), p)
	}
	snap := s.Snapshot()
	if snap.Version != 1 {
		t.Errorf("Version = %d, want 1", snap.Version)
	}
	if len(snap.Instances) != 0 {
		t.Errorf("expected empty Instances, got %d", len(snap.Instances))
	}
}

func TestMutateAndReload(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")

	s, err := OpenAt(p)
	if err != nil {
		t.Fatal(err)
	}
	want := Instance{
		ID:            "abcd1234efgh",
		WorkspaceName: "demo",
		WorkspacePath: "/tmp/demo.yaml",
		StartedAt:     time.Now().UTC().Truncate(time.Second),
		Apps: []AppProcess{
			{Type: "editor", Cmd: "code .", PID: 12345, Status: StatusRunning},
		},
		Status: StatusRunning,
	}
	if err := s.Mutate(func(d *Data) { d.Instances[want.ID] = want }); err != nil {
		t.Fatal(err)
	}

	s2, err := OpenAt(p)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := s2.Snapshot().Instances[want.ID]
	if !ok {
		t.Fatalf("instance %q not found in reloaded state", want.ID)
	}
	if got.WorkspaceName != want.WorkspaceName {
		t.Errorf("WorkspaceName = %q, want %q", got.WorkspaceName, want.WorkspaceName)
	}
	if got.Apps[0].PID != 12345 {
		t.Errorf("PID = %d, want 12345", got.Apps[0].PID)
	}
}

func TestMutateErrAbortsWrite(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	s, err := OpenAt(p)
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errSentinel("nope")
	if err := s.MutateErr(func(d *Data) error {
		d.Instances["x"] = Instance{ID: "x", Status: StatusRunning}
		return sentinel
	}); err != sentinel {
		t.Fatalf("expected sentinel, got %v", err)
	}
	// File should not exist since the only write was aborted.
	if _, err := os.Stat(p); err == nil {
		t.Error("expected no file after aborted write")
	}
}

func TestSnapshotIndependentOfMutations(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenAt(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Mutate(func(d *Data) {
		d.Recent = append(d.Recent, RecentEntry{Name: "alpha", Count: 1})
	}); err != nil {
		t.Fatal(err)
	}
	snap := s.Snapshot()
	// Mutating the returned struct's top-level fields must not
	// affect the store. Map mutations would; that's why
	// callers must use Mutate.
	snap.Version = 999
	if got := s.Snapshot().Version; got == 999 {
		t.Error("Snapshot returned a reference; expected a value copy")
	}
}

func TestSnapshotDeepCopiesNestedValues(t *testing.T) {
	s, err := OpenAt(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Mutate(func(d *Data) {
		d.Instances["x"] = Instance{Apps: []AppProcess{{Browser: &BrowserHandle{PID: 7}}}}
		d.Plugins["p"] = PluginState{GrantedCapabilities: []string{"read"}, Config: map[string]any{"nested": map[string]any{"ok": true}}}
	}); err != nil {
		t.Fatal(err)
	}
	snap := s.Snapshot()
	snap.Instances["x"].Apps[0].Browser.PID = 99
	snap.Plugins["p"].GrantedCapabilities[0] = "write"
	snap.Plugins["p"].Config["nested"].(map[string]any)["ok"] = false
	got := s.Snapshot()
	if got.Instances["x"].Apps[0].Browser.PID != 7 || got.Plugins["p"].GrantedCapabilities[0] != "read" || !got.Plugins["p"].Config["nested"].(map[string]any)["ok"].(bool) {
		t.Fatal("nested Snapshot values alias store state")
	}
}

func TestAtomicWriteLeavesNoTempOnSuccess(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenAt(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("expected only state.json, got %d entries: %v", len(entries), entries)
	}
}

func TestStateBackupRetainsPreviousSnapshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, err := OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Mutate(func(d *Data) { d.Theme = "first" }); err != nil {
		t.Fatal(err)
	}
	if err := s.Mutate(func(d *Data) { d.Theme = "second" }); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	var previous Data
	if err := json.Unmarshal(backup, &previous); err != nil {
		t.Fatal(err)
	}
	if previous.Theme != "first" {
		t.Fatalf("backup theme = %q, want first", previous.Theme)
	}
}

func TestFileIsValidJSON(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	s, err := OpenAt(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Mutate(func(d *Data) {
		d.Recent = []RecentEntry{{Name: "a", Count: 1}, {Name: "b", Count: 1}}
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("state file is not valid JSON: %v", err)
	}
}

func TestOpenAtMigratesLegacyRecent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	legacy := `{"version":1,"recent":["alpha","beta"],"instances":{}}`
	if err := os.WriteFile(p, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := OpenAt(p)
	if err != nil {
		t.Fatal(err)
	}
	snap := s.Snapshot()
	if len(snap.Recent) != 2 {
		t.Fatalf("Recent len = %d, want 2", len(snap.Recent))
	}
	if snap.Recent[0].Name != "alpha" || snap.Recent[0].Count != 0 {
		t.Errorf("Recent[0] = %+v, want {alpha, 0}", snap.Recent[0])
	}
	if snap.Recent[1].Name != "beta" || snap.Recent[1].Count != 0 {
		t.Errorf("Recent[1] = %+v, want {beta, 0}", snap.Recent[1])
	}
}

func TestConcurrentMutationsPreserveAllWrites(t *testing.T) {
	dir := t.TempDir()
	s, err := OpenAt(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}

	const goroutines = 16
	const perGoroutine = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				id := idFor(g, i)
				if err := s.Mutate(func(d *Data) {
					d.Instances[id] = Instance{ID: id, Status: StatusRunning}
				}); err != nil {
					t.Errorf("mutate: %v", err)
					return
				}
			}
		}(g)
	}
	wg.Wait()

	got := s.Snapshot().Instances
	if len(got) != goroutines*perGoroutine {
		t.Errorf("expected %d instances, got %d", goroutines*perGoroutine, len(got))
	}
}

func TestConcurrentStoresPreserveUnrelatedMutations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	a, err := OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	go func() {
		<-start
		errs <- a.Mutate(func(d *Data) { d.Theme = "one" })
	}()
	go func() {
		<-start
		errs <- b.Mutate(func(d *Data) { d.ProjectDir = "/projects" })
	}()
	close(start)
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	final, err := OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	snap := final.Snapshot()
	if snap.Theme != "one" || snap.ProjectDir != "/projects" {
		t.Fatalf("concurrent stores lost an update: %+v", snap)
	}
}

// TestCorruptStateFailsWithBackupHint simulates a crash mid-write: a
// torn state.json must fail loudly with a pointer to the backup, and
// the backup must contain the last good snapshot so recovery is exact.
func TestCorruptStateFailsWithBackupHint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	s, err := OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Mutate(func(d *Data) { d.Theme = "dia" }); err != nil {
		t.Fatal(err)
	}
	if err := s.Mutate(func(d *Data) { d.Theme = "midnight" }); err != nil {
		t.Fatal(err)
	}
	// Tear the file as a killed process would.
	if err := os.WriteFile(path, []byte(`{"version": 1, "instances": {`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = OpenAt(path)
	if err == nil {
		t.Fatal("expected error opening torn state, got nil")
	}
	if !strings.Contains(err.Error(), ".bak") {
		t.Errorf("error should point at the backup, got: %v", err)
	}
	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	var data Data
	if err := json.Unmarshal(backup, &data); err != nil {
		t.Fatalf("backup is not valid JSON: %v", err)
	}
	// The backup is the previous persisted snapshot (one write behind
	// the torn tip), which is the documented recovery point.
	if data.Theme != "dia" {
		t.Errorf("backup theme = %q, want %q (previous snapshot)", data.Theme, "dia")
	}
	// Restoring the backup recovers to that snapshot.
	if err := os.WriteFile(path, backup, 0o600); err != nil {
		t.Fatal(err)
	}
	recovered, err := OpenAt(path)
	if err != nil {
		t.Fatalf("reopen after restore: %v", err)
	}
	if recovered.Snapshot().Theme != "dia" {
		t.Errorf("recovered theme = %q, want dia", recovered.Snapshot().Theme)
	}
}

func idFor(g, i int) string {
	return string([]byte{
		byte('a' + (g / 16)),
		byte('a' + (g % 16)),
		byte('a' + (i / 26)),
		byte('a' + (i % 26)),
	})
}

func TestMutateIfChangedSkipsWriteWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	s, err := OpenAt(p)
	if err != nil {
		t.Fatal(err)
	}

	// Seed a real file on disk, then remove it. If MutateIfChanged
	// writes despite fn reporting no change, the file reappears.
	if err := s.Mutate(func(d *Data) { d.Theme = "seed" }); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}

	if err := s.MutateIfChanged(func(d *Data) bool {
		_ = d.Theme // read-only access, nothing changes
		return false
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(p); err == nil {
		t.Error("MutateIfChanged wrote to disk even though fn reported no change")
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestMutateIfChangedWritesWhenChanged(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "state.json")
	s, err := OpenAt(p)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.MutateIfChanged(func(d *Data) bool {
		d.Theme = "changed"
		return true
	}); err != nil {
		t.Fatal(err)
	}

	reloaded, err := OpenAt(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Snapshot().Theme; got != "changed" {
		t.Errorf("Theme = %q, want %q", got, "changed")
	}
}

type sentinelErr string

func (s sentinelErr) Error() string { return string(s) }

func errSentinel(s string) error { return sentinelErr(s) }

// A brand new store (no state.json on disk) must have every map ready
// to write into. Plugins was left nil here, so the first
// d.Plugins[id] = ... on a fresh install panicked.
func TestOpenAt_FreshStoreHasNonNilMaps(t *testing.T) {
	s, err := OpenAt(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	err = s.Mutate(func(d *Data) {
		d.Instances["i"] = Instance{ID: "i"}
		d.CustomThemes["t"] = CustomTheme{}
		d.Plugins["p"] = PluginState{GrantedCapabilities: []string{"workspaces:read"}}
	})
	if err != nil {
		t.Fatalf("Mutate on a fresh store: %v", err)
	}
	if len(s.Snapshot().Plugins["p"].GrantedCapabilities) != 1 {
		t.Error("plugin state was not persisted")
	}
}
