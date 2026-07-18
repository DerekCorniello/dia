package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/platform"
	"github.com/DerekCorniello/dia/internal/state"
)

// writeWorkspaceFile writes a workspace YAML to a temp dir and returns
// its path, so stop-side hooks (which reload the config from disk) have
// something real to read.
func writeWorkspaceFile(t *testing.T, yaml string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ws.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write workspace: %v", err)
	}
	return path
}

func hookWorkspace(pre, post []string) *config.Workspace {
	return &config.Workspace{
		Name:  "hooked",
		Apps:  []config.App{{Type: "local", Cmd: "app"}},
		Hooks: &config.Hooks{PreStart: pre, PostStart: post},
	}
}

func TestStart_RunsPreAndPostStartHooksInOrder(t *testing.T) {
	rt, pf, _ := newTestRuntime(t)
	w := hookWorkspace([]string{"before-one", "before-two"}, []string{"after"})

	if _, err := rt.Start(w, config.Source{}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	want := []string{"before-one", "before-two", "after"}
	got := pf.ranCmds()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("hooks ran %v, want %v", got, want)
	}
}

func TestStart_PreStartHookIsFatalAndLaunchesNothing(t *testing.T) {
	rt, pf, st := newTestRuntime(t)
	pf.runFn = func(opts platform.LaunchOpts) (string, error) {
		return "boom output", errors.New("exit status 1")
	}
	w := hookWorkspace([]string{"failing-hook"}, nil)

	_, err := rt.Start(w, config.Source{})
	if err == nil {
		t.Fatal("expected Start to fail when pre_start fails")
	}

	var he *HookError
	if !errors.As(err, &he) {
		t.Fatalf("error %v (%T), want a *HookError", err, err)
	}
	if he.Phase != "pre_start" {
		t.Errorf("Phase = %q, want pre_start", he.Phase)
	}
	// The command output is the only thing that explains the failure,
	// so it has to survive into the message the user sees.
	if !strings.Contains(he.Error(), "boom output") {
		t.Errorf("error %q does not include the hook output", he.Error())
	}

	if len(pf.launched) != 0 {
		t.Errorf("launched %d apps, want 0 after a failed pre_start", len(pf.launched))
	}
	if n := len(st.Snapshot().Instances); n != 0 {
		t.Errorf("persisted %d instances, want 0 after a failed pre_start", n)
	}
}

func TestStart_PostStartHookFailureDoesNotFailTheStart(t *testing.T) {
	rt, pf, _ := newTestRuntime(t)
	pf.runFn = func(opts platform.LaunchOpts) (string, error) {
		if opts.Cmd == "after" {
			return "", errors.New("exit status 1")
		}
		return "", nil
	}
	w := hookWorkspace(nil, []string{"after"})

	inst, err := rt.Start(w, config.Source{})
	if err != nil {
		t.Fatalf("Start should survive a failing post_start hook, got %v", err)
	}
	if inst.Status != state.StatusRunning {
		t.Errorf("Status = %s, want running", inst.Status)
	}
}

func TestStart_HookStopsAtFirstFailure(t *testing.T) {
	rt, pf, _ := newTestRuntime(t)
	pf.runFn = func(opts platform.LaunchOpts) (string, error) {
		if opts.Cmd == "first" {
			return "", errors.New("exit status 1")
		}
		return "", nil
	}
	w := hookWorkspace([]string{"first", "second"}, nil)

	if _, err := rt.Start(w, config.Source{}); err == nil {
		t.Fatal("expected Start to fail")
	}
	if got := pf.ranCmds(); len(got) != 1 || got[0] != "first" {
		t.Errorf("ran %v, want only [first]", got)
	}
}

func TestStart_HooksRunInTheConfigDirectory(t *testing.T) {
	rt, pf, _ := newTestRuntime(t)
	path := writeWorkspaceFile(t, "name: hooked\napps:\n  - type: local\n    cmd: app\n")
	w := hookWorkspace([]string{"hook"}, nil)

	if _, err := rt.Start(w, config.Source{Path: path}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(pf.ran) != 1 {
		t.Fatalf("ran %d hooks, want 1", len(pf.ran))
	}
	if want := filepath.Dir(path); pf.ran[0].Cwd != want {
		t.Errorf("hook Cwd = %q, want %q", pf.ran[0].Cwd, want)
	}
}

func TestStart_HookArgsAreSplitWithQuoting(t *testing.T) {
	rt, pf, _ := newTestRuntime(t)
	w := hookWorkspace([]string{`docker compose -f "my compose.yml" up -d`}, nil)

	if _, err := rt.Start(w, config.Source{}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	got := pf.ran[0]
	if got.Cmd != "docker" {
		t.Errorf("Cmd = %q, want docker", got.Cmd)
	}
	want := []string{"compose", "-f", "my compose.yml", "up", "-d"}
	if strings.Join(got.Args, "|") != strings.Join(want, "|") {
		t.Errorf("Args = %v, want %v", got.Args, want)
	}
}

func TestStart_NoHooksRunsNothing(t *testing.T) {
	rt, pf, _ := newTestRuntime(t)
	w := &config.Workspace{Name: "plain", Apps: []config.App{{Type: "local", Cmd: "app"}}}

	if _, err := rt.Start(w, config.Source{}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if got := pf.ranCmds(); len(got) != 0 {
		t.Errorf("ran %v, want no hooks", got)
	}
}

func TestStop_RunsPreAndPostStopHooks(t *testing.T) {
	rt, pf, _ := newTestRuntime(t)
	path := writeWorkspaceFile(t, `name: hooked
apps:
  - type: local
    cmd: app
hooks:
  pre_stop:
    - down-first
  post_stop:
    - clean-after
`)
	w := &config.Workspace{Name: "hooked", Apps: []config.App{{Type: "local", Cmd: "app"}}}

	inst, err := rt.Start(w, config.Source{Path: path})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := rt.Stop(inst.ID, false); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	want := []string{"down-first", "clean-after"}
	if got := pf.ranCmds(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("hooks ran %v, want %v", got, want)
	}
}

func TestStop_ForceSkipsHooks(t *testing.T) {
	rt, pf, _ := newTestRuntime(t)
	path := writeWorkspaceFile(t, `name: hooked
apps:
  - type: local
    cmd: app
hooks:
  pre_stop:
    - down-first
`)
	w := &config.Workspace{Name: "hooked", Apps: []config.App{{Type: "local", Cmd: "app"}}}

	inst, err := rt.Start(w, config.Source{Path: path})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := rt.Stop(inst.ID, true); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if got := pf.ranCmds(); len(got) != 0 {
		t.Errorf("ran %v, want no hooks on a forced stop", got)
	}
}

func TestStop_FailingStopHookStillStopsTheInstance(t *testing.T) {
	rt, pf, st := newTestRuntime(t)
	path := writeWorkspaceFile(t, `name: hooked
apps:
  - type: local
    cmd: app
hooks:
  pre_stop:
    - failing
`)
	w := &config.Workspace{Name: "hooked", Apps: []config.App{{Type: "local", Cmd: "app"}}}

	inst, err := rt.Start(w, config.Source{Path: path})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	pf.runFn = func(opts platform.LaunchOpts) (string, error) {
		return "", errors.New("exit status 1")
	}

	// A cleanup command that fails must not leave the user unable to
	// shut the workspace down from dia.
	if err := rt.Stop(inst.ID, false); err != nil {
		t.Fatalf("Stop should survive a failing pre_stop hook, got %v", err)
	}
	if got := st.Snapshot().Instances[inst.ID].Status; got != state.StatusStopped {
		t.Errorf("Status = %s, want stopped", got)
	}
}

func TestStop_MissingConfigSkipsHooksWithoutFailing(t *testing.T) {
	rt, _, st := newTestRuntime(t)
	w := &config.Workspace{Name: "gone", Apps: []config.App{{Type: "local", Cmd: "app"}}}

	inst, err := rt.Start(w, config.Source{Path: filepath.Join(t.TempDir(), "deleted.yaml")})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := rt.Stop(inst.ID, false); err != nil {
		t.Fatalf("Stop with a deleted config should still work, got %v", err)
	}
	if got := st.Snapshot().Instances[inst.ID].Status; got != state.StatusStopped {
		t.Errorf("Status = %s, want stopped", got)
	}
}

func TestRunHooks_UnparseableCommandIsReported(t *testing.T) {
	rt, pf, _ := newTestRuntime(t)
	w := hookWorkspace([]string{`echo "unterminated`}, nil)

	_, err := rt.Start(w, config.Source{})
	if err == nil {
		t.Fatal("expected an error for an unterminated quote")
	}
	if len(pf.ran) != 0 {
		t.Errorf("ran %d commands, want 0 when the hook cannot be parsed", len(pf.ran))
	}
}
