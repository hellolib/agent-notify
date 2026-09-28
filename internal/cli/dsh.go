package cli

import (
	"encoding/json"

	"github.com/hellolib/agent-notify/internal/agentintegrations"
	"github.com/hellolib/agent-notify/internal/dshhooks"
	"github.com/spf13/cobra"
)

func newDshCmd(streams Streams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dsh",
		Short: "Manage DeepSeek Harness plugin integration",
	}
	cmd.AddCommand(newDshPrintHooksCmd(streams), newDshInstallHooksCmd(), newDshUninstallHooksCmd())
	return cmd
}

// newDshPrintHooksCmd 输出安装计划。
//
// 与其它 agent 不同，DSH 没有「把 hook 条目写进某个 JSON」这回事——安装是一个
// 包管理动作。因此这里不伪造一份 settings JSON，而是如实输出落点、将执行的命令
// 与当前是否已注册。
func newDshPrintHooksCmd(streams Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "print-hooks",
		Short: "Print the DeepSeek Harness install plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := json.MarshalIndent(dshhooks.BuildInstallPlan(), "", "  ")
			if err != nil {
				return err
			}
			_, err = streams.Stdout.Write(append(data, '\n'))
			return err
		},
	}
}

// scopeFlagHelp 是 DSH 两个安装子命令共用的 -scope 说明。
// DSH 的插件装在 profile 下，没有项目级落点，因此只接受 user。
const scopeFlagHelp = "install scope; DSH supports only user"

func newDshInstallHooksCmd() *cobra.Command {
	var scope string
	cmd := &cobra.Command{
		Use:   "install-hooks",
		Short: "Install the agent-notify plugin into a DeepSeek Harness profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			integration := agentintegrations.NewDshIntegration()
			path, err := integration.SettingsPath(scope)
			if err != nil {
				return err
			}
			return integration.Install(path, "")
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "user", scopeFlagHelp)
	return cmd
}

func newDshUninstallHooksCmd() *cobra.Command {
	var scope string
	cmd := &cobra.Command{
		Use:   "uninstall-hooks",
		Short: "Remove the agent-notify plugin from a DeepSeek Harness profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			integration := agentintegrations.NewDshIntegration()
			path, err := integration.SettingsPath(scope)
			if err != nil {
				return err
			}
			return integration.Uninstall(path)
		},
	}
	cmd.Flags().StringVar(&scope, "scope", "user", scopeFlagHelp)
	return cmd
}
