// Package runtime manages the lifecycle of Workspace instances: it
// launches apps via the Platform, tracks PIDs in the state store,
// handles stop/recover, and resolves per-app paths and env.
package runtime

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/platform"
	"github.com/DerekCorniello/dia/internal/registry"
	"github.com/DerekCorniello/dia/internal/state"
)

// RecentLimit is the maximum number of workspace names kept in the
// "recent" list. Older entries are dropped on overflow.
const RecentLimit = 10

// GracePeriod is how long Stop waits between SIGTERM and SIGKILL when
// force is false.
const GracePeriod = 5 * time.Second

// Runtime launches, stops, and reconciles workspace instances. It is
// safe to call Start, Stop, and Reconcile concurrently from different
// goroutines; the underlying state.Store handles synchronization.
type Runtime struct {
	pf  platform.Platform
	st  *state.Store
	reg *registry.Registry
	log *slog.Logger
}

// Options for constructing a Runtime.
type Options struct {
	Platform platform.Platform
	Store    *state.Store
	Registry *registry.Registry
	Logger   *slog.Logger
}

// New returns a Runtime. Platform and Store are required; Registry
// falls back to registry.New(); Logger falls back to slog.Default().
func New(opts Options) *Runtime {
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	reg := opts.Registry
	if reg == nil {
		reg = registry.New()
	}
	return &Runtime{
		pf:  opts.Platform,
		st:  opts.Store,
		reg: reg,
		log: log,
	}
}

// Start launches every app in the workspace concurrently, persists the
// instance in state, and returns the populated Instance. Each app
// failure is recorded in the per-app AppProcess.Err rather than
// aborting the whole workspace, so a single broken app does not prevent
// the rest from coming up.
func (r *Runtime) Start(w *config.Workspace, src config.Source) (*state.Instance, error) {
	if w == nil {
		return nil, errors.New("runtime: nil workspace")
	}
	if len(w.Apps) == 0 {
		return nil, errors.New("runtime: workspace has no apps")
	}

	inst := state.Instance{
		ID:            newID(),
		WorkspaceName: w.Name,
		WorkspacePath: src.Path,
		StartedAt:     time.Now().UTC(),
		Status:        state.StatusRunning,
		Apps:          make([]state.AppProcess, 0, len(w.Apps)),
	}

	// Persist the placeholder before launching so a crash mid-launch
	// still leaves a record we can recover from.
	if err := r.st.Mutate(func(d *state.Data) {
		d.Instances[inst.ID] = inst
	}); err != nil {
		return nil, fmt.Errorf("save instance: %w", err)
	}

	type result struct {
		app  config.App
		proc state.AppProcess
	}

	results := make([]result, len(w.Apps))
	var wg sync.WaitGroup
	for i, app := range w.Apps {
		wg.Add(1)
		go func(i int, app config.App) {
			defer wg.Done()
			results[i] = result{app: app, proc: r.launchOne(app, w.Name, inst.ID)}
		}(i, app)
	}
	wg.Wait()

	// Preserve the input order so the UI list matches the YAML.
	apps := make([]state.AppProcess, len(w.Apps))
	for i, res := range results {
		apps[i] = res.proc
	}
	inst.Apps = apps

	// All apps failed -> the workspace did not actually start.
	allFailed := true
	for _, a := range apps {
		if a.Status == state.StatusRunning {
			allFailed = false
			break
		}
	}
	if allFailed {
		inst.Status = state.StatusCrashed
	}

	if err := r.st.Mutate(func(d *state.Data) {
		d.Instances[inst.ID] = inst
		d.Recent = pushRecent(d.Recent, w.Name, RecentLimit)
	}); err != nil {
		r.log.Warn("save instance after launch", "id", inst.ID, "error", err)
	}

	return &inst, nil
}

func (r *Runtime) launchOne(app config.App, workspaceName, instanceID string) state.AppProcess {
	out := state.AppProcess{Type: app.Type, Cmd: app.Cmd, Status: state.StatusRunning}

	action, err := r.reg.Resolve(app)
	if err != nil {
		out.Status = state.StatusCrashed
		out.Err = err.Error()
		r.log.Error("resolve", "workspace", workspaceName, "app", app.Type, "error", err)
		return out
	}

	switch action.Kind {
	case registry.ActionOpenURL:
		if action.URL == "" {
			out.Status = state.StatusCrashed
			out.Err = "open url: empty URL"
			return out
		}
		if err := r.pf.OpenURL(action.URL); err != nil {
			out.Status = state.StatusCrashed
			out.Err = "open url: " + err.Error()
			r.log.Error("open url", "workspace", workspaceName, "app", app.Type, "error", err)
			return out
		}
		// OpenURL hands off to the OS; there is no PID to track.
		// Record the URL in the Cmd field so the user can see what
		// was opened in the UI and the state file.
		out.Cmd = action.URL
		r.log.Info("opened url",
			"workspace", workspaceName,
			"instance", instanceID,
			"app", app.Type,
			"url", action.URL,
		)
		return out

	case registry.ActionLaunch:
		// fall through to launch handling below
	default:
		out.Status = state.StatusCrashed
		out.Err = fmt.Sprintf("unknown action kind: %v", action.Kind)
		return out
	}

	if action.Launch == nil {
		out.Status = state.StatusCrashed
		out.Err = "launch: nil opts"
		return out
	}

	cwd, err := resolvePath(action.Launch.Cwd)
	if err != nil {
		out.Status = state.StatusCrashed
		out.Err = "resolve cwd: " + err.Error()
		r.log.Error("resolve cwd", "workspace", workspaceName, "app", app.Type, "error", err)
		return out
	}

	handle, err := r.pf.Launch(platform.LaunchOpts{
		Cmd:  action.Launch.Cmd,
		Args: action.Launch.Args,
		Cwd:  cwd,
		Env:  action.Launch.Env,
	})
	if err != nil {
		out.Status = state.StatusCrashed
		out.Err = err.Error()
		r.log.Error("launch", "workspace", workspaceName, "app", app.Type, "error", err)
		return out
	}

	out.PID = handle.PID()
	r.log.Info("launched",
		"workspace", workspaceName,
		"instance", instanceID,
		"app", app.Type,
		"pid", out.PID,
	)
	return out
}

// Stop terminates every running app in the instance. With force=false
// a SIGTERM is sent and the runtime waits up to GracePeriod for the
// processes to exit before escalating to SIGKILL. Apps without a
// tracked PID (e.g. URL opens) are simply marked stopped.
func (r *Runtime) Stop(id string, force bool) error {
	var inst state.Instance
	ok := false
	if err := r.st.Mutate(func(d *state.Data) {
		inst, ok = d.Instances[id]
	}); err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("instance %q not found", id)
	}
	if inst.Status != state.StatusRunning {
		return nil
	}

	for i, app := range inst.Apps {
		if app.PID <= 0 || app.Status != state.StatusRunning {
			inst.Apps[i].Status = state.StatusStopped
			continue
		}
		if err := r.pf.Kill(app.PID, force); err != nil {
			r.log.Warn("kill", "instance", id, "pid", app.PID, "error", err)
		}
		inst.Apps[i].Status = state.StatusStopped
	}
	inst.Status = state.StatusStopped

	if !force {
		// Wait up to GracePeriod for processes to actually exit.
		deadline := time.Now().Add(GracePeriod)
		for _, app := range inst.Apps {
			if app.PID <= 0 {
				continue
			}
			for {
				running, _ := r.pf.IsRunning(app.PID)
				if !running {
					break
				}
				if time.Now().After(deadline) {
					r.log.Warn("grace expired, escalating", "pid", app.PID)
					_ = r.pf.Kill(app.PID, true)
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	return r.st.Mutate(func(d *state.Data) {
		d.Instances[id] = inst
	})
}

// StopAll terminates every running instance. Used on dia shutdown.
func (r *Runtime) StopAll(force bool) error {
	snap := r.st.Snapshot()
	var errs []error
	for id, inst := range snap.Instances {
		if inst.Status != state.StatusRunning {
			continue
		}
		if err := r.Stop(id, force); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

// StopAllWithIDs stops every running instance and returns the IDs that
// were stopped, even if some stops failed.
func (r *Runtime) StopAllWithIDs(force bool) ([]string, error) {
	snap := r.st.Snapshot()
	var ids []string
	var errs []error
	for id, inst := range snap.Instances {
		if inst.Status != state.StatusRunning {
			continue
		}
		ids = append(ids, id)
		if err := r.Stop(id, force); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", id, err))
		}
	}
	return ids, errors.Join(errs...)
}

// Reconcile walks the persisted state and drops PIDs that are no longer
// alive. Called on dia startup so stale entries from a crash or unclean
// shutdown do not accumulate.
func (r *Runtime) Reconcile() error {
	return r.st.Mutate(func(d *state.Data) {
		for id, inst := range d.Instances {
			if inst.Status != state.StatusRunning {
				continue
			}
			anyRunning := false
			for i, app := range inst.Apps {
				if app.PID <= 0 {
					continue
				}
				running, err := r.pf.IsRunning(app.PID)
				if err != nil {
					r.log.Warn("isrunning", "pid", app.PID, "error", err)
					continue
				}
				if running {
					anyRunning = true
				} else {
					inst.Apps[i].Status = state.StatusStopped
				}
			}
			if !anyRunning {
				inst.Status = state.StatusStopped
			}
			d.Instances[id] = inst
		}
	})
}

// pushRecent inserts name at the front of recent, de-duplicates, and
// caps the slice at limit. The entry's Count is incremented if name is
// already present. Returns a new slice.
func pushRecent(recent []state.RecentEntry, name string, limit int) []state.RecentEntry {
	if name == "" {
		return recent
	}
	count := 1
	for _, e := range recent {
		if e.Name == name {
			count = e.Count + 1
			break
		}
	}
	out := make([]state.RecentEntry, 0, limit)
	out = append(out, state.RecentEntry{Name: name, Count: count})
	for _, e := range recent {
		if e.Name == name {
			continue
		}
		out = append(out, e)
		if len(out) == limit {
			break
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// Instances returns a sorted snapshot of all known instances, with the
// most recently started first.
func (r *Runtime) Instances() []state.Instance {
	snap := r.st.Snapshot().Instances
	out := make([]state.Instance, 0, len(snap))
	for _, inst := range snap {
		out = append(out, inst)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAt.After(out[j].StartedAt)
	})
	return out
}
