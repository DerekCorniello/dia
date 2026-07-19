package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DerekCorniello/dia/internal/plugins"
)

// A scaffold that does not load is worse than no scaffold: the author
// starts from something broken and has to debug dia rather than their
// plugin. These tests parse the generated manifest with the real
// loader, which is what caught the app-type scaffold emitting invalid
// JSON (a %q nested inside an already-quoted description).
func TestPluginNew_ScaffoldsValidManifests(t *testing.T) {
	for _, kind := range []string{"panel", "app"} {
		t.Run(kind, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("XDG_STATE_HOME", tmp)

			args := []string{"plugin", "new", "scaffolded"}
			if kind != "panel" {
				args = append(args, "--type", kind)
			}
			if code := Run(args); code != ExitOK {
				t.Fatalf("plugin new returned %d", code)
			}

			dir := filepath.Join(tmp, "dia", "plugins", "scaffolded")
			m, err := plugins.LoadManifest(dir)
			if err != nil {
				data, _ := os.ReadFile(filepath.Join(dir, "plugin.json"))
				t.Fatalf("generated manifest does not load: %v\n%s", err, data)
			}
			if m.ID != "scaffolded" {
				t.Errorf("ID = %q", m.ID)
			}
			if _, err := os.Stat(filepath.Join(dir, "index.js")); err != nil {
				t.Errorf("no entry script: %v", err)
			}
		})
	}
}

func TestPluginNew_AppTypeScaffoldClaimsItsOwnID(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	if code := Run([]string{"plugin", "new", "mytype", "--type", "app"}); code != ExitOK {
		t.Fatal("plugin new failed")
	}

	m, err := plugins.LoadManifest(filepath.Join(tmp, "dia", "plugins", "mytype"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.AppTypes) != 1 || m.AppTypes[0] != "mytype" {
		t.Errorf("AppTypes = %v, want [mytype]", m.AppTypes)
	}
	// Declaring app_types without apps:resolve fails validation, so
	// the scaffold has to request it.
	if !plugins.HasCapability(m.Capabilities, plugins.CapAppsResolve) {
		t.Errorf("Capabilities = %v, want apps:resolve", m.Capabilities)
	}
}

// The scaffolded type does nothing until apps:resolve is granted, and
// nothing in the generated files says so, so the command has to.
func TestPluginNew_AppTypeTellsYouHowToGrantIt(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	var out, errOut bytes.Buffer
	code := runWithIO([]string{"plugin", "new", "mytype", "--type", "app"},
		strings.NewReader(""), &out, &errOut)
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "--caps apps:resolve") {
		t.Errorf("output should tell the author how to grant the capability:\n%s", out.String())
	}
}

// The panel scaffold must not request a mutating capability.
func TestPluginNew_PanelScaffoldIsReadOnly(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	if code := Run([]string{"plugin", "new", "panelplug"}); code != ExitOK {
		t.Fatal("plugin new failed")
	}
	m, err := plugins.LoadManifest(filepath.Join(tmp, "dia", "plugins", "panelplug"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range m.Capabilities {
		if plugins.IsMutatingCapability(c) {
			t.Errorf("panel scaffold requests mutating capability %q", c)
		}
	}
	if len(m.AppTypes) != 0 {
		t.Errorf("panel scaffold should claim no app types, got %v", m.AppTypes)
	}
}

func TestPluginNew_RejectsUnknownType(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if code := Run([]string{"plugin", "new", "whatever", "--type", "nonsense"}); code == ExitOK {
		t.Error("expected a failure for an unknown --type")
	}
}

// End to end: scaffold, grant, and confirm the type resolves. This is
// the path a new plugin author actually walks.
func TestPluginNew_AppTypeResolvesAfterGranting(t *testing.T) {
	tmp := t.TempDir()
	configHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	if code := Run([]string{"plugin", "new", "mytype", "--type", "app"}); code != ExitOK {
		t.Fatal("plugin new failed")
	}
	if code := Run([]string{"plugin", "enable", "mytype", "--caps", "apps:resolve"}); code != ExitOK {
		t.Fatal("plugin enable failed")
	}

	wsDir := filepath.Join(configHome, "dia", "workspaces")
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wsDir, "w.yaml"),
		[]byte("name: w\napps:\n  - type: mytype\n    cwd: /tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := runWithIO([]string{"start", "w", "--dry-run"}, strings.NewReader(""), &out, &errOut)
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if strings.Contains(out.String(), "unknown app type") {
		t.Errorf("the scaffolded type should resolve once granted:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "via plugin mytype") {
		t.Errorf("expected the type to resolve through the plugin:\n%s", out.String())
	}
}
