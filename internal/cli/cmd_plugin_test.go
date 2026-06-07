package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DerekCorniello/dia/internal/state"
)

func TestPluginNew_WritesScaffold(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	if code := Run([]string{"plugin", "new", "hello"}); code != ExitOK {
		t.Fatalf("Run returned %d", code)
	}
	dir := filepath.Join(tmp, "dia", "plugins", "hello")
	mf, err := os.ReadFile(filepath.Join(dir, "plugin.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(mf, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["id"] != "hello" {
		t.Errorf("id = %v, want hello", m["id"])
	}
	entry, err := os.ReadFile(filepath.Join(dir, "index.js"))
	if err != nil {
		t.Fatalf("read entry: %v", err)
	}
	if !strings.Contains(string(entry), "module.exports") {
		t.Error("entry missing module.exports")
	}
}

func TestPluginNew_RejectsBadID(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	for _, bad := range []string{"Has-Caps", "-leading", "trailing-", "has space", "with/slash"} {
		if code := Run([]string{"plugin", "new", bad}); code == ExitOK {
			t.Errorf("expected non-zero for bad id %q", bad)
		}
	}
}

func TestPluginNew_LocalDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	cwd := t.TempDir()
	prev, _ := os.Getwd()
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(prev)
	if code := Run([]string{"plugin", "new", "demo", "--local"}); code != ExitOK {
		t.Fatalf("Run returned %d", code)
	}
	if _, err := os.Stat(filepath.Join(cwd, ".dia", "plugins", "demo", "plugin.json")); err != nil {
		t.Errorf("local plugin not created: %v", err)
	}
}

func TestPluginList_Empty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	if code := Run([]string{"--json", "plugin", "list"}); code != ExitOK {
		t.Fatalf("Run returned %d", code)
	}
}

func TestPluginEnableDisable_State(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	if code := Run([]string{"plugin", "new", "demo"}); code != ExitOK {
		t.Fatal(code)
	}
	if code := Run([]string{"plugin", "enable", "demo"}); code != ExitOK {
		t.Fatalf("enable: %d", code)
	}
	st, err := state.OpenAt(filepath.Join(tmp, "dia", state.StateFile))
	if err != nil {
		t.Fatal(err)
	}
	ps := st.Snapshot().Plugins["demo"]
	if !ps.Enabled {
		t.Error("should be enabled")
	}
	if len(ps.GrantedCapabilities) == 0 {
		t.Error("expected default grants")
	}
	if code := Run([]string{"plugin", "disable", "demo"}); code != ExitOK {
		t.Fatalf("disable: %d", code)
	}
	st2, err := state.OpenAt(filepath.Join(tmp, "dia", state.StateFile))
	if err != nil {
		t.Fatal(err)
	}
	ps = st2.Snapshot().Plugins["demo"]
	if ps.Enabled {
		t.Error("should be disabled")
	}
}

func TestPluginInstall_FromLocalPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "plugin.json"),
		[]byte(`{"id":"hello","name":"H","version":"0.1.0","ui":{"type":"list","title":"T"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := Run([]string{"plugin", "install", src}); code != ExitOK {
		t.Fatalf("Run: %d", code)
	}
	dst := filepath.Join(tmp, "dia", "plugins", "hello")
	if _, err := os.Stat(filepath.Join(dst, "plugin.json")); err != nil {
		t.Errorf("not installed: %v", err)
	}
}

func TestPluginInfo_NotFound(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	if code := Run([]string{"plugin", "info", "missing"}); code != ExitNotFound {
		t.Errorf("expected ExitNotFound, got %d", code)
	}
}

func TestIsValidPluginID(t *testing.T) {
	good := []string{"abc", "hello", "hello-world", "abc123", "a-b-c"}
	bad := []string{"", "a", "ab", "Has-Caps", "-leading", "trailing-", "with space", "with/slash", "a--b", strings.Repeat("a", 41)}
	for _, id := range good {
		if !isValidPluginID(id) {
			t.Errorf("expected valid: %q", id)
		}
	}
	for _, id := range bad {
		if isValidPluginID(id) {
			t.Errorf("expected invalid: %q", id)
		}
	}
}
