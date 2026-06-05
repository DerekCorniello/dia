package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeYAML(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}

const goodYAML = `version: 1
name: %s
apps:
  - type: custom
    cmd: echo %s
`

func TestDiscoverGlobalOnly(t *testing.T) {
	g := t.TempDir()
	writeYAML(t, g, "a.yaml", "version: 1\nname: alpha\napps:\n  - type: custom\n    cmd: echo a\n")
	writeYAML(t, g, "b.yaml", "version: 1\nname: beta\napps:\n  - type: custom\n    cmd: echo b\n")

	sources, err := Discover(DiscoverOptions{GlobalDir: g})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
	if sources[0].Workspace.Name != "alpha" || sources[1].Workspace.Name != "beta" {
		t.Errorf("expected sorted by name, got %s, %s", sources[0].Workspace.Name, sources[1].Workspace.Name)
	}
}

func TestDiscoverEmptyGlobal(t *testing.T) {
	g := t.TempDir()
	sources, err := Discover(DiscoverOptions{GlobalDir: g})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 0 {
		t.Fatalf("expected 0 sources, got %d", len(sources))
	}
}

func TestDiscoverMissingGlobalIsOK(t *testing.T) {
	g := filepath.Join(t.TempDir(), "does-not-exist")
	sources, err := Discover(DiscoverOptions{GlobalDir: g})
	if err != nil {
		t.Fatalf("missing global dir should not error, got: %v", err)
	}
	if len(sources) != 0 {
		t.Fatalf("expected 0 sources, got %d", len(sources))
	}
}

func TestDiscoverProjectLocalWalkUp(t *testing.T) {
	g := t.TempDir()
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	writeYAML(t, root, ProjectLocalFile, "version: 1\nname: repo\napps:\n  - type: custom\n    cmd: echo repo\n")

	sources, err := Discover(DiscoverOptions{GlobalDir: g, CWD: sub})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}
	if !sources[0].Local {
		t.Error("expected Local=true")
	}
	if sources[0].Workspace.Name != "repo" {
		t.Errorf("expected name=repo, got %q", sources[0].Workspace.Name)
	}
}

func TestDiscoverProjectLocalShadowsGlobal(t *testing.T) {
	g := t.TempDir()
	writeYAML(t, g, "shared.yaml", "version: 1\nname: shared\napps:\n  - type: custom\n    cmd: echo global\n")

	root := t.TempDir()
	writeYAML(t, root, ProjectLocalFile, "version: 1\nname: shared\napps:\n  - type: custom\n    cmd: echo local\n")

	sources, err := Discover(DiscoverOptions{GlobalDir: g, CWD: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source after shadow, got %d", len(sources))
	}
	if sources[0].Workspace.Apps[0].Cmd != "echo local" {
		t.Errorf("expected local to win, got cmd=%q", sources[0].Workspace.Apps[0].Cmd)
	}
	if !sources[0].Local {
		t.Error("expected Local=true on shadowed entry")
	}
}

func TestDiscoverMergesAndSorts(t *testing.T) {
	g := t.TempDir()
	writeYAML(t, g, "z.yaml", "version: 1\nname: zeta\napps:\n  - type: custom\n    cmd: z\n")
	writeYAML(t, g, "a.yaml", "version: 1\nname: alpha\napps:\n  - type: custom\n    cmd: a\n")

	root := t.TempDir()
	writeYAML(t, root, ProjectLocalFile, "version: 1\nname: middle\napps:\n  - type: custom\n    cmd: m\n")

	sources, err := Discover(DiscoverOptions{GlobalDir: g, CWD: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 3 {
		t.Fatalf("expected 3 sources, got %d", len(sources))
	}
	want := []string{"alpha", "middle", "zeta"}
	for i, s := range sources {
		if s.Workspace.Name != want[i] {
			t.Errorf("sources[%d].name = %q, want %q", i, s.Workspace.Name, want[i])
		}
	}
}

func TestFindLocal(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "deep", "nested")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	writeYAML(t, root, ProjectLocalFile, "version: 1\nname: x\napps:\n  - type: custom\n    cmd: x\n")

	got := FindLocal(sub)
	want := filepath.Join(root, ProjectLocalFile)
	if got != want {
		t.Errorf("FindLocal = %q, want %q", got, want)
	}

	if FindLocal(t.TempDir()) != "" {
		t.Error("FindLocal should return empty for dir with no .dia.yaml")
	}
}

func TestDiscoverIgnoresNonYAML(t *testing.T) {
	g := t.TempDir()
	writeYAML(t, g, "ok.yaml", "version: 1\nname: ok\napps:\n  - type: custom\n    cmd: x\n")
	writeYAML(t, g, "README.md", "not a workspace")
	writeYAML(t, g, "junk.txt", "also not")

	sources, err := Discover(DiscoverOptions{GlobalDir: g})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 {
		t.Fatalf("expected 1 source (only ok.yaml), got %d", len(sources))
	}
}
