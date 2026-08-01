package cli

import (
	"context"

	"github.com/hellolib/agent-notify/internal/linuxfocus"
	"github.com/spf13/cobra"
)

func newLinuxNotifyWaitCmd(ctx context.Context) *cobra.Command {
	var req linuxfocus.Request
	cmd := &cobra.Command{
		Use:    "linux-notify-wait",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return linuxfocus.WaitNotifyAndFocus(ctx, req)
		},
	}
	cmd.Flags().StringVar(&req.Title, "title", "", "notification title")
	cmd.Flags().StringVar(&req.Body, "body", "", "notification body")
	cmd.Flags().StringVar(&req.WindowID, "window", "", "target X11 window id")
	cmd.Flags().StringVar(&req.Icon, "icon", "", "notification icon file path (per-agent logo)")
	return cmd
}
