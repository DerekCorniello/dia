package cli

import (
	"os"

	"github.com/spf13/cobra"
)

func newCompletionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion script",
		Long: `Generate a shell completion script for dia.

  bash:  source <(dia completion bash)
  zsh:   source <(dia completion zsh)
  fish:  dia completion fish | source
  powershell: dia completion powershell | Out-String | Invoke-Expression

To install permanently:
  bash:  dia completion bash > /etc/bash_completion.d/dia
  zsh:   dia completion zsh > "${fpath[1]}/_dia"
  fish:  dia completion fish > ~/.config/fish/completions/dia.fish`,
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			switch args[0] {
			case "bash":
				err = cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				err = cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				err = cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				err = cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}
			return err
		},
	}
}
