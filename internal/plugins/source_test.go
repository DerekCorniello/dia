package plugins

import (
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fileURL returns a hostless file:// URL for a local path. A Windows
// drive letter must sit behind a leading slash, or git treats it as a
// host (file://C:\... hangs the clone); slashes must be forward.
func fileURL(dir string) string {
	p := filepath.ToSlash(dir)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

// urlPath reverses fileURL into a filesystem path.
func urlPath(u string) (string, error) {
	parsed, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	p := parsed.Path
	if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = strings.TrimPrefix(p, "/")
	}
	return filepath.FromSlash(p), nil
}

func TestParseInstallSource(t *testing.T) {
	existing := t.TempDir()

	tests := []struct {
		name    string
		raw     string
		ref     string
		want    InstallSource
		wantErr string
	}{
		{
			name: "existing directory is a path",
			raw:  existing,
			want: InstallSource{Kind: InstallPath, Path: existing},
		},
		{
			name: "https url",
			raw:  "https://github.com/owner/repo",
			want: InstallSource{Kind: InstallGit, URL: "https://github.com/owner/repo"},
		},
		{
			name: "ssh url",
			raw:  "ssh://git@github.com/owner/repo",
			want: InstallSource{Kind: InstallGit, URL: "ssh://git@github.com/owner/repo"},
		},
		{
			name: "file url",
			raw:  "file:///srv/repos/plugin",
			want: InstallSource{Kind: InstallGit, URL: "file:///srv/repos/plugin"},
		},
		{
			name: "scp style",
			raw:  "git@github.com:owner/repo.git",
			want: InstallSource{Kind: InstallGit, URL: "git@github.com:owner/repo.git"},
		},
		{
			name: "bare host path is normalized to https",
			raw:  "github.com/owner/repo",
			want: InstallSource{Kind: InstallGit, URL: "https://github.com/owner/repo"},
		},
		{
			name: "self-hosted host path",
			raw:  "git.example.org/team/plugin",
			want: InstallSource{Kind: InstallGit, URL: "https://git.example.org/team/plugin"},
		},
		{
			name: "ref is carried onto git sources",
			raw:  "github.com/owner/repo",
			ref:  "v1.2.0",
			want: InstallSource{Kind: InstallGit, URL: "https://github.com/owner/repo", Ref: "v1.2.0"},
		},
		{
			// Indistinguishable from a relative path, so it must not
			// be silently guessed at as a GitHub shorthand.
			name:    "bare owner/repo is rejected",
			raw:     "owner/repo",
			wantErr: "not an existing directory",
		},
		{
			name:    "nonexistent local path is rejected",
			raw:     "./no/such/dir",
			wantErr: "not an existing directory",
		},
		{
			name:    "empty",
			raw:     "",
			wantErr: "empty",
		},
		{
			name:    "ref on a local path is a mistake worth reporting",
			raw:     existing,
			ref:     "v1",
			wantErr: "--ref applies to git sources",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseInstallSource(tt.raw, tt.ref)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected an error containing %q, got %+v", tt.wantErr, got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

// gitRepo creates a real local git repository containing a plugin, and
// returns a file:// URL for it. Using real git against a local repo
// keeps the clone path under test without touching the network.
func gitRepo(t *testing.T, manifest string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("plugin.json", manifest)
	write("index.js", "module.exports = { getData: function () { return []; } };")

	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"add", "."},
		{"commit", "-q", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return fileURL(dir)
}

const gitManifest = `{"id":"from-git","name":"From Git","version":"0.1.0","ui":{"type":"list","title":"T"}}`

func TestInstallFrom_GitClone(t *testing.T) {
	url := gitRepo(t, gitManifest)
	mgr, err := NewManager(t.TempDir(), &fakeHost{})
	if err != nil {
		t.Fatal(err)
	}
	src, err := ParseInstallSource(url, "")
	if err != nil {
		t.Fatal(err)
	}
	dst, err := mgr.InstallFrom(src, false, "")
	if err != nil {
		t.Fatalf("InstallFrom: %v", err)
	}
	if filepath.Base(dst) != "from-git" {
		t.Errorf("installed to %s, want a dir named for the plugin id", dst)
	}
	if _, err := os.Stat(filepath.Join(dst, "index.js")); err != nil {
		t.Errorf("plugin files not copied: %v", err)
	}
	// The clone's history is not part of the plugin.
	if _, err := os.Stat(filepath.Join(dst, ".git")); !os.IsNotExist(err) {
		t.Error(".git should not be copied into the plugins dir")
	}
}

func TestInstallFrom_RecordsProvenance(t *testing.T) {
	url := gitRepo(t, gitManifest)
	mgr, err := NewManager(t.TempDir(), &fakeHost{})
	if err != nil {
		t.Fatal(err)
	}
	src, _ := ParseInstallSource(url, "")
	dst, err := mgr.InstallFrom(src, false, "")
	if err != nil {
		t.Fatal(err)
	}
	prov, ok, err := ReadProvenance(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected provenance to be recorded for a git install")
	}
	if prov.URL != url {
		t.Errorf("URL = %q, want %q", prov.URL, url)
	}
}

func TestInstallFrom_LocalPathRecordsNoProvenance(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "plugin.json"), []byte(gitManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	mgr, err := NewManager(t.TempDir(), &fakeHost{})
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := ParseInstallSource(src, "")
	dst, err := mgr.InstallFrom(parsed, false, "")
	if err != nil {
		t.Fatal(err)
	}
	_, ok, err := ReadProvenance(dst)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("a local-path install has no remote to record")
	}
}

func TestInstallFrom_BadURLFails(t *testing.T) {
	mgr, err := NewManager(t.TempDir(), &fakeHost{})
	if err != nil {
		t.Fatal(err)
	}
	src := InstallSource{Kind: InstallGit, URL: "file:///nonexistent/repo/path"}
	if _, err := mgr.InstallFrom(src, false, ""); err == nil {
		t.Error("expected a clone failure for a nonexistent repo")
	}
}

func TestInstallFrom_RepoWithoutManifestFails(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("not a plugin"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"}, {"config", "user.email", "t@e.com"}, {"config", "user.name", "T"},
		{"add", "."}, {"commit", "-q", "-m", "x"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	mgr, err := NewManager(t.TempDir(), &fakeHost{})
	if err != nil {
		t.Fatal(err)
	}
	src, _ := ParseInstallSource(fileURL(dir), "")
	_, err = mgr.InstallFrom(src, false, "")
	if err == nil {
		t.Fatal("expected an error for a repo with no plugin.json")
	}
	if !strings.Contains(err.Error(), "plugin.json") {
		t.Errorf("error %q should mention the missing manifest", err.Error())
	}
}

func TestInspectSource_ReturnsManifestWithoutInstalling(t *testing.T) {
	url := gitRepo(t, gitManifest)
	globalDir := t.TempDir()
	mgr, err := NewManager(globalDir, &fakeHost{})
	if err != nil {
		t.Fatal(err)
	}
	src, _ := ParseInstallSource(url, "")
	manifest, dir, cleanup, err := mgr.InspectSource(src)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ID != "from-git" {
		t.Errorf("ID = %q, want from-git", manifest.ID)
	}
	if _, err := os.Stat(filepath.Join(dir, "plugin.json")); err != nil {
		t.Errorf("materialized dir should hold the plugin: %v", err)
	}
	// Inspecting must not install.
	if _, err := os.Stat(filepath.Join(globalDir, "from-git")); !os.IsNotExist(err) {
		t.Error("InspectSource must not install the plugin")
	}
	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("cleanup should remove the temp clone")
	}
}

func TestInstallMaterialized_DoesNotRecloneAndRecordsProvenance(t *testing.T) {
	url := gitRepo(t, gitManifest)
	mgr, err := NewManager(t.TempDir(), &fakeHost{})
	if err != nil {
		t.Fatal(err)
	}
	src, _ := ParseInstallSource(url, "")
	_, dir, cleanup, err := mgr.InspectSource(src)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	dst, err := mgr.InstallMaterialized(dir, false, "", &Provenance{URL: src.URL})
	if err != nil {
		t.Fatalf("InstallMaterialized: %v", err)
	}
	prov, ok, err := ReadProvenance(dst)
	if err != nil || !ok {
		t.Fatalf("provenance: %v ok=%v", err, ok)
	}
	if prov.URL != url {
		t.Errorf("URL = %q, want %q", prov.URL, url)
	}
}

func TestUpdate_ReclonesAndPreservesGrants(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	url := gitRepo(t, gitManifest)
	repoDir, err := urlPath(url)
	if err != nil {
		t.Fatal(err)
	}

	mgr, err := NewManager(t.TempDir(), &fakeHost{})
	if err != nil {
		t.Fatal(err)
	}
	src, _ := ParseInstallSource(url, "")
	if _, err := mgr.InstallFrom(src, false, ""); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Enable("from-git"); err != nil {
		t.Fatalf("Enable: %v", err)
	}

	// Publish a new version upstream.
	newManifest := `{"id":"from-git","name":"From Git","version":"0.2.0","ui":{"type":"list","title":"T"}}`
	if err := os.WriteFile(filepath.Join(repoDir, "plugin.json"), []byte(newManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-q", "-m", "bump"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	if _, err := mgr.Update("from-git"); err != nil {
		t.Fatalf("Update: %v", err)
	}

	var found *Loaded
	for _, l := range mgr.List() {
		if l.Manifest.ID == "from-git" {
			l := l
			found = &l
		}
	}
	if found == nil {
		t.Fatal("plugin missing after update")
	}
	if found.Manifest.Version != "0.2.0" {
		t.Errorf("Version = %q, want 0.2.0", found.Manifest.Version)
	}
	// An update is not a reinstall: the user's enabled state survives.
	if !found.Enabled {
		t.Error("enabled state should be preserved across an update")
	}
}

func TestUpdate_LocalInstallHasNothingToUpdateFrom(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "plugin.json"), []byte(gitManifest), 0o644); err != nil {
		t.Fatal(err)
	}
	mgr, err := NewManager(t.TempDir(), &fakeHost{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.Install(src); err != nil {
		t.Fatal(err)
	}
	_, err = mgr.Update("from-git")
	if err == nil {
		t.Fatal("expected an error updating a path-installed plugin")
	}
	if !strings.Contains(err.Error(), "local path") {
		t.Errorf("error %q should explain there is no recorded source", err.Error())
	}
}

func TestUpdate_UnknownPlugin(t *testing.T) {
	mgr, err := NewManager(t.TempDir(), &fakeHost{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.Update("nope"); err == nil {
		t.Error("expected an error for an unknown plugin")
	}
}

func TestUpdate_IDMismatchIsRefused(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	url := gitRepo(t, gitManifest)
	repoDir, err := urlPath(url)
	if err != nil {
		t.Fatal(err)
	}

	mgr, err := NewManager(t.TempDir(), &fakeHost{})
	if err != nil {
		t.Fatal(err)
	}
	src, _ := ParseInstallSource(url, "")
	dst, err := mgr.InstallFrom(src, false, "")
	if err != nil {
		t.Fatal(err)
	}

	// Upstream now claims to be a different plugin entirely.
	swapped := `{"id":"something-else","name":"X","version":"9.0.0","ui":{"type":"list","title":"T"}}`
	if err := os.WriteFile(filepath.Join(repoDir, "plugin.json"), []byte(swapped), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-q", "-m", "swap"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	_, err = mgr.Update("from-git")
	if err == nil {
		t.Fatal("expected an id-mismatch error")
	}
	// The installed plugin must survive a refused update.
	if _, statErr := os.Stat(filepath.Join(dst, "plugin.json")); statErr != nil {
		t.Errorf("the installed plugin should be untouched, got %v", statErr)
	}
}

func TestReadProvenance_MissingAndMalformed(t *testing.T) {
	dir := t.TempDir()
	if _, ok, err := ReadProvenance(dir); err != nil || ok {
		t.Errorf("missing file: got ok=%v err=%v, want ok=false err=nil", ok, err)
	}

	if err := os.WriteFile(filepath.Join(dir, provenanceFile), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadProvenance(dir); err == nil {
		t.Error("expected an error for a malformed provenance file")
	}
}
