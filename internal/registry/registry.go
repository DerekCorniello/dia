// Package registry maps workspace app types to the launch actions
// that realize them. The runtime depends on this package: for each
// app in a workspace it asks the registry to resolve an Action, then
// either launches a process or opens a URL.
package registry

import (
	"fmt"
	"sort"
	"sync"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/platform"
)

// ActionKind tells the runtime whether to launch a process or open a
// URL in the OS default handler.
type ActionKind int

const (
	// ActionLaunch starts a process via platform.Launch.
	ActionLaunch ActionKind = iota
	// ActionOpenURL asks the OS to open URL via platform.OpenURL.
	ActionOpenURL
)

// Action is the concrete plan the runtime should execute for an app.
// Exactly one of Launch or URL is set, depending on Kind.
type Action struct {
	Kind   ActionKind
	Launch *platform.LaunchOpts
	URL    string
}

// Handler resolves an app's config to an Action or an error.
type Handler interface {
	// Type is the canonical name this handler claims (e.g. "local",
	// "open", "gh"). Used for diagnostics.
	Type() string
	// Resolve turns the app config into an Action. A non-nil error
	// means the app could not be started (e.g. plugin not found).
	Resolve(app config.App) (Action, error)
}

// HandlerFunc lets a plain function act as a Handler without a type.
type HandlerFunc struct {
	Name string
	Fn   func(app config.App) (Action, error)
}

func (h HandlerFunc) Type() string                           { return h.Name }
func (h HandlerFunc) Resolve(app config.App) (Action, error) { return h.Fn(app) }

// Registry maps app type strings to handlers. The empty string is a
// special key for the default handler (used when no type is set).
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
}

// New returns a Registry populated with the built-in handlers
// (local, open, browser, gh, gh:* sugar, plugin via the default
// PluginResolver, and implicit `dia-<type>` lookup).
func New() *Registry {
	r := &Registry{handlers: map[string]Handler{}}
	r.Register(HandlerFunc{Name: "local", Fn: resolveLocal})
	r.Register(HandlerFunc{Name: "open", Fn: resolveOpen})
	r.Register(HandlerFunc{Name: "browser", Fn: resolveBrowser})
	r.Register(HandlerFunc{Name: "gh", Fn: resolveGH})
	// Sugar: type encodes the subcommand. Args are passed through.
	r.Register(HandlerFunc{Name: "gh:pr", Fn: resolveGHSugar("pr")})
	r.Register(HandlerFunc{Name: "gh:issue", Fn: resolveGHSugar("issue")})
	r.Register(HandlerFunc{Name: "gh:checkout", Fn: resolveGHSugar("checkout")})
	r.Register(HandlerFunc{Name: "gh:repo-clone", Fn: resolveGHRepoClone})
	// Aliases for `local`. Kept as labels so configs stay readable
	// in YAML.
	r.Register(HandlerFunc{Name: "editor", Fn: resolveLocal})
	r.Register(HandlerFunc{Name: "terminal", Fn: resolveLocal})
	r.Register(HandlerFunc{Name: "service", Fn: resolveLocal})
	r.Register(HandlerFunc{Name: "custom", Fn: resolveLocal})
	return r
}

// WithPlugins returns a copy of the registry that also handles
// `plugin` (explicit) and any unknown type (implicit) by looking up
// `dia-<name>` on PATH via the given resolver.
func (r *Registry) WithPlugins(p *PluginResolver) *Registry {
	out := &Registry{handlers: map[string]Handler{}}
	r.mu.RLock()
	for k, v := range r.handlers {
		out.handlers[k] = v
	}
	r.mu.RUnlock()
	out.Register(HandlerFunc{Name: "plugin", Fn: resolvePlugin(p)})
	return out
}

// Register adds or replaces a handler for the given type.
func (r *Registry) Register(h Handler) {
	r.mu.Lock()
	r.handlers[h.Type()] = h
	r.mu.Unlock()
}

// Resolve returns the Action for the given app. It tries, in order:
// the registered handler for the type, then (if the app declares
// a plugin) the plugin resolver. A nil app or unknown type with no
// plugin is an error.
func (r *Registry) Resolve(app config.App, p *PluginResolver) (Action, error) {
	r.mu.RLock()
	h, ok := r.handlers[app.Type]
	r.mu.RUnlock()
	if ok {
		return h.Resolve(app)
	}
	if app.Plugin != "" {
		return resolvePlugin(p)(app)
	}
	if p == nil {
		return Action{}, fmt.Errorf("unknown app type %q (and no plugin resolver configured)", app.Type)
	}
	// Last resort: implicit `dia-<type>` lookup. This is the
	// mechanism the validator mentions in its unknown-type comment.
	return resolvePlugin(p)(config.App{Type: "", Plugin: app.Type})
}

// Types returns the sorted list of registered type names, for
// diagnostics and `--help` output.
func (r *Registry) Types() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.handlers))
	for k := range r.handlers {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
