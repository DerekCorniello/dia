package runtime

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/platform"
	"github.com/DerekCorniello/dia/internal/registry"
	"github.com/DerekCorniello/dia/internal/state"
)

// HookTimeout bounds a single hook command. Hooks block the start or
// stop they belong to, so an unbounded one would hang the GUI with no
// way out. Long-running processes belong in `apps`, not hooks.
const HookTimeout = 2 * time.Minute

// HookError identifies which hook failed and why. The output is the
// command's combined stdout/stderr, which is usually the only thing
// that tells the user what actually went wrong.
type HookError struct {
	Phase  string
	Index  int
	Cmd    string
	Output string
	Err    error
}

func (e *HookError) Error() string {
	msg := fmt.Sprintf("hook %s[%d] %q: %v", e.Phase, e.Index, e.Cmd, e.Err)
	if e.Output != "" {
		msg += "\n" + e.Output
	}
	return msg
}

func (e *HookError) Unwrap() error { return e.Err }

// runHooks executes one phase's commands in order, stopping at the
// first failure. cwd is the directory the workspace config was loaded
// from, so a hook like "docker compose up -d" resolves relative paths
// the way the user expects.
//
// The returned error is a *HookError; callers decide whether it is
// fatal. It is for pre_start (the apps would come up against
// infrastructure that is not there) and advisory for the rest, where
// the work being wrapped has already happened.
func (r *Runtime) runHooks(phase string, cmds []string, cwd string) error {
	for i, raw := range cmds {
		program, args, err := registry.SplitProgram(raw)
		if err != nil {
			return &HookError{Phase: phase, Index: i, Cmd: raw, Err: err}
		}
		r.log.Info("hook", "phase", phase, "index", i, "cmd", raw)
		out, err := r.pf.Run(platform.LaunchOpts{
			Cmd:  program,
			Args: args,
			Cwd:  cwd,
		}, HookTimeout)
		if err != nil {
			return &HookError{Phase: phase, Index: i, Cmd: raw, Output: out, Err: err}
		}
	}
	return nil
}

// hookCwd is the directory hooks run in: the one holding the
// workspace config. For a project-local .dia.yaml that is the repo
// root, which is what a hook like "docker compose up -d" needs. For a
// global workspace it is the workspaces dir, so those hooks should
// use absolute paths.
func hookCwd(configPath string) string {
	if configPath == "" {
		return ""
	}
	return filepath.Dir(configPath)
}

// workspaceHooks reloads the workspace a running instance came from so
// stop-side hooks can be read. The instance only records the config
// path, and re-reading it is preferable to snapshotting the hooks into
// state, where they would go stale against an edited config. A missing
// or now-invalid config is not an error: the instance still has to be
// stoppable.
func (r *Runtime) workspaceHooks(inst state.Instance) (*config.Workspace, string) {
	if inst.WorkspacePath == "" {
		return nil, ""
	}
	w, err := config.Load(inst.WorkspacePath)
	if err != nil {
		r.log.Warn("load workspace for stop hooks",
			"instance", inst.ID, "path", inst.WorkspacePath, "error", err)
		return nil, ""
	}
	return w, hookCwd(inst.WorkspacePath)
}

// hookPhase returns the commands for a named phase, or nil when the
// workspace declares no hooks at all.
func hookPhase(w *config.Workspace, name string) []string {
	if w == nil || w.Hooks == nil {
		return nil
	}
	for _, p := range w.Hooks.Phases() {
		if p.Name == name {
			return p.Cmds
		}
	}
	return nil
}
