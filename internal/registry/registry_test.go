package registry

import (
	"strings"
	"testing"

	"github.com/DerekCorniello/dia/internal/config"
)

func TestNew_HasBuiltins(t *testing.T) {
	r := New()
	want := []string{"browser", "custom", "editor", "gh", "gh:checkout", "gh:issue", "gh:pr", "gh:repo-clone", "local", "open", "service", "terminal"}
	got := r.Types()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Types = %v, want %v", got, want)
	}
}

func TestResolve_Local(t *testing.T) {
	r := New()
	app := config.App{Type: "local", Cmd: "code", Args: []string{"."}}
	a, err := r.Resolve(app)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if a.Kind != ActionLaunch {
		t.Fatalf("Kind = %v, want ActionLaunch", a.Kind)
	}
	if a.Launch.Cmd != "code" {
		t.Errorf("Cmd = %q, want code", a.Launch.Cmd)
	}
	if len(a.Launch.Args) != 1 || a.Launch.Args[0] != "." {
		t.Errorf("Args = %v, want [\".\"]", a.Launch.Args)
	}
}

func TestResolve_LocalAliases(t *testing.T) {
	r := New()
	for _, typ := range []string{"editor", "terminal", "service", "custom"} {
		app := config.App{Type: typ, Cmd: "x"}
		a, err := r.Resolve(app)
		if err != nil {
			t.Errorf("type %q: %v", typ, err)
			continue
		}
		if a.Kind != ActionLaunch {
			t.Errorf("type %q: Kind = %v", typ, a.Kind)
		}
	}
}

func TestResolve_LocalMissingCmd(t *testing.T) {
	r := New()
	if _, err := r.Resolve(config.App{Type: "local"}); err == nil {
		t.Errorf("expected error for missing cmd")
	}
	if _, err := r.Resolve(config.App{Type: "editor"}); err == nil {
		t.Errorf("expected error for editor without cmd")
	}
}

func TestResolve_Open(t *testing.T) {
	r := New()
	app := config.App{Type: "open", Url: "mailto:hi@example.com"}
	a, err := r.Resolve(app)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if a.Kind != ActionOpenURL {
		t.Fatalf("Kind = %v, want ActionOpenURL", a.Kind)
	}
	if a.URL != "mailto:hi@example.com" {
		t.Errorf("URL = %q", a.URL)
	}
}

func TestResolve_OpenMissingURL(t *testing.T) {
	r := New()
	if _, err := r.Resolve(config.App{Type: "open"}); err == nil {
		t.Errorf("expected error for missing url")
	}
}

func TestResolve_Browser(t *testing.T) {
	r := New()
	a, err := r.Resolve(config.App{Type: "browser", Url: "https://example.com"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if a.Kind != ActionOpenURL {
		t.Errorf("Kind = %v, want ActionOpenURL", a.Kind)
	}
}

func TestResolve_GH(t *testing.T) {
	r := New()
	app := config.App{Type: "gh", Cmd: "pr", Args: []string{"view", "123", "--web"}}
	a, err := r.Resolve(app)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if a.Launch.Cmd != "gh" {
		t.Errorf("Cmd = %q, want gh", a.Launch.Cmd)
	}
	want := []string{"pr", "view", "123", "--web"}
	if strings.Join(a.Launch.Args, ",") != strings.Join(want, ",") {
		t.Errorf("Args = %v, want %v", a.Launch.Args, want)
	}
}

func TestResolve_GHSugar(t *testing.T) {
	r := New()
	cases := []struct {
		typ, sub string
	}{
		{"gh:pr", "pr"},
		{"gh:issue", "issue"},
		{"gh:checkout", "checkout"},
	}
	for _, c := range cases {
		a, err := r.Resolve(config.App{Type: c.typ, Args: []string{"list"}})
		if err != nil {
			t.Errorf("type %q: %v", c.typ, err)
			continue
		}
		if a.Launch.Cmd != "gh" {
			t.Errorf("type %q: Cmd = %q, want gh", c.typ, a.Launch.Cmd)
		}
		if len(a.Launch.Args) < 2 || a.Launch.Args[0] != c.sub {
			t.Errorf("type %q: Args[0] = %q, want %q", c.typ, a.Launch.Args[0], c.sub)
		}
	}
}

func TestResolve_GHRepoClone(t *testing.T) {
	r := New()
	app := config.App{Type: "gh:repo-clone", Url: "https://github.com/o/r"}
	a, err := r.Resolve(app)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := []string{"repo", "clone", "https://github.com/o/r"}
	if strings.Join(a.Launch.Args, ",") != strings.Join(want, ",") {
		t.Errorf("Args = %v, want %v", a.Launch.Args, want)
	}
}

func TestResolve_GHRepoCloneWithCwd(t *testing.T) {
	r := New()
	app := config.App{
		Type: "gh:repo-clone",
		Url:  "https://github.com/o/r",
		Cwd:  "/tmp/dest",
	}
	a, err := r.Resolve(app)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := []string{"repo", "clone", "https://github.com/o/r", "/tmp/dest"}
	if strings.Join(a.Launch.Args, ",") != strings.Join(want, ",") {
		t.Errorf("Args = %v, want %v", a.Launch.Args, want)
	}
	if a.Launch.Cwd != "/tmp/dest" {
		t.Errorf("Cwd = %q, want /tmp/dest", a.Launch.Cwd)
	}
}

func TestResolve_Env(t *testing.T) {
	r := New()
	app := config.App{
		Type: "local",
		Cmd:  "go",
		Env:  map[string]string{"FOO": "bar", "BAZ": "qux"},
	}
	a, err := r.Resolve(app)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	found := map[string]bool{}
	for _, e := range a.Launch.Env {
		k, v, _ := strings.Cut(e, "=")
		found[k] = v == "bar" || v == "qux"
	}
	if !found["FOO"] || !found["BAZ"] {
		t.Errorf("env missing FOO/BAZ: %v", a.Launch.Env)
	}
}

func TestResolve_CwdPassthrough(t *testing.T) {
	r := New()
	app := config.App{Type: "local", Cmd: "go", Cwd: "~/proj"}
	a, err := r.Resolve(app)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if a.Launch.Cwd != "~/proj" {
		t.Errorf("Cwd = %q, want ~/proj", a.Launch.Cwd)
	}
}

func TestResolve_UnknownType(t *testing.T) {
	r := New()
	if _, err := r.Resolve(config.App{Type: "nope"}); err == nil {
		t.Errorf("expected error for unknown type")
	}
}
