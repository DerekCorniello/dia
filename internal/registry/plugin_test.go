package registry

import (
	"errors"
	"strings"
	"testing"

	"github.com/DerekCorniello/dia/internal/config"
)

// stubResolver stands in for the plugin manager.
type stubResolver struct {
	types map[string]string
	out   map[string]any
	err   error
	// gotApp records the marshaled app the handler passed through.
	gotApp map[string]any
	gotTyp string
}

func (s *stubResolver) AppTypes() map[string]string { return s.types }

func (s *stubResolver) ResolveApp(typeName string, app map[string]any) (map[string]any, error) {
	s.gotTyp = typeName
	s.gotApp = app
	if s.err != nil {
		return nil, s.err
	}
	return s.out, nil
}

func TestSyncPluginTypes_LaunchDescriptor(t *testing.T) {
	r := New()
	stub := &stubResolver{
		types: map[string]string{"compose": "compose-plug"},
		out: map[string]any{
			"cmd":  "docker compose",
			"args": []any{"up", "-d"},
			"cwd":  "/srv/app",
			"env":  map[string]any{"COMPOSE_PROJECT_NAME": "demo"},
		},
	}
	if errs := SyncPluginTypes(r, stub); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	action, err := r.Resolve(config.App{Type: "compose"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if action.Kind != ActionLaunch {
		t.Fatalf("Kind = %v, want ActionLaunch", action.Kind)
	}
	// "docker compose" is a two-token cmd; it must be split the same
	// way a built-in cmd is, not exec'd as one program name.
	if action.Launch.Cmd != "docker" {
		t.Errorf("Cmd = %q, want docker", action.Launch.Cmd)
	}
	want := []string{"compose", "up", "-d"}
	if strings.Join(action.Launch.Args, "|") != strings.Join(want, "|") {
		t.Errorf("Args = %v, want %v", action.Launch.Args, want)
	}
	if action.Launch.Cwd != "/srv/app" {
		t.Errorf("Cwd = %q", action.Launch.Cwd)
	}
	if len(action.Launch.Env) != 1 || action.Launch.Env[0] != "COMPOSE_PROJECT_NAME=demo" {
		t.Errorf("Env = %v", action.Launch.Env)
	}
}

func TestSyncPluginTypes_URLDescriptor(t *testing.T) {
	r := New()
	stub := &stubResolver{
		types: map[string]string{"dash": "dash-plug"},
		out:   map[string]any{"url": "https://example.com/dash"},
	}
	SyncPluginTypes(r, stub)

	action, err := r.Resolve(config.App{Type: "dash"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if action.Kind != ActionOpenURL {
		t.Fatalf("Kind = %v, want ActionOpenURL", action.Kind)
	}
	if action.URL != "https://example.com/dash" {
		t.Errorf("URL = %q", action.URL)
	}
}

func TestPluginHandler_PassesAppFieldsThrough(t *testing.T) {
	r := New()
	stub := &stubResolver{
		types: map[string]string{"thing": "thing-plug"},
		out:   map[string]any{"cmd": "true"},
	}
	SyncPluginTypes(r, stub)

	_, err := r.Resolve(config.App{
		Type: "thing",
		Cmd:  "user-cmd",
		Args: []string{"a", "b"},
		Cwd:  "/w",
		Env:  map[string]string{"K": "V"},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if stub.gotTyp != "thing" {
		t.Errorf("type = %q", stub.gotTyp)
	}
	// The plugin sees the workspace entry as the user wrote it, keyed
	// the same way as the YAML.
	if stub.gotApp["cmd"] != "user-cmd" || stub.gotApp["cwd"] != "/w" {
		t.Errorf("app = %+v", stub.gotApp)
	}
	args, ok := stub.gotApp["args"].([]any)
	if !ok || len(args) != 2 || args[0] != "a" {
		t.Errorf("args = %+v", stub.gotApp["args"])
	}
	env, ok := stub.gotApp["env"].(map[string]any)
	if !ok || env["K"] != "V" {
		t.Errorf("env = %+v", stub.gotApp["env"])
	}
}

// A resolver that omits cwd should inherit the one the user wrote in
// the workspace, rather than silently launching in dia's cwd.
func TestPluginHandler_InheritsAppCwdWhenResolverOmitsIt(t *testing.T) {
	r := New()
	SyncPluginTypes(r, &stubResolver{
		types: map[string]string{"thing": "p"},
		out:   map[string]any{"cmd": "true"},
	})
	action, err := r.Resolve(config.App{Type: "thing", Cwd: "/from/workspace"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if action.Launch.Cwd != "/from/workspace" {
		t.Errorf("Cwd = %q, want the app's cwd", action.Launch.Cwd)
	}
}

func TestSyncPluginTypes_RefusesBuiltinNames(t *testing.T) {
	r := New()
	stub := &stubResolver{
		types: map[string]string{"editor": "sneaky", "fine": "sneaky"},
		out:   map[string]any{"cmd": "malicious"},
	}
	errs := SyncPluginTypes(r, stub)
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "editor") {
		t.Errorf("error should name the refused type: %v", errs[0])
	}

	// The built-in must still behave as it always did.
	action, err := r.Resolve(config.App{Type: "editor", Cmd: "code ."})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if action.Launch.Cmd != "code" {
		t.Errorf("the built-in editor handler was replaced: %+v", action.Launch)
	}
	// The plugin's other, non-conflicting type still registers.
	if _, err := r.Resolve(config.App{Type: "fine"}); err != nil {
		t.Errorf("non-conflicting type should still work: %v", err)
	}
}

func TestPluginHandler_ResolverErrorPropagates(t *testing.T) {
	r := New()
	SyncPluginTypes(r, &stubResolver{
		types: map[string]string{"broken": "broken-plug"},
		err:   errors.New("no compose file found"),
	})
	_, err := r.Resolve(config.App{Type: "broken"})
	if err == nil {
		t.Fatal("expected the resolver error to surface")
	}
	if !strings.Contains(err.Error(), "no compose file found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPluginHandler_BadDescriptors(t *testing.T) {
	tests := []struct {
		name    string
		out     map[string]any
		wantErr string
	}{
		{name: "neither cmd nor url", out: map[string]any{}, wantErr: "exactly one"},
		{
			name:    "both cmd and url",
			out:     map[string]any{"cmd": "ls", "url": "https://x.com"},
			wantErr: "exactly one",
		},
		{
			name:    "url with cwd",
			out:     map[string]any{"url": "https://x.com", "cwd": "/tmp"},
			wantErr: "cmd-only fields",
		},
		{
			name:    "unknown key",
			out:     map[string]any{"command": "ls"},
			wantErr: "unknown key",
		},
		{
			name:    "cmd wrong type",
			out:     map[string]any{"cmd": 42},
			wantErr: "must be a string",
		},
		{
			name:    "args not an array",
			out:     map[string]any{"cmd": "ls", "args": "a b"},
			wantErr: "must be an array",
		},
		{
			name:    "arg element not a string",
			out:     map[string]any{"cmd": "ls", "args": []any{"a", 3}},
			wantErr: "must be a string",
		},
		{
			name:    "env value not a string",
			out:     map[string]any{"cmd": "ls", "env": map[string]any{"K": 1}},
			wantErr: "must be a string",
		},
		{
			name:    "blank cmd",
			out:     map[string]any{"cmd": "   "},
			wantErr: "exactly one",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New()
			SyncPluginTypes(r, &stubResolver{
				types: map[string]string{"t": "p"},
				out:   tt.out,
			})
			_, err := r.Resolve(config.App{Type: "t"})
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
			// The message has to say which plugin and type, or the user
			// cannot tell which of their plugins is misbehaving.
			if !strings.Contains(err.Error(), `"p"`) && !strings.Contains(err.Error(), `"t"`) {
				t.Errorf("error should identify the plugin and type: %v", err)
			}
		})
	}
}

func TestSyncPluginTypes_NilArgs(t *testing.T) {
	if errs := SyncPluginTypes(nil, &stubResolver{}); errs != nil {
		t.Errorf("nil registry should be a no-op, got %v", errs)
	}
	if errs := SyncPluginTypes(New(), nil); errs != nil {
		t.Errorf("nil resolver should be a no-op, got %v", errs)
	}
}

func TestIsBuiltinType(t *testing.T) {
	for _, name := range []string{"", "local", "editor", "browser", "gh", "gh:pr", "gh:repo-clone"} {
		if !IsBuiltinType(name) {
			t.Errorf("%q should be built in", name)
		}
	}
	for _, name := range []string{"compose", "my-type", "gh:nope"} {
		if IsBuiltinType(name) {
			t.Errorf("%q should not be built in", name)
		}
	}
}

// Every type the registry ships must be reported as built-in, or a
// plugin could shadow one that was added later.
func TestIsBuiltinType_CoversEveryRegisteredType(t *testing.T) {
	for _, name := range New().Types() {
		if !IsBuiltinType(name) {
			t.Errorf("registry ships type %q but IsBuiltinType does not know it", name)
		}
	}
}

// An app with no type is documented as "runs cmd" and the config
// loader accepts it, but the registry had no handler for the empty
// string, so it failed at launch with `unknown app type ""`.
func TestResolve_EmptyTypeUsesTheDefaultHandler(t *testing.T) {
	action, err := New().Resolve(config.App{Cmd: "code ."})
	if err != nil {
		t.Fatalf("an app with no type must resolve: %v", err)
	}
	if action.Kind != ActionLaunch {
		t.Fatalf("Kind = %v, want ActionLaunch", action.Kind)
	}
	if action.Launch.Cmd != "code" {
		t.Errorf("Cmd = %q, want code", action.Launch.Cmd)
	}
}

func TestResolve_EmptyTypeStillRequiresCmd(t *testing.T) {
	if _, err := New().Resolve(config.App{}); err == nil {
		t.Error("an app with neither type nor cmd must still fail")
	}
}

// `{ cwd: app.cwd, env: app.env }` is how a resolver is naturally
// written, and those fields are nil whenever the workspace entry
// omitted them. That must behave as "not set", not as a type error.
func TestPluginHandler_NilOptionalFieldsAreTreatedAsAbsent(t *testing.T) {
	r := New()
	SyncPluginTypes(r, &stubResolver{
		types: map[string]string{"t": "p"},
		out: map[string]any{
			"cmd":  "docker compose",
			"args": []any{"up"},
			"cwd":  nil,
			"env":  nil,
		},
	})
	action, err := r.Resolve(config.App{Type: "t", Cwd: "/from/workspace"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if action.Launch.Cwd != "/from/workspace" {
		t.Errorf("Cwd = %q, want the app's cwd", action.Launch.Cwd)
	}
	if len(action.Launch.Env) != 0 {
		t.Errorf("Env = %v, want empty", action.Launch.Env)
	}
}

// A nil cmd is still a missing cmd, not a valid one.
func TestPluginHandler_NilCmdStillFails(t *testing.T) {
	r := New()
	SyncPluginTypes(r, &stubResolver{
		types: map[string]string{"t": "p"},
		out:   map[string]any{"cmd": nil},
	})
	if _, err := r.Resolve(config.App{Type: "t"}); err == nil {
		t.Error("expected an error when cmd is nil and there is no url")
	}
}
