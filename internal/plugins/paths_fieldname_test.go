package plugins

import (
	"encoding/json"
	"testing"
)

// pathsInfoShape mirrors the shape a wailsapp.PathsInfo-like value has:
// a Go struct with json tags, matching a HostAPI method that forgot to
// convert through marshalAny before returning.
type pathsInfoShape struct {
	GlobalConfigDir string `json:"global_config_dir"`
}

// TestHostValueFieldNamesThroughJS pins the contract every HostAPI
// method depends on: goja.New() here has no FieldNameMapper
// configured, so a plain Go struct crossing into JS exposes its Go
// field names (GlobalConfigDir), not its json tag
// (global_config_dir) -- a plugin author following the documented,
// JSON-keyed API gets undefined. A map[string]any is unaffected, since
// it carries no Go-side names to begin with. This is exactly why every
// HostAPI method must return the result of marshalAny rather than a
// raw struct; wailsHost.Paths once skipped it (found by testing it
// directly: p.global_config_dir came back undefined against a real
// plugin, fixed to route through marshalAny like its siblings).
func TestHostValueFieldNamesThroughJS(t *testing.T) {
	js := `module.exports = { getData: function () {
		var p = dia.paths();
		return { viaJsonKey: p.global_config_dir, viaGoName: p.GlobalConfigDir };
	} };`

	t.Run("raw struct: json tag is not reachable", func(t *testing.T) {
		host := &fakeHost{pathsOut: pathsInfoShape{GlobalConfigDir: "/home/u/.config/dia"}}
		_, mgr := setupPlugin(t, host, "raw-struct", js, []string{CapPathsRead})
		if err := mgr.Enable("raw-struct"); err != nil {
			t.Fatal(err)
		}
		v, err := mgr.Call("raw-struct", "getData", nil)
		if err != nil {
			t.Fatal(err)
		}
		m := v.(map[string]any)
		if m["viaJsonKey"] != nil {
			t.Errorf("viaJsonKey = %v, want nil (undefined) for a raw struct -- if this now passes, goja's default field mapping changed and marshalAny may no longer be required", m["viaJsonKey"])
		}
		if m["viaGoName"] != "/home/u/.config/dia" {
			t.Errorf("viaGoName = %v, want the config dir via the raw Go field name", m["viaGoName"])
		}
	})

	t.Run("marshalAny output: json tag is reachable", func(t *testing.T) {
		// The same marshal-through-JSON round trip wailsapp.marshalAny
		// does, reproduced here since it is unexported in another
		// package: encode the struct, decode into a plain
		// map[string]any, which carries the json-tagged keys and no
		// Go-side field names at all.
		b, err := json.Marshal(pathsInfoShape{GlobalConfigDir: "/home/u/.config/dia"})
		if err != nil {
			t.Fatal(err)
		}
		var marshaled any
		if err := json.Unmarshal(b, &marshaled); err != nil {
			t.Fatal(err)
		}
		host := &fakeHost{pathsOut: marshaled}
		_, mgr := setupPlugin(t, host, "marshaled", js, []string{CapPathsRead})
		if err := mgr.Enable("marshaled"); err != nil {
			t.Fatal(err)
		}
		v, err := mgr.Call("marshaled", "getData", nil)
		if err != nil {
			t.Fatal(err)
		}
		m := v.(map[string]any)
		if m["viaJsonKey"] != "/home/u/.config/dia" {
			t.Errorf("viaJsonKey = %v, want the config dir -- this is the documented, working shape every HostAPI method must return", m["viaJsonKey"])
		}
	})
}
