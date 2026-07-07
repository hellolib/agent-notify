package cli

import (
	"github.com/hellolib/agent-notify/internal/common"
	"github.com/spf13/cobra"
)

func newCodexCmd(streams Streams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "codex",
		Short: "Manage Codex hook integration",
	}
	cmd.AddCommand(newCodexPrintHooksCmd(streams), newCodexInstallHooksCmd())
	return cmd
}

func newCodexPrintHooksCmd(streams Streams) *cobra.Command {
	var binaryPath string

	cmd := &cobra.Command{
		Use:   "print-hooks",
		Short: "Print Codex hook settings JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrintCodexHooks(streams, firstNonEmpty(binaryPath))
		},
	}
	cmd.Flags().StringVar(&binaryPath, "binary", common.ResolveBinaryPath(""), "agent-notify binary path")
	return cmd
}

func newCodexInstallHooksCmd() *cobra.Command {
	var binaryPath string
	var scope string

	cmd := &cobra.Command{
		Use:   "install-hooks",
		Short: "Install Codex hook settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallCodexHooks(scope, firstNonEmpty(binaryPath))
		},
	}
	cmd.Flags().StringVar(&binaryPath, "binary", common.ResolveBinaryPath(""), "agent-notify binary path")
	cmd.Flags().StringVar(&scope, "scope", "user", "install scope")
	return cmd
}
