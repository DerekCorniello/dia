package cli

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/DerekCorniello/dia/internal/daemon"
	"github.com/DerekCorniello/dia/internal/state"
)

const maxDaemonLogBytes = 10 << 20

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
	path := filepath.Join(dir, state.LogFile)
	f, err := openDaemonLog(path)
	if err != nil {
		w = cmd.ErrOrStderr()
	} else {
		w = f
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func openDaemonLog(path string) (*os.File, error) {
	if info, err := os.Stat(path); err == nil && info.Size() >= maxDaemonLogBytes {
		rotated := path + ".1"
		if err := os.Remove(rotated); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("remove old daemon log: %w", err)
		}
		if err := os.Rename(path, rotated); err != nil {
			return nil, fmt.Errorf("rotate daemon log: %w", err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}
