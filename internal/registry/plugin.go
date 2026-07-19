package registry

import (
	"fmt"
	"sort"
	"strings"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/platform"
)

// AppResolver is the piece of the plugin manager the registry needs.
// It is stated as an interface here, and takes and returns plain maps,
// so this package does not depend on internal/plugins.
type AppResolver interface {
	// AppTypes maps each claimed app type to the plugin id providing it.
	AppTypes() map[string]string
	// ResolveApp asks the owning plugin to turn a marshaled config.App
	// into a launch descriptor: {cmd, args?, cwd?, env?} or {url}.
	ResolveApp(typeName string, app map[string]any) (map[string]any, error)
}

// builtinTypes are the type names dia ships. A plugin cannot claim one:
// silently shadowing `editor` or `browser` would change what every
// existing workspace does.
func builtinTypes() map[string]bool {
	return map[string]bool{
		"":              true,
		"local":         true,
		"editor":        true,
		"terminal":      true,
		"service":       true,
		"custom":        true,
		"ai":            true,
		"open":          true,
		"browser":       true,
		"gh":            true,
		"gh:pr":         true,
		"gh:issue":      true,
		"gh:checkout":   true,
		"gh:repo-clone": true,
	}
}

// IsBuiltinType reports whether name is one of dia's own app types.
func IsBuiltinType(name string) bool { return builtinTypes()[name] }

// SyncPluginTypes makes the registry match the resolver's current
// claims exactly: types no longer claimed are removed, and current
// claims are (re)registered. Reconciling rather than only adding
// matters once grants can change at runtime -- revoking apps:resolve
// has to make the type stop resolving, not merely stop working at the
// next restart.
//
// Claims on built-in type names are refused and returned as errors;
// the caller decides whether that is fatal (it is not: the rest of
// the plugin's types still work, and the user needs to be told which
// one was dropped). Order is deterministic so an error list does not
// shift between runs.
func SyncPluginTypes(r *Registry, resolver AppResolver) []error {
	if r == nil || resolver == nil {
		return nil
	}
	claimed := resolver.AppTypes()

	// Drop every previously-registered plugin handler that is no
	// longer claimed. Built-ins are never pluginHandlers, so they
	// cannot be removed here.
	for _, name := range r.Types() {
		h, ok := r.handler(name)
		if !ok {
			continue
		}
		if _, isPlugin := h.(pluginHandler); !isPlugin {
			continue
		}
		if _, stillClaimed := claimed[name]; !stillClaimed {
			r.Unregister(name)
		}
	}

	types := make([]string, 0, len(claimed))
	for t := range claimed {
		types = append(types, t)
	}
	sort.Strings(types)

	var errs []error
	for _, t := range types {
		if IsBuiltinType(t) {
			errs = append(errs, fmt.Errorf(
				"plugin %q claims app type %q, which is built in; the built-in wins and the plugin's claim is ignored",
				claimed[t], t))
			continue
		}
		r.Register(pluginHandler{typeName: t, pluginID: claimed[t], resolver: resolver})
	}
	return errs
}

// pluginHandler resolves one app type by calling back into the plugin
// that claims it.
type pluginHandler struct {
	typeName string
	pluginID string
	resolver AppResolver
}

func (h pluginHandler) Type() string { return h.typeName }

func (h pluginHandler) Resolve(app config.App) (Action, error) {
	desc, err := h.resolver.ResolveApp(h.typeName, appToMap(app))
	if err != nil {
		return Action{}, err
	}

	d, err := decodeDescriptor(desc)
	if err != nil {
		return Action{}, fmt.Errorf("plugin %q, app type %q: %w", h.pluginID, h.typeName, err)
	}
	if d.URL != "" {
		return Action{Kind: ActionOpenURL, URL: d.URL}, nil
	}

	program, prefixArgs, err := SplitProgram(d.Cmd)
	if err != nil {
		return Action{}, fmt.Errorf("plugin %q, app type %q: %w", h.pluginID, h.typeName, err)
	}
	args := make([]string, 0, len(prefixArgs)+len(d.Args))
	args = append(args, prefixArgs...)
	args = append(args, d.Args...)

	opts := platform.LaunchOpts{
		Cmd:  program,
		Args: args,
		Cwd:  d.Cwd,
		Env:  envMapToSlice(d.Env),
	}
	// A resolver that leaves cwd unset should inherit the app's, which
	// is what a user who wrote `cwd:` in their workspace expects.
	if opts.Cwd == "" {
		opts.Cwd = app.Cwd
	}
	return Action{Kind: ActionLaunch, Launch: &opts}, nil
}

// appToMap marshals a config.App into the shape a plugin's resolveApp
// receives. The keys match the YAML the user wrote, so a plugin author
// reads app.cmd and app.env exactly as they appear in the workspace.
func appToMap(app config.App) map[string]any {
	m := map[string]any{"type": app.Type}
	if app.Label != "" {
		m["label"] = app.Label
	}
	if app.Cmd != "" {
		m["cmd"] = app.Cmd
	}
	if len(app.Args) > 0 {
		args := make([]any, len(app.Args))
		for i, a := range app.Args {
			args[i] = a
		}
		m["args"] = args
	}
	if app.Cwd != "" {
		m["cwd"] = app.Cwd
	}
	if len(app.Env) > 0 {
		env := make(map[string]any, len(app.Env))
		for k, v := range app.Env {
			env[k] = v
		}
		m["env"] = env
	}
	if app.Url != "" {
		m["url"] = app.Url
	}
	return m
}

// descriptor is the decoded form of what a plugin resolveApp returns.
// It lives here rather than in internal/plugins so the dependency
// keeps pointing one way; decodeDescriptor below is the only place
// that validates the shape.
type descriptor struct {
	Cmd  string
	Args []string
	Cwd  string
	Env  map[string]string
	URL  string
}

func decodeDescriptor(m map[string]any) (descriptor, error) {
	var d descriptor
	for k, v := range m {
		// `{ cwd: app.cwd }` where app.cwd is absent is idiomatic JS
		// and arrives here as nil. Treat that as "key not set" rather
		// than a type error, or every resolver has to guard each
		// optional field by hand.
		if v == nil {
			continue
		}
		switch k {
		case "cmd":
			s, ok := v.(string)
			if !ok {
				return d, fmt.Errorf("%q must be a string, got %T", k, v)
			}
			d.Cmd = s
		case "url":
			s, ok := v.(string)
			if !ok {
				return d, fmt.Errorf("%q must be a string, got %T", k, v)
			}
			d.URL = s
		case "cwd":
			s, ok := v.(string)
			if !ok {
				return d, fmt.Errorf("%q must be a string, got %T", k, v)
			}
			d.Cwd = s
		case "args":
			args, err := toStringSlice(v)
			if err != nil {
				return d, fmt.Errorf("%q: %w", k, err)
			}
			d.Args = args
		case "env":
			env, err := toStringMap(v)
			if err != nil {
				return d, fmt.Errorf("%q: %w", k, err)
			}
			d.Env = env
		default:
			return d, fmt.Errorf("unknown key %q in the resolveApp result "+
				"(expected cmd, args, cwd, env, or url)", k)
		}
	}

	hasCmd := strings.TrimSpace(d.Cmd) != ""
	hasURL := d.URL != ""
	switch {
	case hasCmd && hasURL:
		return d, fmt.Errorf(`resolveApp returned both "cmd" and "url"; it must return exactly one`)
	case !hasCmd && !hasURL:
		return d, fmt.Errorf(`resolveApp returned neither "cmd" nor "url"; it must return exactly one`)
	}
	if hasURL && (len(d.Args) > 0 || d.Cwd != "" || len(d.Env) > 0) {
		return d, fmt.Errorf(`resolveApp returned "url" alongside cmd-only fields (args/cwd/env)`)
	}
	return d, nil
}

func toStringSlice(v any) ([]string, error) {
	raw, ok := v.([]any)
	if !ok {
		if s, ok := v.([]string); ok {
			return s, nil
		}
		return nil, fmt.Errorf("must be an array, got %T", v)
	}
	out := make([]string, len(raw))
	for i, item := range raw {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("element %d must be a string, got %T", i, item)
		}
		out[i] = s
	}
	return out, nil
}

func toStringMap(v any) (map[string]string, error) {
	raw, ok := v.(map[string]any)
	if !ok {
		if m, ok := v.(map[string]string); ok {
			return m, nil
		}
		return nil, fmt.Errorf("must be an object, got %T", v)
	}
	out := make(map[string]string, len(raw))
	for k, item := range raw {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("value for %q must be a string, got %T", k, item)
		}
		out[k] = s
	}
	return out, nil
}
