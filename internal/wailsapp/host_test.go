package wailsapp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestHost returns a wailsHost backed by a fully started App in a
// throwaway XDG scratch space, the same setup app_test.go uses.
func newTestHost(t *testing.T) *wailsHost {
	t.Helper()
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	return &wailsHost{app: a}
}

func TestWailsHost_ListWorkspaces(t *testing.T) {
	h := newTestHost(t)
	if _, err := h.app.NewWorkspace("alpha", false); err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}

	got, err := h.ListWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("ListWorkspaces: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	m, ok := got[0].(map[string]any)
	if !ok {
		t.Fatalf("entry is %T, want map[string]any", got[0])
	}
	if m["name"] != "alpha" {
		t.Errorf("name = %v, want alpha", m["name"])
	}
}

func TestWailsHost_GetWorkspace_NotFound(t *testing.T) {
	h := newTestHost(t)
	if _, err := h.GetWorkspace(context.Background(), "does-not-exist"); err == nil {
		t.Error("expected an error for an unknown workspace")
	}
}

func TestWailsHost_Doctor(t *testing.T) {
	h := newTestHost(t)
	checks, err := h.Doctor(context.Background())
	if err != nil {
		t.Fatalf("Doctor: %v", err)
	}
	if len(checks) == 0 {
		t.Error("expected at least one doctor check")
	}
}

func TestWailsHost_Paths(t *testing.T) {
	h := newTestHost(t)
	got, err := h.Paths(context.Background())
	if err != nil {
		t.Fatalf("Paths: %v", err)
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("Paths returned %T, want map[string]any", got)
	}
	if len(m) == 0 {
		t.Error("expected a non-empty paths map")
	}
}

func TestWailsHost_ThemeRoundTrip(t *testing.T) {
	h := newTestHost(t)
	ctx := context.Background()
	if err := h.SetTheme(ctx, "dracula"); err != nil {
		t.Fatalf("SetTheme: %v", err)
	}
	got, err := h.GetTheme(ctx)
	if err != nil {
		t.Fatalf("GetTheme: %v", err)
	}
	if got != "dracula" {
		t.Errorf("GetTheme = %q, want dracula", got)
	}
}

func TestWailsHost_CustomThemeRoundTrip(t *testing.T) {
	h := newTestHost(t)
	ctx := context.Background()
	info := map[string]any{
		"name":         "midnight",
		"color_scheme": "dark",
		"colors":       map[string]any{"primary": "#112233"},
	}
	if err := h.SetCustomTheme(ctx, info); err != nil {
		t.Fatalf("SetCustomTheme: %v", err)
	}

	list, err := h.ListCustomThemes(ctx)
	if err != nil {
		t.Fatalf("ListCustomThemes: %v", err)
	}
	found := false
	for _, item := range list {
		ct, ok := item.(CustomThemeInfo)
		if ok && ct.Name == "midnight" {
			found = true
		}
	}
	if !found {
		t.Errorf("midnight theme not found in %+v", list)
	}

	if err := h.DeleteCustomTheme(ctx, "midnight"); err != nil {
		t.Fatalf("DeleteCustomTheme: %v", err)
	}
	list, err = h.ListCustomThemes(ctx)
	if err != nil {
		t.Fatalf("ListCustomThemes after delete: %v", err)
	}
	for _, item := range list {
		if ct, ok := item.(CustomThemeInfo); ok && ct.Name == "midnight" {
			t.Error("midnight theme still present after delete")
		}
	}
}

func TestWailsHost_SetCustomTheme_RejectsInvalid(t *testing.T) {
	h := newTestHost(t)
	err := h.SetCustomTheme(context.Background(), map[string]any{
		"name":         "bad",
		"color_scheme": "not-a-scheme",
		"colors":       map[string]any{"primary": "#112233"},
	})
	if err == nil {
		t.Error("expected an error for an invalid color_scheme")
	}
}

func TestWailsHost_NewWorkspace(t *testing.T) {
	h := newTestHost(t)
	path, err := h.NewWorkspace(context.Background(), "viaHost")
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}
	if path == "" {
		t.Error("expected a non-empty path")
	}
}

func TestWailsHost_Exec_Success(t *testing.T) {
	h := newTestHost(t)
	out, err := h.Exec(context.Background(), "go", []string{"version"})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if !strings.Contains(out, "go version") {
		t.Errorf("output = %q, want it to contain %q", out, "go version")
	}
}

func TestWailsHost_Exec_NonZeroExitWrapsOutput(t *testing.T) {
	h := newTestHost(t)
	out, err := h.Exec(context.Background(), "go", []string{"this-is-not-a-real-subcommand"})
	if err == nil {
		t.Fatal("expected an error for an invalid subcommand")
	}
	if out == "" {
		t.Error("expected non-empty output from the failed command")
	}
	if !strings.Contains(err.Error(), out) {
		t.Errorf("error %q does not contain the command output %q", err.Error(), out)
	}
}

func TestWailsHost_Exec_CommandNotFound(t *testing.T) {
	h := newTestHost(t)
	if _, err := h.Exec(context.Background(), "dia-does-not-exist-xyz", nil); err == nil {
		t.Error("expected an error for a nonexistent binary")
	}
}

func TestWailsHost_Fetch_GETParsesJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"n":3}`))
	}))
	defer srv.Close()

	h := newTestHost(t)
	got, err := h.Fetch(context.Background(), srv.URL, map[string]any{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("Fetch returned %T, want map[string]any", got)
	}
	if m["ok"] != true || m["n"] != float64(3) {
		t.Errorf("got %+v", m)
	}
}

func TestWailsHost_Fetch_POSTWithBodyAndHeaders(t *testing.T) {
	var gotMethod, gotBody, gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		gotBody = string(b)
		gotHeader = r.Header.Get("X-Test")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	h := newTestHost(t)
	_, err := h.Fetch(context.Background(), srv.URL, map[string]any{
		"method":  "POST",
		"body":    "hello",
		"headers": map[string]any{"X-Test": "abc"},
	})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if gotMethod != "POST" {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotBody != "hello" {
		t.Errorf("body = %q, want hello", gotBody)
	}
	if gotHeader != "abc" {
		t.Errorf("X-Test header = %q, want abc", gotHeader)
	}
}

func TestWailsHost_Fetch_NonJSONBodyPassesThroughAsString(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("plain text, not json"))
	}))
	defer srv.Close()

	h := newTestHost(t)
	got, err := h.Fetch(context.Background(), srv.URL, map[string]any{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got != "plain text, not json" {
		t.Errorf("got %+v, want the raw string", got)
	}
}

func TestWailsHost_Fetch_ErrorStatusReturnsBodyAndError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer srv.Close()

	h := newTestHost(t)
	got, err := h.Fetch(context.Background(), srv.URL, map[string]any{})
	if err == nil {
		t.Fatal("expected an error for a 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %q, want it to mention 500", err.Error())
	}
	if got != "boom" {
		t.Errorf("got %+v, want the error body", got)
	}
}

func TestWailsHost_Fetch_TimeoutOptCancelsSlowRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := newTestHost(t)
	_, err := h.Fetch(context.Background(), srv.URL, map[string]any{"timeout": float64(0.05)})
	if err == nil {
		t.Fatal("expected a timeout error")
	}
}
