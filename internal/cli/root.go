package cli

import (
	"context"
	"io"

	"github.com/spf13/cobra"
)

type Streams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

func NewRootCmd(ctx context.Context, streams Streams) *cobra.Command {
	root := &cobra.Command{
		Use:           "agent-notify",
		Short:         "Configure Claude Code notifications",
		SilenceUsage:  true,
		SilenceErrors: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	root.SetIn(streams.Stdin)
	root.SetOut(streams.Stdout)
	root.SetErr(streams.Stderr)

	// Add version flag
	root.Version = Version

	root.AddCommand(
		newInitCmd(streams),
		newClaudeCmd(streams),
		newZcodeCmd(streams),
		newGrokCmd(streams),
		newTestCmd(ctx, streams),
		newDoctorCmd(streams),
		newHandleClaudeHookCmd(ctx, streams),
		newHandleCodexHookCmd(ctx, streams),
		newHandleZcodeHookCmd(ctx, streams),
		newHandleGrokHookCmd(ctx, streams),
		newLinuxNotifyWaitCmd(ctx),
	)

	return root
}
