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

func TestDiscoverProjectLocalDoesNotShadowGlobal(t *testing.T) {
	g := t.TempDir()
	writeYAML(t, g, "shared.yaml", "version: 1\nname: shared\napps:\n  - type: custom\n    cmd: echo global\n")

	root := t.TempDir()
	writeYAML(t, root, ProjectLocalFile, "version: 1\nname: shared\napps:\n  - type: custom\n    cmd: echo local\n")

	sources, err := Discover(DiscoverOptions{GlobalDir: g, CWD: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources (both local and global), got %d", len(sources))
	}
	// Both should appear; sorted by name (both "shared"), then by whatever order
	// they were discovered. We just verify both are present with distinct paths.
	seen := map[string]bool{}
	for _, s := range sources {
		seen[s.Path] = true
	}
	if len(seen) != 2 {
		t.Error("expected two distinct paths")
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

func TestDiscoverRoots(t *testing.T) {
	g := t.TempDir()
	// Global workspace.
	writeYAML(t, g, "global.yaml", "version: 1\nname: global-ws\napps:\n  - type: custom\n    cmd: echo global\n")

	root1 := t.TempDir()
	writeYAML(t, root1, ".dia.yaml", "version: 1\nname: root1-ws\napps:\n  - type: custom\n    cmd: echo r1\n")

	root2 := t.TempDir()
	dia2 := root2 + "/.dia"
	if err := os.MkdirAll(dia2, 0755); err != nil {
		t.Fatal(err)
	}
	writeYAML(t, dia2, "nested.yaml", "version: 1\nname: root2-ws\napps:\n  - type: custom\n    cmd: echo r2\n")

	sources, err := Discover(DiscoverOptions{
		GlobalDir: g,
		Roots:     []string{root1, root2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 3 {
		t.Fatalf("expected 3 sources (global + 2 roots), got %d", len(sources))
	}
	names := make([]string, len(sources))
	for i, s := range sources {
		names[i] = s.Workspace.Name
	}
	// Sorted by name.
	want := []string{"global-ws", "root1-ws", "root2-ws"}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("sources[%d].name = %q, want %q", i, n, want[i])
		}
	}
}
