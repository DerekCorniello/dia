package cli

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitPluginRepo builds a real local git repo holding a plugin and
// returns a file:// URL for it, so the install path is exercised end
// to end without network access.
func gitPluginRepo(t *testing.T, manifest string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.js"),
		[]byte("module.exports = { getData: function () { return []; } };"), 0o644); err != nil {
		t.Fatal(err)
	}
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
	// A file:// URL needs a hostless triple-slash form with a leading
	// path slash; on Windows a drive letter must not be parsed as the
	// host (file://C:\... hangs the clone). Slashes must be forward.
	p := filepath.ToSlash(dir)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

// urlPath reverses a file:// URL from gitPluginRepo back into the
// filesystem path the repo lives at.
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

const readOnlyManifest = `{"id":"readonly","name":"Read Only","version":"0.1.0",` +
	`"capabilities":["workspaces:read"],"ui":{"type":"list","title":"T"}}`

const mutatingManifest = `{"id":"mutator","name":"Mutator","version":"0.1.0",` +
	`"capabilities":["workspaces:read","cmd:exec"],"ui":{"type":"list","title":"T"}}`

// installArgs runs `dia plugin install` with the given stdin, and
// returns the exit code plus combined output.
func installArgs(t *testing.T, stdin string, args ...string) (int, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := runWithIO(append([]string{"plugin", "install"}, args...),
		strings.NewReader(stdin), &out, &errOut)
	return code, out.String() + errOut.String()
}

func TestPluginInstall_FromGitURL(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	url := gitPluginRepo(t, readOnlyManifest)

	code, out := installArgs(t, "", url)
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, out)
	}
	dst := filepath.Join(tmp, "dia", "plugins", "readonly")
	if _, err := os.Stat(filepath.Join(dst, "plugin.json")); err != nil {
		t.Errorf("not installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "index.js")); err != nil {
		t.Errorf("plugin files missing: %v", err)
	}
}

// A read-only plugin installs without a prompt: nothing it can do
// warrants stopping the user.
func TestPluginInstall_ReadOnlyNeedsNoConfirmation(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	url := gitPluginRepo(t, readOnlyManifest)

	code, out := installArgs(t, "", url)
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, out)
	}
	if !strings.Contains(out, "workspaces:read") {
		t.Errorf("output should list the requested capabilities:\n%s", out)
	}
}

func TestPluginInstall_MutatingCapabilityPromptsAndAccepts(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	url := gitPluginRepo(t, mutatingManifest)

	code, out := installArgs(t, "y\n", url)
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, out)
	}
	if !strings.Contains(out, "(mutating)") {
		t.Errorf("output should flag the mutating capability:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(tmp, "dia", "plugins", "mutator", "plugin.json")); err != nil {
		t.Errorf("not installed after confirmation: %v", err)
	}
}

func TestPluginInstall_MutatingCapabilityDeclined(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	url := gitPluginRepo(t, mutatingManifest)

	code, _ := installArgs(t, "n\n", url)
	if code == ExitOK {
		t.Fatal("declining the prompt must not install the plugin")
	}
	if _, err := os.Stat(filepath.Join(tmp, "dia", "plugins", "mutator")); !os.IsNotExist(err) {
		t.Error("plugin dir should not exist after a declined install")
	}
}

// An empty stdin (no tty, no answer) must not be read as consent.
func TestPluginInstall_EmptyInputIsNotConsent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	url := gitPluginRepo(t, mutatingManifest)

	code, _ := installArgs(t, "", url)
	if code == ExitOK {
		t.Fatal("empty input must not be treated as yes")
	}
}

func TestPluginInstall_YesFlagSkipsPrompt(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	url := gitPluginRepo(t, mutatingManifest)

	code, out := installArgs(t, "", "--yes", url)
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, out)
	}
	if _, err := os.Stat(filepath.Join(tmp, "dia", "plugins", "mutator", "plugin.json")); err != nil {
		t.Errorf("not installed with --yes: %v", err)
	}
}

// --json has no one to prompt, so a mutating plugin must fail loudly
// rather than install unattended.
func TestPluginInstall_JSONRequiresYesForMutating(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	url := gitPluginRepo(t, mutatingManifest)

	var out, errOut bytes.Buffer
	code := runWithIO([]string{"--json", "plugin", "install", url},
		strings.NewReader(""), &out, &errOut)
	if code == ExitOK {
		t.Fatal("expected a failure without --yes in json mode")
	}
	if !strings.Contains(errOut.String(), "--yes") {
		t.Errorf("error should point at --yes:\n%s", errOut.String())
	}
}

func TestPluginInstall_UnrecognizedSourceIsRejected(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	code, out := installArgs(t, "", "owner/repo")
	if code == ExitOK {
		t.Fatal("a bare owner/repo is ambiguous and must be rejected")
	}
	if !strings.Contains(out, "not an existing directory") {
		t.Errorf("error should explain the accepted forms:\n%s", out)
	}
}

func TestPluginInstall_RefOnLocalPathIsRejected(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "plugin.json"), []byte(readOnlyManifest), 0o644); err != nil {
		t.Fatal(err)
	}

	code, out := installArgs(t, "", "--ref", "v1", src)
	if code == ExitOK {
		t.Fatal("--ref with a local path is a mistake and should be reported")
	}
	if !strings.Contains(out, "--ref applies to git sources") {
		t.Errorf("unexpected error:\n%s", out)
	}
}

func TestPluginInstall_AtRef(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	url := gitPluginRepo(t, readOnlyManifest)
	repoDir, err := urlPath(url)
	if err != nil {
		t.Fatal(err)
	}

	// Tag the current commit, then move the branch past it so the tag
	// and the default branch differ.
	for _, args := range [][]string{
		{"tag", "v0.1.0"},
		{"commit", "-q", "--allow-empty", "-m", "later"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	code, out := installArgs(t, "", "--ref", "v0.1.0", url)
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, out)
	}
	if _, err := os.Stat(filepath.Join(tmp, "dia", "plugins", "readonly", "plugin.json")); err != nil {
		t.Errorf("not installed at ref: %v", err)
	}
}

func TestPluginInstall_MissingRefFails(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	url := gitPluginRepo(t, readOnlyManifest)

	code, _ := installArgs(t, "", "--ref", "no-such-tag", url)
	if code == ExitOK {
		t.Fatal("expected a failure for a nonexistent ref")
	}
}

func TestPluginUpdate_UnknownPlugin(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	if code := Run([]string{"plugin", "update", "nope"}); code == ExitOK {
		t.Error("expected a failure updating an unknown plugin")
	}
}

func TestPluginUpdate_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	url := gitPluginRepo(t, readOnlyManifest)
	repoDir, err := urlPath(url)
	if err != nil {
		t.Fatal(err)
	}

	if code, out := installArgs(t, "", url); code != ExitOK {
		t.Fatalf("install: exit %d: %s", code, out)
	}

	bumped := strings.Replace(readOnlyManifest, `"version":"0.1.0"`, `"version":"0.9.0"`, 1)
	if err := os.WriteFile(filepath.Join(repoDir, "plugin.json"), []byte(bumped), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-q", "-m", "bump"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	if code := Run([]string{"plugin", "update", "readonly"}); code != ExitOK {
		t.Fatalf("update returned %d", code)
	}
	data, err := os.ReadFile(filepath.Join(tmp, "dia", "plugins", "readonly", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "0.9.0") {
		t.Errorf("plugin was not updated: %s", data)
	}
}

func TestPluginEnable_GrantsOnlyWhatTheManifestRequests(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	url := gitPluginRepo(t, mutatingManifest) // requests workspaces:read, cmd:exec

	if code, out := installArgs(t, "", "--yes", url); code != ExitOK {
		t.Fatalf("install: %d %s", code, out)
	}

	var out, errOut bytes.Buffer
	// themes:write is not in the manifest and must be dropped.
	code := runWithIO([]string{"--json", "plugin", "grant", "mutator", "--caps", "cmd:exec,themes:write"},
		strings.NewReader(""), &out, &errOut)
	if code != ExitOK {
		t.Fatalf("grant: %d %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "cmd:exec") {
		t.Errorf("expected cmd:exec to be granted:\n%s", out.String())
	}
	if strings.Contains(out.String(), "themes:write") {
		t.Errorf("a capability the manifest never requested must be dropped:\n%s", out.String())
	}
}

// Installing must not grant a mutating capability, however loudly the
// manifest asks for one.
func TestPluginInstall_DoesNotGrantMutatingCapabilities(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	url := gitPluginRepo(t, mutatingManifest)

	if code, out := installArgs(t, "", "--yes", url); code != ExitOK {
		t.Fatalf("install: %d %s", code, out)
	}

	var out, errOut bytes.Buffer
	if code := runWithIO([]string{"--json", "plugin", "info", "mutator"},
		strings.NewReader(""), &out, &errOut); code != ExitOK {
		t.Fatalf("info: %d %s", code, errOut.String())
	}
	// The manifest legitimately *requests* cmd:exec; what must not
	// happen is it appearing in the granted set.
	var info struct {
		Grants []string `json:"grants"`
	}
	if err := json.Unmarshal(out.Bytes(), &info); err != nil {
		t.Fatalf("parse info json: %v\n%s", err, out.String())
	}
	for _, c := range info.Grants {
		if c == "cmd:exec" {
			t.Errorf("cmd:exec must not be granted by installing alone: %v", info.Grants)
		}
	}
}

// Granting twice is idempotent: the second grant re-persists the
// same capability set without error.
func TestPluginGrant_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)
	url := gitPluginRepo(t, readOnlyManifest)
	if code, out := installArgs(t, "", url); code != ExitOK {
		t.Fatalf("install: %d %s", code, out)
	}
	if code := Run([]string{"plugin", "grant", "readonly"}); code != ExitOK {
		t.Errorf("grant returned %d", code)
	}
	if code := Run([]string{"plugin", "grant", "readonly"}); code != ExitOK {
		t.Errorf("grant returned %d", code)
	}
}

func TestPluginGrant_UnknownPlugin(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if code := Run([]string{"plugin", "grant", "nope"}); code != ExitNotFound {
		t.Errorf("expected ExitNotFound, got %d", code)
	}
}
