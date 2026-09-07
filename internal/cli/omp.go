package cli

import (
	"encoding/json"

	"github.com/hellolib/agent-notify/internal/agentintegrations"
	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/omphooks"
	"github.com/spf13/cobra"
)

func newOmpCmd(streams Streams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "omp",
		Short: "Manage OMP (oh-my-pi) extension integration",
	}
	cmd.AddCommand(newOmpPrintHooksCmd(streams), newOmpInstallHooksCmd())
	return cmd
}

func newOmpPrintHooksCmd(streams Streams) *cobra.Command {
	var scope string
	cmd := &cobra.Command{
		Use:   "print-hooks",
		Short: "Print OMP extension settings",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := agentintegrations.NewOmpIntegration().SettingsPath(scope)
			if err != nil {
				return err
			}
			data, err := json.MarshalIndent(omphooks.BuildPluginSettings(path), "", "  ")
			if err != nil {
				return err
			}
			_, err = streams.Stdout.Write(append(data, '\n'))
			return err
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "user", "install scope")
	return cmd
}

func newOmpInstallHooksCmd() *cobra.Command {
	var binaryPath string
	var scope string
	cmd := &cobra.Command{
		Use:   "install-hooks",
		Short: "Install OMP extension",
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := agentintegrations.NewOmpIntegration().SettingsPath(scope)
			if err != nil {
				return err
			}
			return agentintegrations.NewOmpIntegration().Install(path, firstNonEmpty(binaryPath))
		},
	}
	cmd.Flags().StringVar(&binaryPath, "binary", common.ResolveBinaryPath(""), "agent-notify binary path")
	cmd.Flags().StringVar(&scope, "scope", "user", "install scope")
	return cmd
}
