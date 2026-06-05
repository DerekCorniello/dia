package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValid(t *testing.T) {
	w, err := Load("testdata/backend-go.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if w.Name != "backend-go" {
		t.Errorf("name: got %q want %q", w.Name, "backend-go")
	}
	if w.Version != 1 {
		t.Errorf("version: got %d want 1", w.Version)
	}
	if len(w.Apps) != 3 {
		t.Fatalf("apps: got %d want 3", len(w.Apps))
	}
	if w.Apps[0].Type != "editor" || w.Apps[0].Cmd != "code ." {
		t.Errorf("apps[0]: %+v", w.Apps[0])
	}
	if w.Apps[2].Url != "http://localhost:8080" {
		t.Errorf("apps[2].url: got %q", w.Apps[2].Url)
	}
}

func TestLoadMissingApps(t *testing.T) {
	_, err := Load("testdata/missing-apps.yaml")
	if err == nil {
		t.Fatal("expected error for missing apps")
	}
	if !IsValidationError(err) {
		t.Fatalf("expected ValidationErrors, got %T: %v", err, err)
	}
	ve := err.(ValidationErrors)
	if len(ve) != 1 || ve[0].Path != "workspace.apps" {
		t.Fatalf("unexpected errors: %v", ve)
	}
}

func TestLoadBadName(t *testing.T) {
	_, err := Load("testdata/bad-name.yaml")
	if err == nil {
		t.Fatal("expected error for bad name")
	}
	ve := err.(ValidationErrors)
	found := false
	for _, e := range ve {
		if e.Path == "workspace.name" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected workspace.name error, got: %v", ve)
	}
}

func TestLoadEditorNoCmd(t *testing.T) {
	_, err := Load("testdata/editor-no-cmd.yaml")
	if err == nil {
		t.Fatal("expected error for editor without cmd")
	}
	ve := err.(ValidationErrors)
	found := false
	for _, e := range ve {
		if e.Path == "workspace.apps[0].cmd" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected apps[0].cmd error, got: %v", ve)
	}
}

func TestLoadUnknownTypeAccepted(t *testing.T) {
	w, err := Load("testdata/unknown-type.yaml")
	if err != nil {
		t.Fatalf("unknown types must be accepted (may be plugins), got: %v", err)
	}
	if w.Apps[0].Type != "my-custom-plugin" {
		t.Fatalf("type not preserved: %q", w.Apps[0].Type)
	}
}

func TestLoadFutureVersion(t *testing.T) {
	_, err := Load("testdata/future-version.yaml")
	if err == nil {
		t.Fatal("expected error for newer config version")
	}
	if !strings.Contains(err.Error(), "newer than") {
		t.Fatalf("expected version error, got: %v", err)
	}
}

func TestLoadUnversionedDefaults(t *testing.T) {
	_, err := Load("testdata/bad-name.yaml")
	if err == nil {
		t.Fatal("expected name error")
	}
	// bad-name.yaml is missing the version key; the loader should
	// not treat that as a problem. Confirm via the error contents.
	ve, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("not ValidationErrors: %T", err)
	}
	for _, e := range ve {
		if e.Path == "workspace.version" {
			t.Fatalf("missing version should not error: %v", ve)
		}
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("testdata/does-not-exist.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist in chain, got: %v", err)
	}
}

func TestLoadMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(bad, []byte(":\n  - : :\n  not valid: ["), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(bad)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestValidateNil(t *testing.T) {
	if err := Validate(nil); err == nil {
		t.Fatal("expected error for nil workspace")
	}
}

func TestValidateBrowserBadScheme(t *testing.T) {
	w := &Workspace{
		Version: 1,
		Name:    "x",
		Apps:    []App{{Type: "browser", Url: "ftp://example.com"}},
	}
	err := Validate(w)
	if err == nil {
		t.Fatal("expected error for ftp:// url")
	}
}

func TestValidName(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", false},
		{"a", true},
		{"abc", true},
		{"abc-def", true},
		{"abc-1", true},
		{"1abc", true},
		{"-abc", false},
		{"ABC", false},
		{"abc_def", false},
		{"abc def", false},
	}
	for _, c := range cases {
		if got := validName(c.in); got != c.want {
			t.Errorf("validName(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
