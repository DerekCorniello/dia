package main

import (
	"context"
	"embed"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/DerekCorniello/dia/internal/cli"
	"github.com/DerekCorniello/dia/internal/wailsapp"
)

//go:embed all:frontend/dist
var assets embed.FS

// App is the wails-bound application surface. wails requires this type
// to live in the main package; it stays a thin facade and delegates to
// internal/wailsapp, which holds the real implementation.
type App struct {
	ctx   context.Context
	inner *wailsapp.App
}

func newApp() *App {
	return &App{inner: wailsapp.New()}
}

// Startup is called by the wails runtime after the window is created.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.inner.Startup(ctx)
}

func main() {
	if len(os.Args) > 1 {
		os.Exit(cli.Run(os.Args[1:]))
	}

	app := newApp()
	err := wails.Run(&options.App{
		Title:  "dia",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.Startup,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		println("Error:", err.Error())
		os.Exit(1)
	}
}
