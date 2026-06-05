package wailsapp

import "context"

// App is the Wails-bound application surface. Methods exported on App
// become available to the Svelte frontend as `wailsjs/go/wailsapp/App.*`.
type App struct {
	ctx context.Context
}

// New returns a new App with no context set; Startup fills it in.
func New() *App {
	return &App{}
}

// Startup is called by the Wails runtime after the window is created.
// The context is required for any runtime calls (events, dialogs, etc.).
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}
