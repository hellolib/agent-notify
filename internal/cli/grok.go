package cli

import (
	"github.com/hellolib/agent-notify/internal/common"
	"github.com/spf13/cobra"
)

func newGrokCmd(streams Streams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "grok",
		Short: "Manage Grok hook integration",
	}
	cmd.AddCommand(newGrokPrintHooksCmd(streams), newGrokInstallHooksCmd())
	return cmd
}

func newGrokPrintHooksCmd(streams Streams) *cobra.Command {
	var binaryPath string

	cmd := &cobra.Command{
		Use:   "print-hooks",
		Short: "Print Grok hook settings JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrintGrokHooks(streams, firstNonEmpty(binaryPath))
		},
	}
	cmd.Flags().StringVar(&binaryPath, "binary", common.ResolveBinaryPath(""), "agent-notify binary path")
	return cmd
}

func newGrokInstallHooksCmd() *cobra.Command {
	var binaryPath string
	var scope string

	cmd := &cobra.Command{
		Use:   "install-hooks",
		Short: "Install Grok hook settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallGrokHooks(scope, firstNonEmpty(binaryPath))
		},
	}
	cmd.Flags().StringVar(&binaryPath, "binary", common.ResolveBinaryPath(""), "agent-notify binary path")
	cmd.Flags().StringVar(&scope, "scope", "user", "install scope")
	return cmd
}
