package cli

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/DerekCorniello/dia/internal/daemon"
	"github.com/DerekCorniello/dia/internal/state"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "serve",
		Short:  "Run the dia session daemon",
		Long:   "Run the daemon that owns workspace sessions. The client spawns this automatically on first use; running it by hand is for debugging.",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := daemon.NewServer(daemon.Options{
				StateDir: flagsFromCmd(cmd).StateDir,
				Logger:   serveLogger(cmd),
			})
			if err != nil {
				return err
			}
			return srv.Serve()
		},
	}
}

// newDialClient connects to the daemon for the command's state dir,
// spawning one if it is not running. Lifecycle verbs live on the
// daemon; this is the only way to reach the runtime's authority.
func newDialClient(cmd *cobra.Command) (*daemon.Client, error) {
	dir := flagsFromCmd(cmd).StateDir
	if dir == "" {
		var err error
		dir, err = state.ResolveStateDir()
		if err != nil {
			return nil, err
		}
	}
	return daemon.Ensure(daemon.EnsureOpts{StateDir: dir})
}

// serveLogger writes daemon logs to the state log file so a detached
// daemon is not tied to the terminal that spawned it. Falls back to
// stderr when the file cannot be opened.
func serveLogger(cmd *cobra.Command) *slog.Logger {
	dir := flagsFromCmd(cmd).StateDir
	if dir == "" {
		if d, err := state.ResolveStateDir(); err == nil {
			dir = d
		}
	}
	var w io.Writer
	f, err := os.OpenFile(filepath.Join(dir, state.LogFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		w = cmd.ErrOrStderr()
	} else {
		w = f
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo}))
}
