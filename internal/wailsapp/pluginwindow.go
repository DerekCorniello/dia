package wailsapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/plugins"
	"github.com/DerekCorniello/dia/internal/state"
)

// PluginWindowApp is the wails binding for a plugin's new-window
// process. It exposes a single DiaCall method that the plugin's
// HTML/JS can use to invoke any dia.* method or any exported
// function on the plugin's module.exports. The plugin window is
// spawned by the main dia binary (or by the
// `dia --plugin-window=<id>` flag) and runs in its own process; it
// has no workspace runtime and can only perform read-only
// operations against the shared state file.
type PluginWindowApp struct {
	ctx       context.Context
	manifest  *plugins.Manifest
	pluginDir string
	host      *pluginWindowHost
	runtime   *plugins.Runtime
}

// DiaCall dispatches a single method call. method can be a dia.*
// bridge method (e.g. "listWorkspaces") or any function exported on
// the plugin's module.exports. argsJSON is a JSON-encoded array of
// positional arguments. The return value is JSON-encoded.
func (a *PluginWindowApp) DiaCall(method string, argsJSON string) (string, error) {
	if method == "" {
		return "", errors.New("method is required")
	}
	var args []any
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("parse args: %w", err)
		}
	}
	var (
		v   any
		err error
	)
	if a.host != nil {
		v, err = a.host.dispatch(method, args)
	}
	if err != nil && errors.Is(err, errHostUnhandled) && a.runtime != nil {
		v, err = a.runtime.Call(context.Background(), method, args)
	}
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// RunPluginWindow starts a wails app that hosts a single plugin's
// panel in a new window. It is invoked by main.go when the binary
// is launched with --plugin-window=<id>. When workspaceName is set
// the plugin's config from that workspace's PluginRef is loaded and
// passed to the goja runtime via dia.getConfig().
func RunPluginWindow(id string, workspaceName string) error {
	if id == "" {
		return errors.New("plugin id is required")
	}
	dir, err := state.ResolveStateDir()
	if err != nil {
		return fmt.Errorf("resolve state dir: %w", err)
	}
	full, err := resolvePluginDir(id, dir)
	if err != nil {
		return err
	}
	manifest, err := plugins.LoadManifest(full)
	if err != nil {
		return err
	}
	if manifest.UI.Type != "window" {
		return fmt.Errorf("plugin %q is not a window plugin (ui.type=%q)", id, manifest.UI.Type)
	}
	// Load workspace-scoped config if a workspace was specified.
	var cfg map[string]any
	if workspaceName != "" {
		ws, err := loadWorkspacePluginConfig(workspaceName, id)
		if err != nil {
			return fmt.Errorf("load plugin config: %w", err)
		}
		cfg = ws
	}
	host, err := newPluginWindowHost(dir)
	if err != nil {
		return err
	}

	grants := plugins.GrantCapabilities(manifest.Capabilities, plugins.DefaultReadCapabilities())
	rt, err := plugins.NewRuntime(manifest, full, host, grants, cfg)
	if err != nil {
		return err
	}
	if err := rt.Load(); err != nil {
		return err
	}
	app := &PluginWindowApp{
		manifest:  manifest,
		pluginDir: full,
		host:      host,
		runtime:   rt,
	}
	width := manifest.UI.Width
	if width <= 0 {
		width = 900
	}
	height := manifest.UI.Height
	if height <= 0 {
		height = 700
	}
	handler := &pluginAssetHandler{pluginDir: full, manifest: manifest}
	return wails.Run(&options.App{
		Title:  manifest.UI.Title,
		Width:  width,
		Height: height,
		AssetServer: &assetserver.Options{
			Handler: handler,
		},
		BackgroundColour: &options.RGBA{R: 30, G: 30, B: 46, A: 1},
		OnStartup:        func(ctx context.Context) { app.ctx = ctx },
		Bind: []interface{}{
			app,
		},
	})
}

// resolvePluginDir finds the on-disk directory for a plugin id,
// preferring the project-local copy over the global one.
func resolvePluginDir(id, stateDir string) (string, error) {
	global := filepath.Join(plugins.GlobalPluginsDir(stateDir), id)
	if cwd, _ := os.Getwd(); cwd != "" {
		local := filepath.Join(plugins.LocalPluginsDir(cwd), id)
		if _, err := os.Stat(filepath.Join(local, "plugin.json")); err == nil {
			return local, nil
		}
	}
	if _, err := os.Stat(filepath.Join(global, "plugin.json")); err == nil {
		return global, nil
	}
	return "", fmt.Errorf("plugin %q not found in %s or %s", id, global, filepath.Dir(global))
}

// pluginAssetHandler serves the plugin's panel files: panel/index.html
// (or a generated default), panel/panel.js (or the path in
// ui.entry), panel/styles.css, and a generated dia.js that wires
// window.dia to the wails binding. All other requests 404.
type pluginAssetHandler struct {
	pluginDir string
	manifest  *plugins.Manifest
}

func (h *pluginAssetHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := r.URL.Path
	switch path {
	case "/":
		h.serveIndex(w, r)
	case "/panel.js":
		h.servePanelJS(w, r)
	case "/styles.css":
		h.serveStyles(w, r)
	case "/dia.js":
		h.serveDia(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *pluginAssetHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	custom := filepath.Join(h.pluginDir, plugins.DefaultPanelHTML)
	if data, err := os.ReadFile(custom); err == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(generatedPanelHTML(h.manifest)))
}

func (h *pluginAssetHandler) servePanelJS(w http.ResponseWriter, r *http.Request) {
	rel := h.manifest.PanelJSPath()
	if rel == "" {
		rel = plugins.DefaultPanelJS
	}
	full := filepath.Join(h.pluginDir, filepath.Clean(rel))
	data, err := os.ReadFile(full)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, "panel.js not found: "+rel, http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	_, _ = w.Write(data)
}

func (h *pluginAssetHandler) serveStyles(w http.ResponseWriter, r *http.Request) {
	full := filepath.Join(h.pluginDir, plugins.DefaultPanelCSS)
	data, err := os.ReadFile(full)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	_, _ = w.Write(data)
}

func (h *pluginAssetHandler) serveDia(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	_, _ = w.Write([]byte(generatedDiaJS()))
}

// generatedPanelHTML returns the default HTML the host shows when
// the plugin does not ship its own panel/index.html. It includes a
// <div id="root"> mount point, loads /panel.js, and exposes
// window.dia via /dia.js.
func generatedPanelHTML(m *plugins.Manifest) string {
	title := m.UI.Title
	if title == "" {
		title = m.Name
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>%s</title>
  <link rel="stylesheet" href="/styles.css">
  <style>
    html, body { margin: 0; height: 100%%; background: #1e1e2e; color: #e0e0e0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    body { display: flex; flex-direction: column; }
    #root { flex: 1; min-height: 0; display: flex; flex-direction: column; }
  </style>
</head>
<body>
  <div id="root"></div>
  <script src="/dia.js"></script>
  <script src="/panel.js"></script>
</body>
</html>
`, templateEscape(title))
}

// generatedDiaJS returns the JS that creates window.dia from the
// wails binding. The plugin can use dia.call(method, args) for any
// method, or dia.<name>() for the typed helpers.
func generatedDiaJS() string {
	return `(function () {
  function call(method, args) {
    var p = window.go && window.go.wailsapp && window.go.wailsapp.App
      ? window.go.wailsapp.App.DiaCall(method, JSON.stringify(args == null ? [] : args))
      : Promise.reject(new Error("wails runtime not ready"));
    return p.then(function (raw) { return raw == null || raw === "" ? null : JSON.parse(raw); });
  }
  function callArgless(method) { return call(method, []); }
  window.dia = {
    call: call,
    capabilities: function () { return callArgless("capabilities"); },
    pluginDir: function () { return callArgless("pluginDir"); },
    listWorkspaces: function () { return callArgless("listWorkspaces"); },
    getWorkspace: function (name) { return call("getWorkspace", [name]); },
    listInstances: function () { return callArgless("listInstances"); },
    doctor: function () { return callArgless("doctor"); },
    paths: function () { return callArgless("paths"); },
    getTheme: function () { return callArgless("getTheme"); },
    listCustomThemes: function () { return callArgless("listCustomThemes"); }
  };
})();
`
}

func templateEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// pluginWindowHost implements plugins.HostAPI for the read-only
// subset of operations a plugin window can perform. The plugin
// window process does not own the workspace runtime, so mutating
// operations return errHostUnhandled and the dispatcher falls
// through to the plugin's goja runtime.
type pluginWindowHost struct {
	stateFile string
	store     *state.Store
}

var errHostUnhandled = errors.New("plugin window host: unhandled")

func newPluginWindowHost(stateDir string) (*pluginWindowHost, error) {
	st, err := state.OpenAt(filepath.Join(stateDir, state.StateFile))
	if err != nil {
		return nil, err
	}
	return &pluginWindowHost{stateFile: filepath.Join(stateDir, state.StateFile), store: st}, nil
}

func (h *pluginWindowHost) dispatch(method string, args []any) (any, error) {
	switch method {
	case "listWorkspaces":
		return pluginWindowListWorkspaces()
	case "listInstances":
		return pluginWindowListInstances(h.store), nil
	case "doctor":
		return []any{}, nil
	case "paths":
		return PathsInfo{StateFile: h.stateFile}, nil
	case "getTheme":
		t := h.store.Snapshot().Theme
		if t == "" {
			t = state.DefaultTheme
		}
		return t, nil
	case "listCustomThemes":
		return pluginWindowListCustomThemes(h.store), nil
	}
	return nil, errHostUnhandled
}

func (h *pluginWindowHost) ListWorkspaces(ctx context.Context) ([]any, error) {
	ws, err := pluginWindowListWorkspaces()
	if err != nil {
		return nil, err
	}
	return ws, nil
}
func (h *pluginWindowHost) GetWorkspace(ctx context.Context, name string) (any, error) {
	return nil, errHostUnhandled
}
func (h *pluginWindowHost) StartWorkspace(ctx context.Context, name string) (any, error) {
	return nil, errHostUnhandled
}
func (h *pluginWindowHost) ListInstances(ctx context.Context) ([]any, error) {
	return pluginWindowListInstances(h.store), nil
}
func (h *pluginWindowHost) StopInstance(ctx context.Context, id string) error {
	return errHostUnhandled
}
func (h *pluginWindowHost) StopAll(ctx context.Context) (int, error) {
	return 0, errHostUnhandled
}
func (h *pluginWindowHost) Doctor(ctx context.Context) ([]any, error) {
	return []any{}, nil
}
func (h *pluginWindowHost) Paths(ctx context.Context) (any, error) {
	return PathsInfo{StateFile: h.stateFile}, nil
}
func (h *pluginWindowHost) GetTheme(ctx context.Context) (string, error) {
	t := h.store.Snapshot().Theme
	if t == "" {
		t = state.DefaultTheme
	}
	return t, nil
}
func (h *pluginWindowHost) SetTheme(ctx context.Context, name string) error {
	return errHostUnhandled
}
func (h *pluginWindowHost) ListCustomThemes(ctx context.Context) ([]any, error) {
	return pluginWindowListCustomThemes(h.store), nil
}
func (h *pluginWindowHost) SetCustomTheme(ctx context.Context, info any) error {
	return errHostUnhandled
}
func (h *pluginWindowHost) DeleteCustomTheme(ctx context.Context, name string) error {
	return errHostUnhandled
}
func (h *pluginWindowHost) NewWorkspace(ctx context.Context, name string) (string, error) {
	return "", errHostUnhandled
}

func pluginWindowListInstances(st *state.Store) []any {
	if st == nil {
		return nil
	}
	snap := st.Snapshot()
	out := make([]any, 0, len(snap.Instances))
	for _, inst := range snap.Instances {
		apps := make([]ProcessInfo, 0, len(inst.Apps))
		for _, a := range inst.Apps {
			apps = append(apps, ProcessInfo{
				Type:   a.Type,
				Cmd:    a.Cmd,
				PID:    a.PID,
				Status: string(a.Status),
				Err:    a.Err,
			})
		}
		started := ""
		if !inst.StartedAt.IsZero() {
			started = inst.StartedAt.UTC().Format("2006-01-02T15:04:05Z")
		}
		out = append(out, InstanceInfo{
			ID:            inst.ID,
			Workspace:     inst.WorkspaceName,
			WorkspacePath: inst.WorkspacePath,
			StartedAt:     started,
			Status:        string(inst.Status),
			Apps:          apps,
		})
	}
	return out
}

func pluginWindowListCustomThemes(st *state.Store) []any {
	if st == nil {
		return nil
	}
	snap := st.Snapshot()
	out := make([]any, 0, len(snap.CustomThemes))
	for name, t := range snap.CustomThemes {
		colors := make(map[string]string, len(t.Colors))
		for k, v := range t.Colors {
			colors[k] = v
		}
		out = append(out, CustomThemeInfo{
			Name:        name,
			ColorScheme: t.ColorScheme,
			Colors:      colors,
		})
	}
	return out
}

// pluginWindowListWorkspaces reads the on-disk workspace configs.
// The plugin window process never starts or stops workspaces, but
// can list them for read-only panels.
func pluginWindowListWorkspaces() ([]any, error) {
	sources, err := config.Discover(config.DiscoverOptions{
		GlobalDir: config.DefaultGlobalDir(),
	})
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(sources))
	for _, s := range sources {
		info := WorkspaceInfo{
			Name:    s.Workspace.Name,
			Apps:    len(s.Workspace.Apps),
			Source:  workspaceSourceLabel(s),
			Path:    s.Path,
			Running: false,
		}
		m, err := marshalAny(info)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func workspaceSourceLabel(s config.Source) string {
	if s.Local {
		return "local"
	}
	return "global"
}

// loadWorkspacePluginConfig finds the workspace YAML by name, locates
// the plugin ref with the given ID, and returns its config map.
func loadWorkspacePluginConfig(workspaceName, pluginID string) (map[string]any, error) {
	sources, err := config.Discover(config.DiscoverOptions{
		GlobalDir: config.DefaultGlobalDir(),
	})
	if err != nil {
		return nil, err
	}
	for _, s := range sources {
		if s.Workspace.Name == workspaceName {
			for _, ref := range s.Workspace.Plugins {
				if ref.ID == pluginID {
					return ref.Config, nil
				}
			}
			return nil, fmt.Errorf("plugin %q not found in workspace %q", pluginID, workspaceName)
		}
	}
	return nil, fmt.Errorf("workspace %q not found", workspaceName)
}
