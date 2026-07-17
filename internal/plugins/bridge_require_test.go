package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

// writeLib writes a file under the plugin dir, creating parent dirs.
func writeLib(t *testing.T, pdir, rel, content string) {
	t.Helper()
	full := filepath.Join(pdir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestRequireIsolatesModules verifies that two required modules each get
// their own module/exports and do not clobber one another or the entry.
func TestRequireIsolatesModules(t *testing.T) {
	host := &fakeHost{}
	entry := `
var a = require("./lib/a");
var b = require("./lib/b");
module.exports = { getData: function () { return a.name + "," + b.name; } };
`
	pdir, mgr := setupPlugin(t, host, "iso", entry, nil)
	writeLib(t, pdir, "lib/a.js", `module.exports = { name: "a" };`)
	writeLib(t, pdir, "lib/b.js", `module.exports = { name: "b" };`)
	if err := mgr.Enable("iso"); err != nil {
		t.Fatal(err)
	}
	v, err := mgr.Call("iso", "getData", nil)
	if err != nil {
		t.Fatal(err)
	}
	if v != "a,b" {
		t.Errorf("modules clobbered each other: got %v, want %q", v, "a,b")
	}
}

// TestRequireCachesByPath verifies a module is executed at most once and
// repeated requires return the same instance.
func TestRequireCachesByPath(t *testing.T) {
	host := &fakeHost{}
	entry := `
var x = require("./lib/obj");
x.v = 1;
var y = require("./lib/obj");
module.exports = { getData: function () { return (x === y) && y.v === 1; } };
`
	pdir, mgr := setupPlugin(t, host, "cache", entry, nil)
	writeLib(t, pdir, "lib/obj.js", `module.exports = {};`)
	if err := mgr.Enable("cache"); err != nil {
		t.Fatal(err)
	}
	v, err := mgr.Call("cache", "getData", nil)
	if err != nil {
		t.Fatal(err)
	}
	if v != true {
		t.Errorf("require not cached: same-instance and mutation check returned %v", v)
	}
}

// TestRequireReturnsModuleExports verifies a module that exports a
// function is callable by the requiring module.
func TestRequireReturnsModuleExports(t *testing.T) {
	host := &fakeHost{}
	entry := `
var add = require("./lib/add");
module.exports = { getData: function () { return add(2, 3); } };
`
	pdir, mgr := setupPlugin(t, host, "addfn", entry, nil)
	writeLib(t, pdir, "lib/add.js", `module.exports = function (a, b) { return a + b; };`)
	if err := mgr.Enable("addfn"); err != nil {
		t.Fatal(err)
	}
	v, err := mgr.Call("addfn", "getData", nil)
	if err != nil {
		t.Fatal(err)
	}
	if n, ok := v.(int64); !ok || n != 5 {
		t.Errorf("required function not callable: got %v (%T)", v, v)
	}
}

// TestRequireRejectsTraversal verifies that escaping the plugin dir is
// blocked, including parent traversal and absolute paths.
func TestRequireRejectsTraversal(t *testing.T) {
	host := &fakeHost{}
	entry := `
function tryRequire(p) {
	try { require(p); return "loaded"; } catch (e) { return "blocked"; }
}
module.exports = { getData: function () {
	return tryRequire("../escape") + "," + tryRequire("/etc/passwd");
} };
`
	pdir, mgr := setupPlugin(t, host, "trav", entry, nil)
	// A real file one level up that must remain unreachable.
	writeLib(t, filepath.Dir(pdir), "escape.js", `module.exports = "escaped";`)
	if err := mgr.Enable("trav"); err != nil {
		t.Fatal(err)
	}
	v, err := mgr.Call("trav", "getData", nil)
	if err != nil {
		t.Fatal(err)
	}
	if v != "blocked,blocked" {
		t.Errorf("traversal not blocked: got %v", v)
	}
}
