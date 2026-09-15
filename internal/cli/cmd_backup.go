package cli

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/state"
)

func newBackupCmd() *cobra.Command {
	var out string
	var includeSeeds bool
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Archive workspaces and state for backup or migration",
		Long: "Write a tar.gz containing workspace YAMLs and state.json (+ .bak). " +
			"Managed browser seeds are excluded by default because they hold " +
			"credentials; use --include-seeds only for a same-machine move and " +
			"treat the archive as secret. Shut the daemon down first (`dia shutdown`) " +
			"so the archive captures a quiescent state.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if out == "" {
				return fmt.Errorf("--out is required")
			}
			stateDir, err := resolveStateDir(cmd)
			if err != nil {
				return err
			}
			f, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
			if err != nil {
				return fmt.Errorf("open %s: %w", out, err)
			}
			defer f.Close()
			gz := gzip.NewWriter(f)
			defer gz.Close()
			tw := tar.NewWriter(gz)
			defer tw.Close()
			addFile := func(name, path string) error {
				info, err := os.Stat(path)
				if err != nil {
					if os.IsNotExist(err) {
						return nil
					}
					return err
				}
				if !info.Mode().IsRegular() {
					return nil
				}
				hdr, err := tar.FileInfoHeader(info, "")
				if err != nil {
					return err
				}
				hdr.Name = name
				if err := tw.WriteHeader(hdr); err != nil {
					return err
				}
				src, err := os.Open(path)
				if err != nil {
					return err
				}
				defer src.Close()
				_, err = io.Copy(tw, src)
				return err
			}
			// Workspace YAMLs.
			globalDir := config.DefaultGlobalDir()
			err = filepath.WalkDir(globalDir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if d.IsDir() {
					return nil
				}
				if !strings.HasSuffix(d.Name(), ".yaml") && !strings.HasSuffix(d.Name(), ".yml") {
					return nil
				}
				rel, err := filepath.Rel(globalDir, path)
				if err != nil {
					return err
				}
				return addFile(filepath.Join("workspaces", rel), path)
			})
			if err != nil {
				return err
			}
			// State + backup.
			stateFile := filepath.Join(stateDir, state.StateFile)
			if err := addFile("state.json", stateFile); err != nil {
				return err
			}
			if err := addFile("state.json.bak", stateFile+".bak"); err != nil {
				return err
			}
			// Seeds only on explicit opt-in.
			if includeSeeds {
				seedsDir := filepath.Join(stateDir, "browser", "seeds")
				err = filepath.WalkDir(seedsDir, func(path string, d fs.DirEntry, err error) error {
					if err != nil || d.IsDir() {
						return nil
					}
					rel, err := filepath.Rel(stateDir, path)
					if err != nil {
						return err
					}
					return addFile(rel, path)
				})
				if err != nil {
					return err
				}
			}
			return newOutput(cmd).Printf("wrote %s\n", out)
		},
	}
	cmd.Flags().StringVar(&out, "out", "", "destination tar.gz path (required)")
	cmd.Flags().BoolVar(&includeSeeds, "include-seeds", false, "include managed browser seeds (credential material)")
	return cmd
}
