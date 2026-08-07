package cli

import (
	"github.com/hellolib/agent-notify/internal/common"
	"github.com/spf13/cobra"
)

func newOpenCodeCmd(streams Streams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "opencode",
		Short: "Manage OpenCode plugin integration",
	}
	cmd.AddCommand(newOpenCodePrintHooksCmd(streams), newOpenCodeInstallHooksCmd())
	return cmd
}

func newOpenCodePrintHooksCmd(streams Streams) *cobra.Command {
	var binaryPath string

	cmd := &cobra.Command{
		Use:   "print-hooks",
		Short: "Print OpenCode plugin config JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrintOpenCodeHooks(streams, firstNonEmpty(binaryPath))
		},
	}
	cmd.Flags().StringVar(&binaryPath, "binary", common.ResolveBinaryPath(""), "agent-notify binary path")
	return cmd
}

func newOpenCodeInstallHooksCmd() *cobra.Command {
	var binaryPath string
	var scope string

	cmd := &cobra.Command{
		Use:   "install-hooks",
		Short: "Install OpenCode plugin",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallOpenCodeHooks(scope, firstNonEmpty(binaryPath))
		},
	}
	cmd.Flags().StringVar(&binaryPath, "binary", common.ResolveBinaryPath(""), "agent-notify binary path")
	cmd.Flags().StringVar(&scope, "scope", "user", "install scope")
	return cmd
}
