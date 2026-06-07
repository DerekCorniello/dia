package plugins

import "context"

type HostAPI interface {
	ListWorkspaces(ctx context.Context) ([]any, error)
	GetWorkspace(ctx context.Context, name string) (any, error)
	StartWorkspace(ctx context.Context, name string) (any, error)
	ListInstances(ctx context.Context) ([]any, error)
	StopInstance(ctx context.Context, id string) error
	StopAll(ctx context.Context) (int, error)
	Doctor(ctx context.Context) ([]any, error)
	Paths(ctx context.Context) (any, error)
	GetTheme(ctx context.Context) (string, error)
	SetTheme(ctx context.Context, name string) error
	ListCustomThemes(ctx context.Context) ([]any, error)
	SetCustomTheme(ctx context.Context, info any) error
	DeleteCustomTheme(ctx context.Context, name string) error
	NewWorkspace(ctx context.Context, name string) (string, error)
}
