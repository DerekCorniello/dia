// Package cli implements the dia command-line interface. The
// entry point is Run, which main.go calls when os.Args has at least
// one element. Run constructs a cobra command tree, sets up the
// runtime, state store, platform, and registry, and dispatches to
// the requested subcommand.
package cli

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/platform"
	"github.com/DerekCorniello/dia/internal/registry"
	"github.com/DerekCorniello/dia/internal/runtime"
	"github.com/DerekCorniello/dia/internal/state"
	"github.com/DerekCorniello/dia/internal/version"
)

// Exit codes. The CLI uses these to communicate outcome to scripts
// and shells; main.go passes them through to os.Exit.
const (
	ExitOK            = 0
	ExitGeneral       = 1
	ExitUsage         = 2
	ExitNotFound      = 3
	ExitAlreadyExists = 4
)

// Run executes the CLI with the given args (typically os.Args[1:]).
// Returns the process exit code.
func Run(args []string) int {
	cmd := newRootCmd()
	cmd.SetArgs(args)
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(cmd.ErrOrStderr(), "Error:", err)
		return exitCodeFor(err)
	}
	return ExitOK
}

// newRootCmd builds the cobra command tree. The root command prints
// help by default and accepts --version/--json/--state-dir.
func newRootCmd() *cobra.Command {
	var (
		showVersion bool
		jsonOut     bool
		stateDir    string
	)
	cmd := &cobra.Command{
		Use:           "dia",
		Short:         "dia: a cross-platform desktop launcher for dev workspaces",
		Long:          "dia defines, starts, and stops dev workspaces from YAML configs. Run `dia` with no arguments to open the desktop launcher.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				fmt.Fprintf(cmd.OutOrStdout(), "dia version %s\n", version.Version)
				return nil
			}
			return cmd.Help()
		},
	}
	cmd.PersistentFlags().BoolVarP(&showVersion, "version", "V", false, "print version and exit")
	cmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "output machine-readable JSON")
	cmd.PersistentFlags().StringVar(&stateDir, "state-dir", "", "override XDG state dir (default: $XDG_STATE_HOME or ~/.local/state)")

	cmd.AddCommand(
		newStartCmd(),
		newStopCmd(),
		newStatusCmd(),
		newListCmd(),
		newNewCmd(),
		newEditCmd(),
		newOpenCmd(),
		newReconcileCmd(),
		newDoctorCmd(),
		newPluginsCmd(),
	)
	return cmd
}

type flagKey struct{}

// flagValues reads the persistent flag values from the command's
// flag set. Cobra inherits persistent flags to subcommands, so
// flagsFromCmd works on any command in the tree.
type flagValues struct {
	JSON     bool
	StateDir string
}

func flagsFromCmd(cmd *cobra.Command) flagValues {
	json, _ := cmd.Flags().GetBool("json")
	stateDir, _ := cmd.Flags().GetString("state-dir")
	return flagValues{JSON: json, StateDir: stateDir}
}

// setup bundles the runtime dependencies constructed for a command
// invocation. It is created on demand by getSetup and discarded
// after the command returns.
type setup struct {
	PF       platform.Platform
	Store    *state.Store
	Runtime  *runtime.Runtime
	Plugins  *registry.PluginResolver
	Reg      *registry.Registry
	StateDir string
	Logger   *slog.Logger
}

// newSetup constructs the runtime, state store, platform, registry,
// and plugin resolver. The state directory is taken from override
// (typically the --state-dir flag); if empty, the XDG default is
// used. The state store is created (or opened) at stateDir/state.json.
func newSetup(override string, errOut io.Writer) (*setup, error) {
	dir := override
	if dir == "" {
		var err error
		dir, err = state.ResolveStateDir()
		if err != nil {
			return nil, fmt.Errorf("state dir: %w", err)
		}
	}
	st, err := state.OpenAt(filepath.Join(dir, state.StateFile))
	if err != nil {
		return nil, fmt.Errorf("open state: %w", err)
	}
	pf := platform.New()
	plugins := registry.NewPluginResolver()
	reg := registry.New()
	var log *slog.Logger
	if errOut == nil {
		log = slog.Default()
	} else {
		log = slog.New(slog.NewTextHandler(errOut, &slog.HandlerOptions{Level: slog.LevelWarn}))
	}
	rt := runtime.New(runtime.Options{
		Platform: pf,
		Store:    st,
		Registry: reg,
		Plugins:  plugins,
		Logger:   log,
	})
	return &setup{
		PF: pf, Store: st, Runtime: rt, Plugins: plugins, Reg: reg,
		StateDir: dir, Logger: log,
	}, nil
}

// resolveWorkspace finds a workspace by name across the global and
// project-local config locations. The returned Source.Path is the
// absolute path the workspace was loaded from.
func resolveWorkspace(name string) (*config.Workspace, config.Source, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, config.Source{}, fmt.Errorf("getwd: %w", err)
	}
	all, err := config.Discover(config.DiscoverOptions{
		GlobalDir: config.DefaultGlobalDir(),
		CWD:       cwd,
	})
	if err != nil {
		return nil, config.Source{}, fmt.Errorf("discover workspaces: %w", err)
	}
	for _, w := range all {
		if w.Workspace.Name == name {
			return w.Workspace, w, nil
		}
	}
	return nil, config.Source{}, &NotFoundError{What: "workspace " + name}
}

// NotFoundError indicates the requested entity (workspace, instance)
// does not exist.
type NotFoundError struct {
	What string
}

func (e *NotFoundError) Error() string { return e.What + " not found" }

// exitCodeFor maps an error to a process exit code.
func exitCodeFor(err error) int {
	if err == nil {
		return ExitOK
	}
	var nf *NotFoundError
	if errors.As(err, &nf) {
		return ExitNotFound
	}
	if isUsageError(err) {
		return ExitUsage
	}
	return ExitGeneral
}

func isUsageError(err error) bool {
	if err == nil {
		return false
	}
	// Cobra wraps flag/command problems as *cmd.FlagError or
	// *cmd.SyntaxError, but for portability we just match on
	// the string. The set is small and stable.
	s := err.Error()
	switch {
	case errors.Is(err, errInvalidArgs):
		return true
	case containsAny(s, "unknown command", "unknown flag", "unknown shorthand", "required flag", "invalid argument", "accepts ", "requires arg"):
		return true
	}
	return false
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if contains(s, sub) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	return len(sub) <= len(s) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// errInvalidArgs is a sentinel a command can return to ask cobra to
// print usage.
var errInvalidArgs = errors.New("invalid arguments")
