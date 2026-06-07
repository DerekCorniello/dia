package plugins

import (
	"context"
	"fmt"
	"github.com/dop251/goja"
	"path/filepath"
	"strings"
)

type Bridge struct {
	rt        *goja.Runtime
	granted   map[string]struct{}
	host      HostAPI
	pluginDir string
	config    map[string]any
}

func NewBridge(rt *goja.Runtime, requested, grants []string, host HostAPI, cfg map[string]any) *Bridge {
	m := make(map[string]struct{}, len(grants))
	for _, g := range grants {
		m[g] = struct{}{}
	}
	return &Bridge{rt: rt, granted: m, host: host, config: cfg}
}
func (b *Bridge) require(need string) error {
	if _, ok := b.granted[need]; !ok {
		return fmt.Errorf("capability %q not granted", need)
	}
	return nil
}

func (b *Bridge) has(c string) bool {
	_, ok := b.granted[c]
	return ok
}
func (b *Bridge) DiaObject() map[string]any {
	return map[string]any{
		"listWorkspaces":    b.listWorkspaces,
		"getWorkspace":      b.getWorkspace,
		"startWorkspace":    b.startWorkspace,
		"listInstances":     b.listInstances,
		"stopInstance":      b.stopInstance,
		"stopAll":           b.stopAll,
		"doctor":            b.doctor,
		"paths":             b.paths,
		"getTheme":          b.getTheme,
		"setTheme":          b.setTheme,
		"listCustomThemes":  b.listCustomThemes,
		"setCustomTheme":    b.setCustomTheme,
		"deleteCustomTheme": b.deleteCustomTheme,
		"newWorkspace":      b.newWorkspace,
		"pluginDir":         b.getPluginDir,
		"capabilities":      b.capabilities,
		"getConfig":         b.getConfig,
	}
}
func (b *Bridge) getConfig() map[string]any {
	return b.config
}
func (b *Bridge) getPluginDir() string { return b.pluginDir }
func (b *Bridge) capabilities() []string {
	out := make([]string, 0, len(b.granted))
	for k := range b.granted {
		out = append(out, k)
	}
	return out
}
func (b *Bridge) listWorkspaces() ([]any, error) {
	if err := b.require(CapWorkspacesRead); err != nil {
		return nil, err
	}
	return b.host.ListWorkspaces(context.Background())
}
func (b *Bridge) getWorkspace(name string) (any, error) {
	if err := b.require(CapWorkspacesRead); err != nil {
		return nil, err
	}
	return b.host.GetWorkspace(context.Background(), name)
}
func (b *Bridge) startWorkspace(name string) (any, error) {
	if err := b.require(CapWorkspacesStart); err != nil {
		return nil, err
	}
	return b.host.StartWorkspace(context.Background(), name)
}
func (b *Bridge) listInstances() ([]any, error) {
	if err := b.require(CapInstancesRead); err != nil {
		return nil, err
	}
	return b.host.ListInstances(context.Background())
}
func (b *Bridge) stopInstance(id string) error {
	if err := b.require(CapInstancesStop); err != nil {
		return err
	}
	return b.host.StopInstance(context.Background(), id)
}
func (b *Bridge) stopAll() (int, error) {
	if err := b.require(CapInstancesStop); err != nil {
		return 0, err
	}
	return b.host.StopAll(context.Background())
}
func (b *Bridge) doctor() ([]any, error) {
	if err := b.require(CapDoctorRead); err != nil {
		return nil, err
	}
	return b.host.Doctor(context.Background())
}
func (b *Bridge) paths() (any, error) {
	if err := b.require(CapPathsRead); err != nil {
		return nil, err
	}
	return b.host.Paths(context.Background())
}
func (b *Bridge) getTheme() (string, error) {
	if err := b.require(CapThemesRead); err != nil {
		return "", err
	}
	return b.host.GetTheme(context.Background())
}
func (b *Bridge) setTheme(name string) error {
	if err := b.require(CapThemesWrite); err != nil {
		return err
	}
	return b.host.SetTheme(context.Background(), name)
}
func (b *Bridge) listCustomThemes() ([]any, error) {
	if err := b.require(CapThemesRead); err != nil {
		return nil, err
	}
	return b.host.ListCustomThemes(context.Background())
}
func (b *Bridge) setCustomTheme(info any) error {
	if err := b.require(CapThemesWrite); err != nil {
		return err
	}
	return b.host.SetCustomTheme(context.Background(), info)
}
func (b *Bridge) deleteCustomTheme(name string) error {
	if err := b.require(CapThemesWrite); err != nil {
		return err
	}
	return b.host.DeleteCustomTheme(context.Background(), name)
}
func (b *Bridge) newWorkspace(name string) (string, error) {
	if err := b.require(CapWorkspacesNew); err != nil {
		return "", err
	}
	return b.host.NewWorkspace(context.Background(), name)
}
func (b *Bridge) NewRequire(pluginDir string) func(string) (goja.Value, error) {
	b.pluginDir = pluginDir
	return func(spec string) (goja.Value, error) {
		clean := filepath.Clean(spec)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
			return nil, fmt.Errorf("require %q: must be a relative path inside the plugin", spec)
		}
		full := filepath.Join(pluginDir, clean)
		data, err := readAll(full, maxPluginFileBytes)
		if err != nil {
			return nil, err
		}
		program, err := goja.Compile(full, string(data), true)
		if err != nil {
			return nil, fmt.Errorf("compile %s: %w", full, err)
		}
		v, err := b.rt.RunProgram(program)
		if err != nil {
			return nil, fmt.Errorf("run %s: %w", full, err)
		}
		if obj, ok := v.(*goja.Object); ok {
			if exp := obj.Get("module"); exp != nil && !goja.IsUndefined(exp) {
				return exp, nil
			}
			if exp := obj.Get("exports"); exp != nil && !goja.IsUndefined(exp) {
				return exp, nil
			}
		}
		return v, nil
	}
}
