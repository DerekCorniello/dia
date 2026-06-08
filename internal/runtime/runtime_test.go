package runtime

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/platform"
	"github.com/DerekCorniello/dia/internal/state"
)

func newTestRuntime(t *testing.T) (*Runtime, *mockPlatform, *state.Store) {
	t.Helper()
	st, err := state.OpenAt(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("state.OpenAt: %v", err)
	}
	pf := newMock()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return New(Options{Platform: pf, Store: st, Logger: log}), pf, st
}

type mockPlatform struct {
	mu          sync.Mutex
	nextPID     int
	launched    []platform.LaunchOpts
	killCalls   []killCall
	killFn      func(pid int, force bool) error
	running     map[int]bool
	runningSeed map[int]bool
	openURLs    []string
	openURLErr  error
}

type killCall struct {
	PID   int
	Force bool
}

func newMock() *mockPlatform {
	return &mockPlatform{nextPID: 1000, running: map[int]bool{}, runningSeed: map[int]bool{}}
}

func (m *mockPlatform) Launch(opts platform.LaunchOpts) (platform.ProcessHandle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextPID++
	pid := m.nextPID
	m.launched = append(m.launched, opts)
	m.running[pid] = true
	return &mockHandle{pid: pid, pf: m}, nil
}

func (m *mockPlatform) OpenURL(url string) error {
	m.mu.Lock()
	m.openURLs = append(m.openURLs, url)
	err := m.openURLErr
	m.mu.Unlock()
	return err
}

func (m *mockPlatform) IsRunning(pid int) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.running[pid]
	return r && ok, nil
}

func (m *mockPlatform) Kill(pid int, force bool) error {
	m.mu.Lock()
	kc := killCall{PID: pid, Force: force}
	fn := m.killFn
	m.mu.Unlock()
	if fn != nil {
		if err := fn(pid, force); err != nil {
			return err
		}
	}
	m.mu.Lock()
	m.killCalls = append(m.killCalls, kc)
	delete(m.running, pid)
	m.mu.Unlock()
	return nil
}

func (m *mockPlatform) RevealInFileManager(path string) error { return nil }

func (m *mockPlatform) OpenFile(path string) error { return nil }

func (m *mockPlatform) MarkDead(pid int) {
	m.mu.Lock()
	delete(m.running, pid)
	m.mu.Unlock()
}

type mockHandle struct {
	pid int
	pf  *mockPlatform
}

func (h *mockHandle) PID() int { return h.pid }
func (h *mockHandle) Done() <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		for {
			r, _ := h.pf.IsRunning(h.pid)
			if !r {
				close(ch)
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()
	return ch
}

func TestNewID(t *testing.T) {
	ids := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		id := newID()
		if len(id) != 12 {
			t.Fatalf("id length = %d, want 12", len(id))
		}
		for _, r := range id {
			if !((r >= 'A' && r <= 'Z') || (r >= '2' && r <= '7')) {
				t.Fatalf("id %q contains non-base32 char %q", id, r)
			}
		}
		if _, dup := ids[id]; dup {
			t.Fatalf("duplicate id %q in 1000 runs", id)
		}
		ids[id] = struct{}{}
	}
}

func TestResolvePath(t *testing.T) {
	t.Setenv("DIA_TEST_VAR", "/tmp/expanded")
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"/abs/path", "/abs/path"},
		{"$DIA_TEST_VAR/file", "/tmp/expanded/file"},
		{"${DIA_TEST_VAR}/file", "/tmp/expanded/file"},
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		cases = append(cases, struct{ in, want string }{"~/x", filepath.Join(home, "x")})
	}
	for _, c := range cases {
		got, err := resolvePath(c.in)
		if err != nil {
			t.Errorf("resolvePath(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("resolvePath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStart_OpenURL(t *testing.T) {
	rt, pf, _ := newTestRuntime(t)
	w := &config.Workspace{
		Name: "urlws",
		Apps: []config.App{
			{Type: "browser", Url: "https://example.com"},
			{Type: "open", Url: "mailto:hi@example.com"},
		},
	}
	inst, err := rt.Start(w, config.Source{Path: "/x"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(inst.Apps) != 2 {
		t.Fatalf("apps = %d, want 2", len(inst.Apps))
	}
	for i, a := range inst.Apps {
		if a.PID != 0 {
			t.Errorf("apps[%d].PID = %d, want 0 (URL apps have no PID)", i, a.PID)
		}
		if a.Status != state.StatusRunning {
			t.Errorf("apps[%d].Status = %s, want running", i, a.Status)
		}
	}
	// Apps launch concurrently, so don't assume openURLs order.
	sort.Strings(pf.openURLs)
	if got := strings.Join(pf.openURLs, ","); got != "https://example.com,mailto:hi@example.com" {
		t.Errorf("openURLs = %q, want https...,mailto:...", got)
	}
	// The Cmd field in the state should be the URL, not the type
	// or anything else, so the user can see what was opened.
	if inst.Apps[0].Cmd != "https://example.com" {
		t.Errorf("apps[0].Cmd = %q, want https://example.com", inst.Apps[0].Cmd)
	}
}

func TestStart_OpenURLFailure(t *testing.T) {
	rt, pf, _ := newTestRuntime(t)
	pf.openURLErr = errors.New("xdg-open exploded")
	w := &config.Workspace{
		Name: "failurl",
		Apps: []config.App{{Type: "open", Url: "https://example.com"}},
	}
	inst, err := rt.Start(w, config.Source{Path: "/x"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if inst.Apps[0].Status != state.StatusCrashed {
		t.Errorf("Status = %s, want crashed", inst.Apps[0].Status)
	}
	if !strings.Contains(inst.Apps[0].Err, "open url") {
		t.Errorf("Err = %q, want contains 'open url'", inst.Apps[0].Err)
	}
}

func TestStart_GHSugar(t *testing.T) {
	rt, pf, _ := newTestRuntime(t)
	w := &config.Workspace{
		Name: "ghws",
		Apps: []config.App{
			{Type: "gh:pr", Args: []string{"view", "123", "--web"}},
		},
	}
	inst, err := rt.Start(w, config.Source{Path: "/x"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if inst.Apps[0].PID == 0 {
		t.Fatalf("no process started")
	}
	if got := pf.launched[0].Cmd; got != "gh" {
		t.Errorf("Cmd = %q, want gh", got)
	}
	want := []string{"pr", "view", "123", "--web"}
	if strings.Join(pf.launched[0].Args, ",") != strings.Join(want, ",") {
		t.Errorf("Args = %v, want %v", pf.launched[0].Args, want)
	}
}

func TestStart_UnknownType(t *testing.T) {
	rt, _, _ := newTestRuntime(t)
	w := &config.Workspace{
		Name: "nope",
		Apps: []config.App{{Type: "nope"}},
	}
	inst, err := rt.Start(w, config.Source{Path: "/x"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if inst.Apps[0].Status != state.StatusCrashed {
		t.Errorf("Status = %s, want crashed", inst.Apps[0].Status)
	}
	if !strings.Contains(inst.Apps[0].Err, "unknown app type") {
		t.Errorf("Err = %q, want contains 'unknown app type'", inst.Apps[0].Err)
	}
}

func TestStart_LocalCmdWithSpaces(t *testing.T) {
	rt, pf, _ := newTestRuntime(t)
	w := &config.Workspace{
		Name: "splitsy",
		Apps: []config.App{{Type: "local", Cmd: `code "/tmp/My Code"`}},
	}
	if _, err := rt.Start(w, config.Source{Path: "/x"}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if pf.launched[0].Cmd != "code" {
		t.Errorf("Cmd = %q, want code", pf.launched[0].Cmd)
	}
	if got := strings.Join(pf.launched[0].Args, ","); got != "/tmp/My Code" {
		t.Errorf("Args = %v, want [/tmp/My Code]", pf.launched[0].Args)
	}
}

func TestStop_OpenAppNoPID(t *testing.T) {
	rt, pf, _ := newTestRuntime(t)
	w := &config.Workspace{
		Name: "ws",
		Apps: []config.App{
			{Type: "local", Cmd: "echo"},
			{Type: "open", Url: "https://example.com"},
		},
	}
	inst, err := rt.Start(w, config.Source{Path: "/x"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := rt.Stop(inst.ID, true); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	// Only the local app should have a kill call. The open app
	// has no PID.
	if len(pf.killCalls) != 1 {
		t.Errorf("killCalls = %d, want 1", len(pf.killCalls))
	}
}

func TestPushRecent(t *testing.T) {
	r := pushRecent(nil, "a", 3)
	r = pushRecent(r, "b", 3)
	r = pushRecent(r, "a", 3)
	if len(r) != 2 {
		t.Fatalf("after dedup: len = %d, want 2", len(r))
	}
	if r[0].Name != "a" || r[0].Count != 2 {
		t.Errorf("r[0] = %+v, want {a, 2}", r[0])
	}
	if r[1].Name != "b" || r[1].Count != 1 {
		t.Errorf("r[1] = %+v, want {b, 1}", r[1])
	}
	for i := 0; i < 10; i++ {
		r = pushRecent(r, fmt.Sprintf("ws%d", i), 3)
	}
	if len(r) != 3 {
		t.Errorf("limit not enforced: len = %d", len(r))
	}
}

func TestStart_LocalAndCustom(t *testing.T) {
	rt, pf, st := newTestRuntime(t)
	w := &config.Workspace{
		Name: "test",
		Apps: []config.App{
			{Type: "local", Cmd: "echo", Cwd: t.TempDir()},
			{Type: "custom", Cmd: "echo", Cwd: t.TempDir()},
		},
	}
	inst, err := rt.Start(w, config.Source{Path: "/fake/path/test.yaml"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if inst.Status != state.StatusRunning {
		t.Errorf("inst.Status = %s, want running", inst.Status)
	}
	if len(inst.Apps) != 2 {
		t.Fatalf("len(apps) = %d, want 2", len(inst.Apps))
	}
	for i, a := range inst.Apps {
		if a.PID == 0 {
			t.Errorf("apps[%d].PID = 0", i)
		}
		if a.Status != state.StatusRunning {
			t.Errorf("apps[%d].Status = %s, want running", i, a.Status)
		}
		if a.Err != "" {
			t.Errorf("apps[%d].Err = %q, want empty", i, a.Err)
		}
	}
	if len(pf.launched) != 2 {
		t.Errorf("mock launched = %d, want 2", len(pf.launched))
	}
	snap := st.Snapshot()
	if _, ok := snap.Instances[inst.ID]; !ok {
		t.Errorf("instance not persisted")
	}
	if len(snap.Recent) != 1 || snap.Recent[0].Name != "test" {
		t.Errorf("Recent = %+v, want [{test 1}]", snap.Recent)
	}
}

func TestStart_PreservesAppOrder(t *testing.T) {
	rt, _, _ := newTestRuntime(t)
	w := &config.Workspace{
		Name: "ord",
		Apps: []config.App{
			{Type: "local", Cmd: "a"},
			{Type: "local", Cmd: "b"},
			{Type: "local", Cmd: "c"},
		},
	}
	inst, err := rt.Start(w, config.Source{Path: "/x.yaml"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	want := []string{"a", "b", "c"}
	for i, app := range inst.Apps {
		if app.Cmd != want[i] {
			t.Errorf("apps[%d].Cmd = %q, want %q", i, app.Cmd, want[i])
		}
	}
}

func TestStart_AllAppsFail(t *testing.T) {
	rt, _, _ := newTestRuntime(t)
	rt.pf = &failingPlatform{}
	w := &config.Workspace{
		Name: "broken",
		Apps: []config.App{{Type: "local", Cmd: "x"}},
	}
	inst, err := rt.Start(w, config.Source{Path: "/x"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if inst.Status != state.StatusCrashed {
		t.Errorf("Status = %s, want crashed", inst.Status)
	}
}

type failingPlatform struct{}

func (failingPlatform) Launch(platform.LaunchOpts) (platform.ProcessHandle, error) {
	return nil, errors.New("simulated launch failure")
}
func (failingPlatform) OpenURL(string) error             { return nil }
func (failingPlatform) IsRunning(int) (bool, error)      { return false, nil }
func (failingPlatform) Kill(int, bool) error             { return nil }
func (failingPlatform) RevealInFileManager(string) error { return nil }
func (failingPlatform) OpenFile(string) error            { return nil }

func TestStart_NilOrEmptyWorkspace(t *testing.T) {
	rt, _, _ := newTestRuntime(t)
	if _, err := rt.Start(nil, config.Source{}); err == nil {
		t.Errorf("expected error on nil workspace")
	}
	if _, err := rt.Start(&config.Workspace{Name: "x"}, config.Source{}); err == nil {
		t.Errorf("expected error on empty apps")
	}
}

func TestStart_BadCwd(t *testing.T) {
	rt, _, _ := newTestRuntime(t)
	// Force os.UserHomeDir to fail by clearing HOME on unix.
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		t.Setenv("HOME", "")
	} else {
		t.Setenv("USERPROFILE", "")
	}
	w := &config.Workspace{
		Name: "bad",
		Apps: []config.App{{Type: "local", Cmd: "echo", Cwd: "~/x"}},
	}
	inst, err := rt.Start(w, config.Source{Path: "/x"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if inst.Apps[0].Status != state.StatusCrashed {
		t.Errorf("Status = %s, want crashed", inst.Apps[0].Status)
	}
	if !strings.Contains(inst.Apps[0].Err, "resolve cwd") {
		t.Errorf("Err = %q, want contains 'resolve cwd'", inst.Apps[0].Err)
	}
}

func TestStop_Running(t *testing.T) {
	rt, pf, st := newTestRuntime(t)
	w := &config.Workspace{Name: "x", Apps: []config.App{{Type: "local", Cmd: "echo"}}}
	inst, err := rt.Start(w, config.Source{Path: "/x"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := rt.Stop(inst.ID, true); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if len(pf.killCalls) == 0 {
		t.Errorf("no kill calls recorded")
	}
	got := st.Snapshot().Instances[inst.ID]
	if got.Status != state.StatusStopped {
		t.Errorf("Status = %s, want stopped", got.Status)
	}
}

func TestStop_NotFound(t *testing.T) {
	rt, _, _ := newTestRuntime(t)
	if err := rt.Stop("nope", true); err == nil {
		t.Errorf("expected error on unknown id")
	}
}

func TestStop_GracefulEscalates(t *testing.T) {
	rt, pf, st := newTestRuntime(t)
	pf.killFn = func(pid int, force bool) error {
		pf.MarkDead(pid)
		return nil
	}
	w := &config.Workspace{Name: "g", Apps: []config.App{{Type: "local", Cmd: "echo"}}}
	inst, err := rt.Start(w, config.Source{Path: "/x"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := rt.Stop(inst.ID, false); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if got := st.Snapshot().Instances[inst.ID]; got.Status != state.StatusStopped {
		t.Errorf("Status = %s, want stopped", got.Status)
	}
}

func TestStopAll(t *testing.T) {
	rt, _, _ := newTestRuntime(t)
	for i := 0; i < 3; i++ {
		w := &config.Workspace{
			Name: fmt.Sprintf("w%d", i),
			Apps: []config.App{{Type: "local", Cmd: "echo"}},
		}
		if _, err := rt.Start(w, config.Source{Path: "/x"}); err != nil {
			t.Fatalf("Start: %v", err)
		}
	}
	if err := rt.StopAll(true); err != nil {
		t.Fatalf("StopAll: %v", err)
	}
	for _, inst := range rt.Instances() {
		if inst.Status != state.StatusStopped {
			t.Errorf("instance %s still %s", inst.ID, inst.Status)
		}
	}
}

func TestReconcile(t *testing.T) {
	rt, pf, _ := newTestRuntime(t)
	w := &config.Workspace{
		Name: "rec",
		Apps: []config.App{
			{Type: "local", Cmd: "a"},
			{Type: "local", Cmd: "b"},
		},
	}
	inst, err := rt.Start(w, config.Source{Path: "/x"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	for _, app := range inst.Apps {
		pf.MarkDead(app.PID)
	}
	if err := rt.Reconcile(); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	got := rt.Instances()
	if len(got) != 1 {
		t.Fatalf("Instances len = %d", len(got))
	}
	if got[0].Status != state.StatusStopped {
		t.Errorf("Status = %s, want stopped", got[0].Status)
	}
	for _, app := range got[0].Apps {
		if app.Status != state.StatusStopped {
			t.Errorf("app %s = %s, want stopped", app.Cmd, app.Status)
		}
	}
}

func TestInstances_SortedByStartTime(t *testing.T) {
	rt, _, _ := newTestRuntime(t)
	var counter int64
	for i := 0; i < 3; i++ {
		w := &config.Workspace{
			Name: fmt.Sprintf("w%d", i),
			Apps: []config.App{{Type: "local", Cmd: "e"}},
		}
		atomic.AddInt64(&counter, 1)
		if _, err := rt.Start(w, config.Source{Path: "/x"}); err != nil {
			t.Fatalf("Start: %v", err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	insts := rt.Instances()
	for i := 1; i < len(insts); i++ {
		if insts[i-1].StartedAt.Before(insts[i].StartedAt) {
			t.Errorf("not sorted desc: insts[%d] < insts[%d]", i-1, i)
		}
	}
}

func TestPlatform_StartWithArgsAndEnv(t *testing.T) {
	rt, pf, _ := newTestRuntime(t)
	w := &config.Workspace{
		Name: "args",
		Apps: []config.App{{
			Type: "local",
			Cmd:  "code",
			Args: []string{"--wait", "/tmp/x"},
			Env:  map[string]string{"FOO": "bar"},
		}},
	}
	if _, err := rt.Start(w, config.Source{Path: "/x"}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(pf.launched) != 1 {
		t.Fatalf("launches = %d, want 1", len(pf.launched))
	}
	got := pf.launched[0]
	if got.Cmd != "code" {
		t.Errorf("Cmd = %q, want code", got.Cmd)
	}
	if len(got.Args) != 2 || got.Args[0] != "--wait" {
		t.Errorf("Args = %v, want [--wait /tmp/x]", got.Args)
	}
	found := false
	for _, e := range got.Env {
		if e == "FOO=bar" {
			found = true
		}
	}
	if !found {
		t.Errorf("Env missing FOO=bar: %v", got.Env)
	}
}
