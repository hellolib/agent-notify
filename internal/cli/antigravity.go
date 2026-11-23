package cli

import (
	"github.com/hellolib/agent-notify/internal/common"
	"github.com/spf13/cobra"
)

func newAntigravityCmd(streams Streams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "antigravity",
		Short: "Manage Antigravity hook integration",
	}
	cmd.AddCommand(
		newAntigravityPrintHooksCmd(streams),
		newAntigravityInstallHooksCmd(),
		newAntigravityUninstallHooksCmd(),
	)
	return cmd
}

func newAntigravityPrintHooksCmd(streams Streams) *cobra.Command {
	var binaryPath string

	cmd := &cobra.Command{
		Use:   "print-hooks",
		Short: "Print Antigravity hook settings JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPrintAntigravityHooks(streams, firstNonEmpty(binaryPath))
		},
	}
	cmd.Flags().StringVar(&binaryPath, "binary", common.ResolveBinaryPath(""), "agent-notify binary path")
	return cmd
}

func newAntigravityInstallHooksCmd() *cobra.Command {
	var binaryPath string
	var scope string

	cmd := &cobra.Command{
		Use:   "install-hooks",
		Short: "Install Antigravity hook settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallAntigravityHooks(scope, firstNonEmpty(binaryPath))
		},
	}
	cmd.Flags().StringVar(&binaryPath, "binary", common.ResolveBinaryPath(""), "agent-notify binary path")
	cmd.Flags().StringVar(&scope, "scope", "user", "install scope")
	return cmd
}

func newAntigravityUninstallHooksCmd() *cobra.Command {
	var scope string

	cmd := &cobra.Command{
		Use:   "uninstall-hooks",
		Short: "Remove Antigravity hook settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUninstallAntigravityHooks(scope)
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "user", "install scope")
	return cmd
}
