package cli

import (
	"context"

	"github.com/hellolib/agent-notify/internal/codexhooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/spf13/cobra"
)

func newHandleCodexNotifyCmd(ctx context.Context, streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:    "handle-codex-notify payload",
		Short:  "Internal Codex notify handler",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := config.DefaultPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			statePath, err := config.StatePath()
			if err != nil {
				return err
			}
			logPath, err := config.LogPath()
			if err != nil {
				return err
			}
			return codexhooks.HandleLegacyNotify(ctx, cfg, statePath, logPath, args[0])
		},
	}
}
