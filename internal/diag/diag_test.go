package diag

import "testing"

func TestPlatformOpenHelper(t *testing.T) {
	got := PlatformOpenHelper()
	switch got {
	case "open", "cmd", "xdg-open":
	default:
		t.Errorf("PlatformOpenHelper() = %q, want open/cmd/xdg-open", got)
	}
}

func TestRunChecks_AllFieldsPopulated(t *testing.T) {
	got := RunChecks("/tmp/state", "/tmp/state/state.json")
	names := map[string]bool{}
	for _, c := range got {
		names[c.Name] = true
		if c.Status != "ok" && c.Status != "warn" && c.Status != "fail" {
			t.Errorf("check %q has bad status %q", c.Name, c.Status)
		}
	}
	for _, want := range []string{"platform", "state dir", "state file", "gh cli", "url handler"} {
		if !names[want] {
			t.Errorf("missing check %q", want)
		}
	}
}
