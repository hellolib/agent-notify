package cli

import (
	"context"
	"fmt"

	"github.com/hellolib/agent-notify/internal/app/tester"
	"github.com/spf13/cobra"
)

func newTestCmd(ctx context.Context, streams Streams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Send test notifications",
	}
	cmd.AddCommand(
		newTestFeishuCmd(ctx, streams),
		newTestSystemCmd(ctx, streams),
		newTestWechatCmd(ctx, streams),
		newTestBarkCmd(ctx, streams),
		newTestDingTalkCmd(ctx, streams),
		newTestNtfyCmd(ctx, streams),
	)
	return cmd
}

func newTestWechatCmd(ctx context.Context, streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "wechat",
		Short: "Send a WeChat webhook test notification",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTestWechat(ctx, streams)
		},
	}
}

func newTestFeishuCmd(ctx context.Context, streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "feishu",
		Short: "Send a Feishu test notification",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTestFeishu(ctx, streams)
		},
	}
}

func newTestSystemCmd(ctx context.Context, streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "system",
		Short: "Send a system test notification",
		RunE: func(cmd *cobra.Command, args []string) error {
			svc := tester.NewService()
			result, err := svc.TestSystem(ctx)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(streams.Stdout, result.Message)
			return err
		},
	}
}

func newTestBarkCmd(ctx context.Context, streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "bark",
		Short: "Send a Bark test notification",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTestBark(ctx, streams)
		},
	}
}

func newTestDingTalkCmd(ctx context.Context, streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "dingtalk",
		Short: "Send a DingTalk test notification",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTestDingTalk(ctx, streams)
		},
	}
}

func newTestNtfyCmd(ctx context.Context, streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "ntfy",
		Short: "Send a Ntfy test notification",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTestNtfy(ctx, streams)
		},
	}
}
