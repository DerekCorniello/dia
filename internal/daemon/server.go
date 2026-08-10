package daemon

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/platform"
	"github.com/DerekCorniello/dia/internal/plugins"
	"github.com/DerekCorniello/dia/internal/registry"
	"github.com/DerekCorniello/dia/internal/state"
	"github.com/DerekCorniello/dia/internal/version"

	"github.com/DerekCorniello/dia/internal/runtime"
)

// Server is the session daemon. It owns the runtime, supervises
// everything it drives, and accepts commands over a unix socket /
// named pipe. Exactly one server runs per state dir; the client
// spawns it on first use and detaches when done.
type Server struct {
	store    *state.Store
	rt       *runtime.Runtime
	perf     platform.Platform
	reg      *registry.Registry
	pmgr     *plugins.Manager
	log      *slog.Logger
	stateDir string

	ln net.Listener
	// mu serializes dispatch: handlers mutate the same store the
	// runtime supervises, and the resolver manager is not safe for
	// concurrent use.
	mu        sync.Mutex
	closeOnce sync.Once
}

// Options for constructing a server.
type Options struct {
	StateDir string
	Store    *state.Store
	Registry *registry.Registry
	Platform platform.Platform
	Logger   *slog.Logger
}

// NewServer wires up the daemon's own runtime, registry, plugin
// resolver, and store. It does not bind the socket; call Serve.
func NewServer(opts Options) (*Server, error) {
	stateDir := opts.StateDir
	if stateDir == "" {
		var err error
		stateDir, err = state.ResolveStateDir()
		if err != nil {
			return nil, err
		}
	}
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	st := opts.Store
	if st == nil {
		var err error
		st, err = state.OpenAt(filepath.Join(stateDir, state.StateFile))
		if err != nil {
			return nil, fmt.Errorf("open state: %w", err)
		}
	}
	pf := opts.Platform
	if pf == nil {
		pf = platform.New()
	}
	reg := opts.Registry
	if reg == nil {
		reg = registry.New()
	}
	pmgr := newResolverManager(log, st, reg, stateDir)
	rt := runtime.New(runtime.Options{
		Platform: pf,
		Store:    st,
		Registry: reg,
		Logger:   log,
	})
	return &Server{
		store:    st,
		rt:       rt,
		perf:     pf,
		reg:      reg,
		pmgr:     pmgr,
		log:      log,
		stateDir: stateDir,
	}, nil
}

// ResolverManager re-syncs plugin-claimed app types onto the registry
// from the current persisted grants. This is what lets a workspace
// with a plugin app type start under the daemon, and rediscovering is
// cheap enough to run before each start so grant changes apply
// immediately, not at the next daemon restart.
func newResolverManager(log *slog.Logger, st *state.Store, reg *registry.Registry, stateDir string) *plugins.Manager {
	pmgr, err := plugins.NewManager(plugins.GlobalPluginsDir(stateDir), &nullResolverHost{})
	if err != nil {
		log.Warn("init plugin manager", "error", err)
		return nil
	}
	if cwd, _ := os.Getwd(); cwd != "" {
		pmgr.SetLocalDir(cwd)
	}
	if err := pmgr.Discover(); err != nil {
		log.Warn("discover plugins", "error", err)
		return pmgr
	}
	syncPluginGrants(log, st, reg, pmgr)
	return pmgr
}

// syncPluginGrants applies persisted grants and registers claimed app
// types. Capabilities are never granted by being requested; the
// stored grants are intersected against manifests inside
// ApplyPersistedGrants.
func syncPluginGrants(log *slog.Logger, st *state.Store, reg *registry.Registry, pmgr *plugins.Manager) {
	snap := st.Snapshot()
	grants := make(map[string][]string, len(snap.Plugins))
	for id, ps := range snap.Plugins {
		grants[id] = ps.GrantedCapabilities
	}
	pmgr.ApplyPersistedGrants(grants)
	for _, err := range registry.SyncPluginTypes(reg, pmgr) {
		log.Warn("register plugin app type", "error", err)
	}
	for _, c := range pmgr.AppTypeConflicts() {
		log.Warn("app type claimed by more than one plugin",
			"type", c.Type, "using", c.Winner, "ignored", c.Loser)
	}
}

// Serve binds the daemon socket and blocks handling connections until
// the listener is closed (Shutdown verb or Close).
func (s *Server) Serve() error {
	path := socketPath(s.stateDir)
	ln, err := listenSocket(path)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", path, err)
	}
	s.ln = ln
	s.log.Info("daemon serving", "socket", path)
	defer removeSocket(path)

	for {
		conn, err := ln.Accept()
		if err != nil {
			return nil
		}
		go s.handle(conn)
	}
}

// Close stops all supervised workspaces and closes the listener.
func (s *Server) Close() {
	s.closeOnce.Do(func() {
		if err := s.rt.StopAll(true); err != nil {
			s.log.Warn("stop all on shutdown", "error", err)
		}
		if s.ln != nil {
			_ = s.ln.Close()
		}
	})
}

// SocketPath exposes the daemon socket path for a state dir.
func SocketPath(stateDir string) string { return socketPath(stateDir) }

// handle serves a single client connection.
func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	for {
		line, err := readLine(conn)
		if err != nil {
			return
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			_ = writeResponse(conn, response{Error: "bad request"})
			continue
		}
		result, err := s.dispatch(req)
		if err != nil {
			_ = writeResponse(conn, response{ID: req.ID, Error: err.Error()})
		} else {
			_ = writeResponse(conn, response{ID: req.ID, Result: result})
		}
		if req.Method == MethodShutdown {
			// The response above must reach the client before the
			// daemon tears down; Close stops supervision and the
			// listener, and Serve returns so the process exits.
			s.Close()
			return
		}
	}
}

// dispatch routes a request to a handler under the server mutex.
func (s *Server) dispatch(req request) (json.RawMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch req.Method {
	case MethodStart:
		return s.doStart(req.Params)
	case MethodStop:
		return s.doStop(req.Params)
	case MethodRestart:
		return s.doRestart(req.Params)
	case MethodStopAll:
		return s.doStopAll(req.Params)
	case MethodList, MethodStatus:
		return s.doList()
	case MethodReconcile:
		return s.doReconcile()
	case MethodVersion:
		return s.doVersion()
	case MethodShutdown:
		return s.doShutdown()
	default:
		return nil, fmt.Errorf("unknown method %q", req.Method)
	}
}

// verbParams decodes request parameters into out. An empty body means
// no parameters (nil out is acceptable).
func verbParams(raw json.RawMessage, out any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

// StartParams is the payload for MethodStart and MethodRestart.
type StartParams struct {
	// Name is the workspace to start. It resolves through
	// config.Discover unless Path is provided.
	Name string `json:"name"`
	// Path optionally overrides discovery: load the workspace YAML at
	// this absolute path directly. The client uses this to preserve
	// cwd-origin workspace discovery, which the daemon cannot repeat.
	Path string `json:"path,omitempty"`
	// Cwd overrides the working directory for every app without its
	// own cwd, mirroring the CLI --cwd flag.
	Cwd string `json:"cwd,omitempty"`
}

// StartReply is the payload returned by start/restart.
type StartReply struct {
	Instance *state.Instance `json:"instance"`
	// Attached is true when the workspace was already running and the
	// request was an idempotent attach rather than a fresh launch.
	Attached bool `json:"attached"`
}

func (s *Server) doStart(raw json.RawMessage) (json.RawMessage, error) {
	var p StartParams
	if err := verbParams(raw, &p); err != nil {
		return nil, fmt.Errorf("start: bad params: %w", err)
	}
	inst, attached, err := s.start(p)
	if err != nil {
		return nil, err
	}
	return jsonify(StartReply{Instance: inst, Attached: attached})
}

// start is the idempotent session entrance. Workspaces are keyed by
// name: starting a name that already has a running instance attaches
// to it (tmux server semantics) instead of launching a duplicate.
func (s *Server) start(p StartParams) (*state.Instance, bool, error) {
	if p.Name == "" {
		return nil, false, fmt.Errorf("workspace name is required")
	}
	if existing := s.runningInstanceByName(p.Name); existing != nil {
		return existing, true, nil
	}

	w, src, err := s.resolveWorkspace(p)
	if err != nil {
		return nil, false, err
	}
	if p.Cwd != "" {
		for i := range w.Apps {
			if w.Apps[i].Cwd == "" {
				w.Apps[i].Cwd = p.Cwd
			}
		}
	}
	// Refresh claimed app types before launching: grants may have
	// changed from another client since the server started, and the
	// launch below resolves them through the registry.
	if s.pmgr != nil {
		syncPluginGrants(s.log, s.store, s.reg, s.pmgr)
	}
	if err := s.enableWorkspacePlugins(w); err != nil {
		s.log.Warn("enable workspace plugins", "workspace", w.Name, "error", err)
	}

	inst, err := s.rt.Start(w, src)
	if err != nil {
		return nil, false, err
	}
	if len(w.Plugins) > 0 {
		ids := make([]string, 0, len(w.Plugins))
		for _, ref := range w.Plugins {
			ids = append(ids, ref.ID)
		}
		if err := s.store.Mutate(func(d *state.Data) {
			i := d.Instances[inst.ID]
			i.Plugins = ids
			d.Instances[inst.ID] = i
		}); err != nil {
			s.log.Warn("mutate instance plugins", "error", err)
		}
	}
	if s.pmgr != nil {
		for _, ref := range w.Plugins {
			if loaded, ok := s.pmgr.Loaded(ref.ID); ok && loaded.Manifest != nil && loaded.Manifest.UI.Type == "window" {
				pid, err := s.spawnPluginWindow(ref.ID, w.Name, src.Path)
				if err != nil {
					s.log.Warn("spawn plugin window", "id", ref.ID, "error", err)
					continue
				}
				if err := s.store.Mutate(func(d *state.Data) {
					i := d.Instances[inst.ID]
					i.PluginPIDs = append(i.PluginPIDs, pid)
					d.Instances[inst.ID] = i
				}); err != nil {
					s.log.Warn("mutate instance plugin pids", "error", err)
				}
			}
		}
	}
	return inst, false, nil
}

// resolveWorkspace loads the workspace config, honoring the Path
// override for cwd-dependent discovery done on the client side.
func (s *Server) resolveWorkspace(p StartParams) (*config.Workspace, config.Source, error) {
	if p.Path != "" {
		w, err := config.Load(p.Path)
		if err != nil {
			return nil, config.Source{}, err
		}
		return w, config.Source{Workspace: w, Path: p.Path}, nil
	}
	cwd, _ := os.Getwd()
	all, err := config.Discover(config.DiscoverOptions{
		GlobalDir: config.DefaultGlobalDir(),
		Roots:     s.store.Snapshot().Roots,
		CWD:       cwd,
	})
	if err != nil {
		return nil, config.Source{}, err
	}
	for _, so := range all {
		if so.Workspace.Name == p.Name {
			return so.Workspace, so, nil
		}
	}
	return nil, config.Source{}, &NoWorkspaceError{Name: p.Name}
}

// NoWorkspaceError is returned when a workspace name does not resolve.
type NoWorkspaceError struct{ Name string }

func (e *NoWorkspaceError) Error() string { return "workspace " + e.Name + " not found" }

// runningInstanceByName returns the running instance for a workspace
// name, or nil when none is running.
func (s *Server) runningInstanceByName(name string) *state.Instance {
	for _, inst := range s.rt.Instances() {
		if inst.WorkspaceName == name && inst.Status == state.StatusRunning {
			cp := inst
			return &cp
		}
	}
	return nil
}

// StopParams is the payload for MethodStop.
type StopParams struct {
	Name  string `json:"name"`
	Force bool   `json:"force"`
}

func (s *Server) doStop(raw json.RawMessage) (json.RawMessage, error) {
	var p StopParams
	if err := verbParams(raw, &p); err != nil {
		return nil, fmt.Errorf("stop: bad params: %w", err)
	}
	ids := []string{}
	for _, inst := range s.rt.Instances() {
		if inst.WorkspaceName != p.Name || inst.Status != state.StatusRunning {
			continue
		}
		if err := s.stopInstance(inst.ID, p.Force); err != nil {
			return nil, fmt.Errorf("stop %s: %w", inst.ID, err)
		}
		ids = append(ids, inst.ID)
	}
	if len(ids) == 0 {
		return nil, &NoWorkspaceError{Name: p.Name}
	}
	return jsonify(map[string]any{"stopped": ids})
}

func (s *Server) doStopAll(raw json.RawMessage) (json.RawMessage, error) {
	var p struct {
		Force bool `json:"force"`
	}
	if err := verbParams(raw, &p); err != nil {
		return nil, fmt.Errorf("stop-all: bad params: %w", err)
	}
	ids, err := s.rt.StopAllWithIDs(p.Force)
	if err != nil {
		return nil, err
	}
	return jsonify(map[string]any{"stopped": ids})
}

// doRestart stops any running instance and starts a fresh one.
func (s *Server) doRestart(raw json.RawMessage) (json.RawMessage, error) {
	var p StartParams
	if err := verbParams(raw, &p); err != nil {
		return nil, fmt.Errorf("restart: bad params: %w", err)
	}
	if existing := s.runningInstanceByName(p.Name); existing != nil {
		if err := s.stopInstance(existing.ID, true); err != nil {
			return nil, err
		}
	}
	inst, attached, err := s.start(p)
	if err != nil {
		return nil, err
	}
	return jsonify(StartReply{Instance: inst, Attached: attached})
}

// stopInstance stops a running instance, including its plugin windows.
func (s *Server) stopInstance(id string, force bool) error {
	snap := s.store.Snapshot()
	if inst, ok := snap.Instances[id]; ok {
		for _, ppid := range inst.PluginPIDs {
			if ppid > 0 {
				_ = s.perf.Kill(ppid, true)
			}
		}
	}
	return s.rt.Stop(id, force)
}

func (s *Server) doList() (json.RawMessage, error) {
	return jsonify(s.rt.Instances())
}

func (s *Server) doReconcile() (json.RawMessage, error) {
	before := len(s.rt.Instances())
	runningBefore := runningCount(s.store)
	if err := s.rt.Reconcile(); err != nil {
		return nil, err
	}
	runningAfter := runningCount(s.store)
	return jsonify(map[string]int{
		"reconciled": runningBefore - runningAfter,
		"remaining":  runningAfter,
		"total":      before,
	})
}

func runningCount(st *state.Store) int {
	n := 0
	for _, inst := range st.Snapshot().Instances {
		if inst.Status == state.StatusRunning {
			n++
		}
	}
	return n
}

func (s *Server) doVersion() (json.RawMessage, error) {
	return jsonify(map[string]string{"version": version.Version})
}

// doShutdown acknowledges the shutdown request but does not close:
// the client needs the response before the process dies. The caller
// closes after writing it.
func (s *Server) doShutdown() (json.RawMessage, error) {
	return jsonify(map[string]string{"stopped": "true"})
}

// spawnPluginWindow starts a detached `dia --plugin-window` process
// to show a plugin's panel, matching the GUI's behavior so
// daemon-launched workspaces get the same panels.
func (s *Server) spawnPluginWindow(pluginID, workspaceName, workspacePath string) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("get executable: %w", err)
	}
	handle, err := s.perf.Launch(platform.LaunchOpts{
		Cmd: exe,
		Args: []string{
			"--plugin-window=" + pluginID,
			"--workspace=" + workspaceName,
			"--workspace-path=" + workspacePath,
		},
	})
	if err != nil {
		return 0, err
	}
	return handle.PID(), nil
}

// enableWorkspacePlugins persists workspace plugin config and derived
// grants for each referenced plugin and loads it into the daemon's
// resolver. Capabilities are never granted by being enabled; grants
// are intersected against the manifest from the user-approved list.
func (s *Server) enableWorkspacePlugins(w *config.Workspace) error {
	if s.pmgr == nil || len(w.Plugins) == 0 {
		return nil
	}
	for _, ref := range w.Plugins {
		loaded, ok := s.pmgr.Loaded(ref.ID)
		if !ok {
			continue
		}
		if loaded.Manifest == nil {
			return fmt.Errorf("plugin %q has no valid manifest: %s", ref.ID, loaded.LastError)
		}
		granted := workspacePluginGrants(s.store, loaded.Manifest, ref.ID)
		if err := s.pmgr.SetConfig(ref.ID, ref.Config); err != nil {
			s.log.Warn("set plugin config", "id", ref.ID, "error", err)
		}
		if err := s.pmgr.EnableWithGrants(ref.ID, granted); err != nil {
			return err
		}
		if err := s.store.Mutate(func(d *state.Data) {
			if d.Plugins == nil {
				d.Plugins = map[string]state.PluginState{}
			}
			prev := d.Plugins[ref.ID]
			prev.GrantedCapabilities = granted
			prev.Config = ref.Config
			d.Plugins[ref.ID] = prev
		}); err != nil {
			return err
		}
	}
	return nil
}

// workspacePluginGrants derives the granted capability set for a
// plugin from the persisted state, intersected with the manifest. A
// nil store yields the read-only defaults.
func workspacePluginGrants(st *state.Store, m *plugins.Manifest, id string) []string {
	granted := plugins.DefaultReadCapabilities()
	if st != nil {
		if ps, ok := st.Snapshot().Plugins[id]; ok && ps.GrantedCapabilities != nil {
			granted = ps.GrantedCapabilities
		}
	}
	return plugins.GrantCapabilities(m.Capabilities, granted)
}

// jsonify marshals a value to a result payload.
func jsonify(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}
