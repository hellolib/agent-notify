package cli

import (
	"context"

	"github.com/hellolib/agent-notify/internal/antigravityhooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/spf13/cobra"
)

func newHandleAntigravityHookCmd(ctx context.Context, streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:    "handle-antigravity-hook",
		Short:  "Internal Antigravity hook handler",
		Hidden: true,
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
			return antigravityhooks.Handle(ctx, cfg, statePath, logPath, streams.Stdin, streams.Stdout)
		},
	}
}
