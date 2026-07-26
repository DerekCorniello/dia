package registry

import (
	"fmt"
	"strings"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/platform"
)

func buildLaunch(app config.App, cmd string, extraArgs ...string) platform.LaunchOpts {
	args := make([]string, 0, len(app.Args)+len(extraArgs))
	args = append(args, extraArgs...)
	args = append(args, app.Args...)
	return platform.LaunchOpts{
		Cmd:  cmd,
		Args: args,
		Cwd:  app.Cwd,
		Env:  envMapToSlice(app.Env),
	}
}

func envMapToSlice(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}

func resolveLocal(app config.App) (Action, error) {
	if strings.TrimSpace(app.Cmd) == "" {
		return Action{}, fmt.Errorf("type %q: cmd is required", appOrType(app, "local"))
	}
	program, prefixArgs, err := SplitProgram(app.Cmd)
	if err != nil {
		return Action{}, fmt.Errorf("type %q: %w", appOrType(app, "local"), err)
	}
	args := make([]string, 0, len(prefixArgs)+len(app.Args))
	args = append(args, prefixArgs...)
	args = append(args, app.Args...)
	opts := platform.LaunchOpts{
		Cmd:  program,
		Args: args,
		Cwd:  app.Cwd,
		Env:  envMapToSlice(app.Env),
	}
	return Action{Kind: ActionLaunch, Launch: &opts}, nil
}

func resolveOpen(app config.App) (Action, error) {
	if app.Url == "" {
		return Action{}, fmt.Errorf("type \"open\": url is required")
	}
	return Action{Kind: ActionOpenURL, URL: app.Url}, nil
}

func resolveBrowser(app config.App) (Action, error) {
	urls := collectURLs(app)
	if len(urls) == 0 {
		return Action{}, fmt.Errorf("type \"browser\": url is required")
	}
	if app.Browser == "" {
		if len(urls) > 1 {
			return Action{}, fmt.Errorf("type \"browser\": urls requires \"browser\" to be set; the OS default handler can only open one url at a time")
		}
		return Action{Kind: ActionOpenURL, URL: urls[0]}, nil
	}
	cmd, args := browserLaunch(app.Browser, urls, app.NewWindow)
	opts := buildLaunch(app, cmd, args...)
	return Action{Kind: ActionLaunch, Launch: &opts}, nil
}

// collectURLs merges an app's singular Url and its Urls list, in that
// order, dropping any blanks that validation didn't already catch (e.g.
// when Resolve is called directly in tests, bypassing Validate).
func collectURLs(app config.App) []string {
	urls := make([]string, 0, len(app.Urls)+1)
	if app.Url != "" {
		urls = append(urls, app.Url)
	}
	for _, u := range app.Urls {
		if u != "" {
			urls = append(urls, u)
		}
	}
	return urls
}

func resolveGH(app config.App) (Action, error) {
	if strings.TrimSpace(app.Cmd) == "" {
		return Action{}, fmt.Errorf("type \"gh\": cmd (the gh subcommand) is required")
	}
	opts := buildLaunch(app, "gh", app.Cmd)
	return Action{Kind: ActionLaunch, Launch: &opts}, nil
}

func resolveGHSugar(subcommand string) func(app config.App) (Action, error) {
	return func(app config.App) (Action, error) {
		opts := buildLaunch(app, "gh", subcommand)
		return Action{Kind: ActionLaunch, Launch: &opts}, nil
	}
}

func resolveGHRepoClone(app config.App) (Action, error) {
	if app.Url == "" {
		return Action{}, fmt.Errorf("type \"gh:repo-clone\": url is required")
	}
	opts := buildLaunch(app, "gh", "repo", "clone")
	opts.Args = append(opts.Args, app.Url)
	if app.Cwd != "" {
		opts.Args = append(opts.Args, app.Cwd)
	}
	return Action{Kind: ActionLaunch, Launch: &opts}, nil
}

func appOrType(app config.App, fallback string) string {
	if app.Type != "" {
		return app.Type
	}
	return fallback
}
