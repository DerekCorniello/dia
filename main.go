package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/DerekCorniello/dia/internal/cli"
	"github.com/DerekCorniello/dia/internal/wailsapp"
)

//go:embed all:frontend/dist
var assets embed.FS

// Note: wails binds *wailsapp.App directly. The generator routes
// the JS bindings under wailsjs/go/wailsapp/App, not
// wailsjs/go/main/App, because the type's Go package is wailsapp.
// A main.App facade that delegated to wailsapp.App was tried first
// but the generator still put the bindings in wailsapp once the
// methods returned wailsapp.* types; binding the implementation
// type directly is the simpler choice.

func main() {
	if pluginID, ok := parsePluginWindowFlag(os.Args[1:]); ok {
		workspaceName := parseWorkspaceFlag(os.Args[1:])
		if err := wailsapp.RunPluginWindow(pluginID, workspaceName); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 {
		os.Exit(cli.Run(os.Args[1:]))
	}

	app := wailsapp.New()
	err := wails.Run(&options.App{
		Title:  "dia",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        func(ctx context.Context) { app.Startup(ctx) },
		Bind: []any{
			app,
		},
	})
	if err != nil {
		println("Error:", err.Error())
		os.Exit(1)
	}
}

// parsePluginWindowFlag scans args for "--plugin-window=<id>" or
// the split "--plugin-window" "<id>". The flag is consumed before
// cli.Run so the subcommand parser does not see it. The spawn is
// internal to dia (set by OpenPluginWindow) so users rarely pass
// it themselves.
func parsePluginWindowFlag(args []string) (string, bool) {
	for i, a := range args {
		if after, ok := strings.CutPrefix(a, "--plugin-window="); ok {
			return after, true
		}
		if a == "--plugin-window" && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}

// parseWorkspaceFlag scans args for "--workspace=<name>" or the split
// "--workspace" "<name>". Returns empty string if not found.
func parseWorkspaceFlag(args []string) string {
	for i, a := range args {
		if after, ok := strings.CutPrefix(a, "--workspace="); ok {
			return after
		}
		if a == "--workspace" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
