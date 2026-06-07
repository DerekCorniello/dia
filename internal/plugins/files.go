// Package plugins: file IO helper used by both runtime and bridge.
package plugins

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

const maxPluginFileBytes = 1 << 20

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
