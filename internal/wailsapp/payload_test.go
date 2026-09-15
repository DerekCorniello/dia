package wailsapp

import (
	"encoding/json"
	"strings"
	"testing"
)

// payloadBudget is the per-call JSON budget for list endpoints. It is
// deliberately far below the daemon's 4MiB line limit: the Wails IPC
// path historically choked on large single results, so lists stay lean
// and details move to GetWorkspace/GetPlugin.
const payloadBudget = 256 << 10

func marshalSize(t *testing.T, v any) int {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return len(b)
}

func TestListPayloadBudget(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())

	ws, err := a.ListWorkspaces()
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	if n := marshalSize(t, ws); n > payloadBudget {
		t.Errorf("ListWorkspaces payload %d bytes exceeds %d budget", n, payloadBudget)
	}
	if n := marshalSize(t, a.ListPlugins()); n > payloadBudget {
		t.Errorf("ListPlugins payload %d bytes exceeds %d budget", n, payloadBudget)
	}
	if n := marshalSize(t, a.ListInstances()); n > payloadBudget {
		t.Errorf("ListInstances payload %d bytes exceeds %d budget", n, payloadBudget)
	}
}

func TestListPluginsOmitsHeavyFields(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	raw, err := json.Marshal(a.ListPlugins())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, heavy := range []string{"longDescription", "configSchema", "profile_dir", "seed_dir"} {
		if strings.Contains(string(raw), heavy) {
			t.Errorf("ListPlugins payload contains heavy field %q", heavy)
		}
	}
}

func TestPaginatedLists(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	if _, err := a.NewWorkspace("alpha", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := a.NewWorkspace("beta", ""); err != nil {
		t.Fatal(err)
	}
	page, err := a.ListWorkspacesPaginated(1, 0)
	if err != nil {
		t.Fatalf("ListWorkspacesPaginated: %v", err)
	}
	if len(page) != 1 {
		t.Errorf("limit=1 returned %d workspaces", len(page))
	}
	empty, err := a.ListWorkspacesPaginated(10, 1<<20)
	if err != nil {
		t.Fatalf("ListWorkspacesPaginated offset past end: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("offset past end returned %d, want 0", len(empty))
	}
	if got := a.ListInstancesPaginated(10, 1<<20); len(got) != 0 {
		t.Errorf("ListInstancesPaginated offset past end returned %d, want 0", len(got))
	}
}
