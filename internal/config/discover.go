package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	// GlobalDirName is the subdirectory under $XDG_CONFIG_HOME that
	// holds global workspace YAMLs.
	GlobalDirName = "dia/workspaces"

	// ProjectLocalFile is the per-repo config file dia looks for.
	ProjectLocalFile = ".dia.yaml"

	// LocalDirName is the project-local directory for dia files
	// (workspace YAMLs, plugins).
	LocalDirName = ".dia"
)

// Source describes a discovered workspace and where it came from.
type Source struct {
	Workspace *Workspace
	Path      string // absolute path to the YAML file
	Local     bool   // true for project-local; false for global
}

// DiscoverOptions controls how Discover searches for workspaces.
type DiscoverOptions struct {
	// CWD is the directory to start the project-local walk-up from.
	// If empty, os.Getwd is used.
	CWD string

	// GlobalDir is the absolute path to the global workspace dir.
	// If empty, the default XDG path is used.
	GlobalDir string

	// StopAt is a directory at which to stop the project-local walk
	// (typically the filesystem root or a git toplevel). Optional.
	StopAt string
}

// Discover loads global workspaces and, if a .dia.yaml is found by
// walking up from CWD, the project-local workspace. Project-local
// shadows global on name collision.
func Discover(opts DiscoverOptions) ([]Source, error) {
	if opts.CWD == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get cwd: %w", err)
		}
		opts.CWD = cwd
	}
	if opts.GlobalDir == "" {
		opts.GlobalDir = defaultGlobalDir()
	}

	byName := make(map[string]Source)
	var paths []string

	// Global: glob *.yaml in opts.GlobalDir.
	entries, err := os.ReadDir(opts.GlobalDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read global dir %s: %w", opts.GlobalDir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		paths = append(paths, filepath.Join(opts.GlobalDir, name))
	}

	// Project-local: walk up from CWD looking for .dia.yaml.
	localPaths := map[string]bool{}
	if local := findProjectLocal(opts.CWD, opts.StopAt); local != "" {
		paths = append(paths, local)
		localPaths[local] = true
	}

	// Project-local: glob .dia/*.yaml in CWD.
	diaDir := filepath.Join(opts.CWD, LocalDirName)
	if entries, err := os.ReadDir(diaDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
				continue
			}
			p := filepath.Join(diaDir, name)
			paths = append(paths, p)
			localPaths[p] = true
		}
	}

	// Load and dedupe.
	for _, p := range paths {
		w, err := Load(p)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", p, err)
		}
		src := Source{Workspace: w, Path: p, Local: localPaths[p]}
		if _, ok := byName[w.Name]; ok {
			// Project-local wins on collision. Local files
			// always come after global in the path list, so
			// when both define the same name, the local one
			// overwrites the global entry.
		}
		byName[w.Name] = src
	}

	// Stable, sorted output.
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]Source, 0, len(byName))
	for _, n := range names {
		out = append(out, byName[n])
	}
	return out, nil
}

// FindLocal returns the path of the .dia.yaml walking up from dir, or
// empty string if none is found.
func FindLocal(dir string) string {
	return findProjectLocal(dir, "")
}

func findProjectLocal(start, stopAt string) string {
	dir, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	stop := stopAt
	if stop == "" {
		stop = string(filepath.Separator)
	}
	for {
		candidate := filepath.Join(dir, ProjectLocalFile)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		} else if !errors.Is(err, fs.ErrNotExist) {
			return ""
		}
		if dir == stop {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func defaultGlobalDir() string {
	return DefaultGlobalDir()
}

// DefaultGlobalDir is the exported form of defaultGlobalDir. It
// returns the absolute path to the global workspace directory,
// honoring $XDG_CONFIG_HOME and falling back to ~/.config.
func DefaultGlobalDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(".", GlobalDirName)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, GlobalDirName)
}
