package daemon

import (
	"context"
	"errors"
)

// nullResolverHost is a plugins.HostAPI that returns defensively empty
// values for everything. The daemon's plugin manager is used only for
// its resolver side -- claiming app types and holding manifests so
// window plugins can be spawned -- and never runs panel goja runtimes,
// so no host capability is ever consulted here.
type nullResolverHost struct{}

var errDaemonHostUnhandled = errors.New("daemon resolver host: unhandled")

func (h *nullResolverHost) ListWorkspaces(context.Context) ([]any, error) {
	return nil, nil
}

func (h *nullResolverHost) GetWorkspace(context.Context, string) (any, error) {
	return nil, nil
}

func (h *nullResolverHost) StartWorkspace(context.Context, string) (any, error) {
	return nil, nil
}

func (h *nullResolverHost) ListInstances(context.Context) ([]any, error) {
	return nil, nil
}

func (h *nullResolverHost) StopInstance(context.Context, string) error {
	return errDaemonHostUnhandled
}

func (h *nullResolverHost) StopAll(context.Context) (int, error) {
	return 0, errDaemonHostUnhandled
}

func (h *nullResolverHost) Doctor(context.Context) ([]any, error) {
	return nil, nil
}

func (h *nullResolverHost) Paths(context.Context) (any, error) {
	return nil, nil
}

func (h *nullResolverHost) GetTheme(context.Context) (string, error) {
	return "", nil
}

func (h *nullResolverHost) SetTheme(context.Context, string) error {
	return errDaemonHostUnhandled
}

func (h *nullResolverHost) ListCustomThemes(context.Context) ([]any, error) {
	return nil, nil
}

func (h *nullResolverHost) SetCustomTheme(context.Context, any) error {
	return errDaemonHostUnhandled
}

func (h *nullResolverHost) DeleteCustomTheme(context.Context, string) error {
	return errDaemonHostUnhandled
}

func (h *nullResolverHost) NewWorkspace(context.Context, string) (string, error) {
	return "", errDaemonHostUnhandled
}

func (h *nullResolverHost) Exec(context.Context, string, []string) (string, error) {
	return "", errDaemonHostUnhandled
}

func (h *nullResolverHost) Fetch(context.Context, string, map[string]any) (any, error) {
	return nil, errDaemonHostUnhandled
}
