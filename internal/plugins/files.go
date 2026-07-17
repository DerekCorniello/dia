// Package plugins: file IO helper used by both runtime and bridge.
package plugins

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const maxPluginFileBytes = 1 << 20

// ContainedRelPath reports whether p is a relative path that still
// sits inside its base directory once cleaned. The cleaned form is
// what matters: "panel/../../../etc/passwd" has no leading ".." but
// still escapes, so a prefix check on the raw string is not enough.
func ContainedRelPath(p string) bool {
	if p == "" || filepath.IsAbs(p) || strings.HasPrefix(p, "/") {
		return false
	}
	clean := filepath.Clean(p)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func readFileLimited(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("file not found: %s", path)
		}
		return "", err
	}
	if len(data) > maxPluginFileBytes {
		return "", fmt.Errorf("file too large (>%d bytes)", maxPluginFileBytes)
	}
	return string(data), nil
}

// resolveRequire applies Node-style extension resolution: if path does
// not exist and has no extension, the ".js" candidate is returned so
// require("./lib/foo") locates lib/foo.js. The path is returned
// unchanged when it already exists or already has an extension.
func resolveRequire(path string) string {
	if _, err := os.Stat(path); err == nil {
		return path
	}
	if filepath.Ext(path) == "" {
		return path + ".js"
	}
	return path
}

func readAll(path string, max int) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, err
	}
	if len(data) > max {
		return nil, fmt.Errorf("file too large (>%d bytes)", max)
	}
	return data, nil
}
