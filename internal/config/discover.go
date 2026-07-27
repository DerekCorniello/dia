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

	// Roots are additional directories to scan for .dia.yaml and
	// .dia/*.yaml files. Every root is scanned unconditionally,
	// so workspaces are visible regardless of the current directory.
	Roots []string

	// StopAt is a directory at which to stop the project-local walk
	// (typically the filesystem root or a git toplevel). Optional.
	StopAt string
}

// Discover loads all workspaces from the global dir, every root,
// and the CWD walk-up. Every discovered workspace is returned;
// name collisions are not shadowed -- each Source carries its full
// path so callers can disambiguate.
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

	seen := map[string]bool{}
	var sources []Source

	// collect scans a directory for .yaml/.yml workspace files and
	// appends them as sources. The Local flag is set when the path
	// falls under one of the roots or the CWD walk-up.
	collect := func(dir string, local bool) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
				continue
			}
			p := filepath.Join(dir, name)
			if seen[p] {
				continue
			}
			seen[p] = true
			w, err := Load(p)
			if err != nil {
				return
			}
			sources = append(sources, Source{Workspace: w, Path: p, Local: local})
		}
	}

	// Global: glob *.yaml in opts.GlobalDir.
	collect(opts.GlobalDir, false)

	// Roots: every root is scanned for .dia.yaml and .dia/*.yaml.
	for _, root := range opts.Roots {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		// .dia.yaml at the root itself.
		localPath := filepath.Join(absRoot, ProjectLocalFile)
		if _, err := os.Stat(localPath); err == nil && !seen[localPath] {
			seen[localPath] = true
			w, err := Load(localPath)
			if err != nil {
				continue
			}
			sources = append(sources, Source{Workspace: w, Path: localPath, Local: true})
		}
		// .dia/*.yaml inside the root.
		collect(filepath.Join(absRoot, LocalDirName), true)
	}

	// Project-local: walk up from CWD looking for .dia.yaml.
	if local := findProjectLocal(opts.CWD, opts.StopAt); local != "" && !seen[local] {
		seen[local] = true
		w, err := Load(local)
		if err == nil {
			sources = append(sources, Source{Workspace: w, Path: local, Local: true})
		}
	}

	// Project-local: glob .dia/*.yaml in CWD.
	collect(filepath.Join(opts.CWD, LocalDirName), true)

	// Stable, sorted output.
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Workspace.Name < sources[j].Workspace.Name
	})
	return sources, nil
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
