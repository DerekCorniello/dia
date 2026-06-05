package registry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ErrPluginNotFound is returned by PluginResolver.Resolve when no
// `dia-<name>` executable is found on PATH.
var ErrPluginNotFound = errors.New("plugin not found")

// PluginResolver looks up `dia-<name>` on PATH. Resolutions are
// cached for the lifetime of the resolver, which is the lifetime of
// the dia process in the v1 design.
type PluginResolver struct {
	mu    sync.RWMutex
	cache map[string]string

	// LookPath is the underlying PATH lookup. Replaced in tests
	// to point at a fixture dir without touching the real PATH.
	LookPath func(file string) (string, error)
	// Suffix is prepended to the plugin name to form the
	// executable name. Default: "dia-".
	Suffix string
}

// NewPluginResolver returns a resolver that uses the current process
// PATH and the default `dia-` prefix.
func NewPluginResolver() *PluginResolver {
	return &PluginResolver{
		cache:    map[string]string{},
		LookPath: defaultLookPath,
		Suffix:   "dia-",
	}
}

// NewPluginResolverAt returns a resolver that searches only the given
// directories, in order. Used by tests and by callers that want to
// restrict the plugin search to specific paths.
func NewPluginResolverAt(dirs []string) *PluginResolver {
	cleaned := make([]string, 0, len(dirs))
	for _, d := range dirs {
		if d != "" {
			cleaned = append(cleaned, d)
		}
	}
	r := &PluginResolver{
		cache:  map[string]string{},
		Suffix: "dia-",
	}
	r.LookPath = func(file string) (string, error) {
		for _, d := range cleaned {
			full := filepath.Join(d, file)
			if isExecutable(full) {
				return full, nil
			}
		}
		return "", ErrPluginNotFound
	}
	return r
}

func defaultLookPath(file string) (string, error) {
	return execLookPath(file)
}

// Resolve returns the absolute path of the `dia-<name>` executable,
// or ErrPluginNotFound if no such file exists on PATH. The result is
// cached.
func (p *PluginResolver) Resolve(name string) (string, error) {
	if name == "" {
		return "", errors.New("plugin: empty name")
	}
	if strings.ContainsRune(name, os.PathSeparator) || strings.ContainsRune(name, '/') {
		return "", fmt.Errorf("plugin: name %q contains path separator", name)
	}
	binary := p.Suffix + name

	p.mu.RLock()
	if path, ok := p.cache[binary]; ok {
		p.mu.RUnlock()
		return path, nil
	}
	p.mu.RUnlock()

	path, err := p.LookPath(binary)
	if err != nil {
		return "", fmt.Errorf("plugin %q: %w", name, ErrPluginNotFound)
	}
	p.mu.Lock()
	p.cache[binary] = path
	p.mu.Unlock()
	return path, nil
}

// Forget clears the cache. Useful for tests; not used in production.
func (p *PluginResolver) Forget() {
	p.mu.Lock()
	p.cache = map[string]string{}
	p.mu.Unlock()
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	mode := info.Mode()
	if mode&0o111 == 0 {
		return false
	}
	return true
}
