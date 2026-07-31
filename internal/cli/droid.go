package cli

import (
	"github.com/hellolib/agent-notify/internal/common"
	"github.com/spf13/cobra"
)

func newDroidCmd(streams Streams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "droid",
		Short: "Manage Droid hook integration",
	}
	cmd.AddCommand(newDroidPrintHooksCmd(streams), newDroidInstallHooksCmd())
	return cmd
}

func newDroidPrintHooksCmd(streams Streams) *cobra.Command {
	var binaryPath string

	cmd := &cobra.Command{
		Use:   "print-hooks",
		Short: "Print Droid hook settings JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrintDroidHooks(streams, firstNonEmpty(binaryPath))
		},
	}
	cmd.Flags().StringVar(&binaryPath, "binary", common.ResolveBinaryPath(""), "agent-notify binary path")
	return cmd
}

func newDroidInstallHooksCmd() *cobra.Command {
	var binaryPath string
	var scope string

	cmd := &cobra.Command{
		Use:   "install-hooks",
		Short: "Install Droid hook settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallDroidHooks(scope, firstNonEmpty(binaryPath))
		},
	}
	cmd.Flags().StringVar(&binaryPath, "binary", common.ResolveBinaryPath(""), "agent-notify binary path")
	cmd.Flags().StringVar(&scope, "scope", "user", "install scope")
	return cmd
}
