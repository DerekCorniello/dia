package plugins

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// cloneTimeout bounds a git clone. Without it a prompt-less clone
// against an unreachable host hangs the CLI indefinitely.
const cloneTimeout = 2 * time.Minute

// provenanceFile records where a plugin was installed from, written
// inside the installed plugin dir. Keeping it with the plugin rather
// than in the state store means it is removed with the plugin and
// cannot drift out of sync with what is on disk.
const provenanceFile = ".dia-source.json"

// InstallKind distinguishes the two ways a plugin can be installed.
type InstallKind int

const (
	// InstallPath copies an existing local directory.
	InstallPath InstallKind = iota
	// InstallGit clones a remote repository first.
	InstallGit
)

// InstallSource is a parsed `dia plugin install` argument.
type InstallSource struct {
	Kind InstallKind
	// Path is the local directory, for InstallPath.
	Path string
	// URL is the normalized clone URL, for InstallGit.
	URL string
	// Ref is an optional branch or tag to clone, for InstallGit.
	Ref string
}

// gitSchemes are the URL schemes treated as git remotes. file:// is
// included because it is how you clone a repository that is already
// on disk, which is also what makes this testable without a network.
var gitSchemes = []string{"https://", "http://", "ssh://", "git://", "file://"}

// ParseInstallSource decides whether raw names a local directory or a
// git remote. An existing directory always wins: a local path is
// unambiguous and is what the argument meant before remotes were
// supported.
//
// Recognized remotes are any URL with a git scheme, scp-style
// git@host:owner/repo, and a bare host/owner/repo whose first segment
// looks like a hostname (github.com/user/repo), which is normalized to
// https. A bare owner/repo is deliberately NOT accepted: it is
// indistinguishable from a relative path.
func ParseInstallSource(raw, ref string) (InstallSource, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return InstallSource{}, errors.New("install source is empty")
	}

	if info, err := os.Stat(raw); err == nil && info.IsDir() {
		if ref != "" {
			return InstallSource{}, fmt.Errorf("--ref applies to git sources, but %s is a local directory", raw)
		}
		return InstallSource{Kind: InstallPath, Path: raw}, nil
	}

	for _, scheme := range gitSchemes {
		if strings.HasPrefix(raw, scheme) {
			return InstallSource{Kind: InstallGit, URL: raw, Ref: ref}, nil
		}
	}
	if strings.HasPrefix(raw, "git@") {
		return InstallSource{Kind: InstallGit, URL: raw, Ref: ref}, nil
	}
	if looksLikeHostPath(raw) {
		return InstallSource{Kind: InstallGit, URL: "https://" + raw, Ref: ref}, nil
	}

	return InstallSource{}, fmt.Errorf(
		"%q is not an existing directory and not a recognized git URL "+
			"(use https://host/owner/repo, git@host:owner/repo, or host.tld/owner/repo)", raw)
}

// looksLikeHostPath reports whether s is a bare host/owner/repo, i.e.
// its first segment contains a dot (a hostname) and something follows.
func looksLikeHostPath(s string) bool {
	parts := strings.Split(s, "/")
	if len(parts) < 2 {
		return false
	}
	host := parts[0]
	if !strings.Contains(host, ".") || strings.HasPrefix(host, ".") {
		return false
	}
	return parts[1] != ""
}

// materialize returns a directory holding the plugin's files. For a
// git source it clones into a fresh temp dir and returns a cleanup
// func; for a local path it returns the path and a no-op.
func (s InstallSource) materialize() (dir string, cleanup func(), err error) {
	if s.Kind == InstallPath {
		return s.Path, func() {}, nil
	}
	tmp, err := os.MkdirTemp("", "dia-plugin-clone-")
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir: %w", err)
	}
	cleanup = func() { _ = os.RemoveAll(tmp) }
	target := filepath.Join(tmp, "repo")
	if err := gitClone(s.URL, s.Ref, target); err != nil {
		cleanup()
		return "", nil, err
	}
	return target, cleanup, nil
}

// gitClone shells out to the user's git. Shelling out beats taking on
// a git library dependency for one shallow clone, and it means the
// user's existing credential helpers and SSH config just work.
func gitClone(url, ref, dst string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("git is required to install a plugin from a URL, but it was not found on PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), cloneTimeout)
	defer cancel()

	args := []string{"clone"}
	// Shallow clone only for remotes. --depth makes git negotiate a
	// pack over the upload-pack protocol, which adds nothing for a
	// file:// source already on disk and is a known intermittent hang
	// on Windows; a local clone via the filesystem is what we want.
	if !strings.HasPrefix(url, "file://") {
		args = append(args, "--depth", "1")
	}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	// "--" stops a URL that begins with a dash from being read as a flag.
	args = append(args, "--", url, dst)

	cmd := exec.CommandContext(ctx, "git", args...)
	// Fail fast instead of blocking on a credential prompt the CLI
	// cannot answer.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf("git clone %s timed out after %s", url, cloneTimeout)
	}
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if ref != "" {
			return fmt.Errorf("git clone %s at ref %q: %v\n%s", url, ref, err, msg)
		}
		return fmt.Errorf("git clone %s: %v\n%s", url, err, msg)
	}
	return nil
}
