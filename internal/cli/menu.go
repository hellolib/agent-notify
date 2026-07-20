package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/hellolib/agent-notify/internal/agentintegrations"
	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/feishucli"
	"github.com/hellolib/agent-notify/internal/i18n"
)

const banner = `
╔════════════════════════════════════════════════════════════════╗
║            █████╗  ██████╗ ███████╗███╗   ██╗████████╗         ║
║           ██╔══██╗██╔════╝ ██╔════╝████╗  ██║╚══██╔══╝         ║
║           ███████║██║  ███╗█████╗  ██╔██╗ ██║   ██║            ║
║           ██╔══██║██║   ██║██╔══╝  ██║╚██╗██║   ██║            ║
║           ██║  ██║╚██████╔╝███████╗██║ ╚████║   ██║            ║
║           ╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚═╝  ╚═══╝   ╚═╝            ║
║                        Agent Notify                            ║
║    Claude Code / Codex / ZCode / Grok Notification Tool        ║
╚════════════════════════════════════════════════════════════════╝
`

func runMenu(ctx context.Context, streams Streams) error {
	prompter, err := newPrompter(streams)
	if err != nil {
		return err
	}

	// 只在首次显示 banner
	renderBanner(streams)

	for {
		choice, err := prompter.Select("", []PromptOption{
			{Label: i18n.T("menu.agent_config"), Value: "init"},
			{Label: i18n.T("menu.channel_config"), Value: "channels"},
			{Label: i18n.T("menu.test"), Value: "test"},
			{Label: i18n.T("menu.doctor"), Value: "doctor"},
			{Label: i18n.T("menu.view_config"), Value: "view"},
			{Label: i18n.T("menu.clean_config"), Value: "clean"},
			{Label: i18n.T("menu.language"), Value: "language"},
			{Label: i18n.T("menu.quit"), Value: "quit"},
		}, "init")
		if err != nil {
			if errors.Is(err, ErrCancelled) {
				return nil // Ctrl+C 退出程序
			}
			return err
		}

		switch choice {
		case "init":
			if err := runInitFlow(ctx, streams, prompter, "", "", common.ResolveBinaryPath("")); err != nil {
				if errors.Is(err, ErrCancelled) {
					fmt.Fprintln(streams.Stdout) // 仅换行，不显示错误
				} else {
					fmt.Fprintf(streams.Stdout, "\n%s: %v\n\n", i18n.T("err.config_failed"), err)
				}
			} else {
				fmt.Fprint(streams.Stdout, "\n"+i18n.T("msg.config_done")+"\n\n")
			}
		case "channels":
			if err := runChannelsMenu(ctx, streams, prompter); err != nil {
				if !errors.Is(err, ErrCancelled) {
					fmt.Fprintf(streams.Stdout, "\n%s: %v\n\n", i18n.T("err.config_failed"), err)
				}
			}
		case "test":
			if err := runTestMenu(ctx, streams, prompter); err != nil {
				if !errors.Is(err, ErrCancelled) {
					fmt.Fprintf(streams.Stdout, "\n%s: %v\n\n", i18n.T("err.test_failed"), err)
				}
			}
		case "doctor":
			if err := runDoctor(streams); err != nil {
				fmt.Fprintf(streams.Stdout, "\n%s: %v\n\n", i18n.T("err.doctor_failed"), err)
			}
		case "view":
			if err := printCurrentNotifyConfig(streams); err != nil {
				fmt.Fprintf(streams.Stdout, "\n%s: %v\n\n", i18n.T("err.view_failed"), err)
			}
		case "clean":
			if err := runCleanConfig(streams, prompter); err != nil {
				if !errors.Is(err, ErrCancelled) {
					fmt.Fprintf(streams.Stdout, "\n%s: %v\n\n", i18n.T("err.clean_failed"), err)
				}
			}
		case "language":
			if err := runSelectLanguage(streams, prompter); err != nil {
				if !errors.Is(err, ErrCancelled) {
					fmt.Fprintf(streams.Stdout, "\n%s: %v\n\n", i18n.T("err.config_failed"), err)
				}
			}
		case "quit":
			return nil
		}
	}
}

func runTestMenu(ctx context.Context, streams Streams, prompter Prompter) error {
	choice, err := prompter.Select(i18n.T("test.title"), []PromptOption{
		{Label: i18n.T("test.system"), Value: "system"},
		{Label: i18n.T("test.feishu"), Value: "feishu"},
		{Label: i18n.T("test.wechat_personal"), Value: "wechat"},
		{Label: i18n.T("test.wechat"), Value: "wechat-work"},
		{Label: i18n.T("test.dingtalk"), Value: "dingtalk"},
		{Label: i18n.T("test.bark"), Value: "bark"},
		{Label: i18n.T("test.ntfy"), Value: "ntfy"},
		{Label: i18n.T("test.slack"), Value: "slack"},
		{Label: i18n.T("test.back"), Value: "back"},
	}, "system")
	if err != nil {
		return err
	}

	switch choice {
	case "feishu":
		return runTestFeishu(ctx, streams)
	case "system":
		return runTestSystem(ctx, streams)
	case "wechat":
		return runTestWechat(ctx, streams)
	case "wechat-work":
		return runTestWechatWork(ctx, streams)
	case "dingtalk":
		return runTestDingTalk(ctx, streams)
	case "bark":
		return runTestBark(ctx, streams)
	case "ntfy":
		return runTestNtfy(ctx, streams)
	case "slack":
		return runTestSlack(ctx, streams)
	default:
		return nil
	}
}

func runChannelsMenu(ctx context.Context, streams Streams, prompter Prompter) error {
	for {
		choice, err := prompter.Select(i18n.T("channel.title"), []PromptOption{
			{Label: i18n.T("channel.feishu"), Value: "feishu-init"},
			{Label: i18n.T("channel.wechat_personal"), Value: "wechat-init"},
			{Label: i18n.T("channel.wechat"), Value: "wechatwork-init"},
			{Label: i18n.T("channel.dingtalk"), Value: "dingtalk-init"},
			{Label: i18n.T("channel.bark"), Value: "bark-init"},
			{Label: i18n.T("channel.ntfy"), Value: "ntfy-init"},
			{Label: i18n.T("channel.slack"), Value: "slack-init"},
			{Label: i18n.T("channel.back"), Value: "back"},
		}, "feishu-init")
		if err != nil {
			return err
		}

		switch choice {
		case "feishu-init":
			if _, err := feishucli.Reinitialize(ctx); err != nil {
				return err
			}
			fmt.Fprintln(streams.Stdout, i18n.T("msg.feishu_cli_done"))
		case "wechat-init":
			if err := runInitWechat(streams, prompter); err != nil {
				return err
			}
		case "wechatwork-init":
			if err := runInitWechatWork(streams, prompter); err != nil {
				return err
			}
		case "dingtalk-init":
			if err := runInitDingTalk(streams, prompter); err != nil {
				return err
			}
		case "bark-init":
			if err := runInitBark(streams, prompter); err != nil {
				return err
			}
		case "ntfy-init":
			if err := runInitNtfy(streams, prompter); err != nil {
				return err
			}
		case "slack-init":
			if err := runInitSlack(streams, prompter); err != nil {
				return err
			}
		case "back":
			return nil
		}
	}
}

func runCleanConfig(streams Streams, prompter Prompter) error {
	confirm, err := prompter.Confirm(i18n.T("clean.confirm"), false)
	if err != nil {
		return err
	}
	if !confirm {
		fmt.Fprintln(streams.Stdout, i18n.T("clean.cancelled"))
		return nil
	}

	// 清理 agent-notify 配置
	cfgPath, err := config.DefaultPath()
	if err != nil {
		return err
	}
	if err := os.Remove(cfgPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%s: %w", i18n.T("clean.delete_failed"), err)
	}

	// 清理状态文件
	statePath, err := config.StatePath()
	if err != nil {
		return err
	}
	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%s: %w", i18n.T("clean.delete_failed"), err)
	}

	// 清理日志文件
	logPath, err := config.LogPath()
	if err != nil {
		return err
	}
	if err := os.Remove(logPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("%s: %w", i18n.T("clean.delete_failed"), err)
	}

	// 清理 Claude / Codex / ZCode / Grok 中由本插件写入的 hook（保留用户挂载的其他 hook）
	for _, integ := range []agentintegrations.Integration{
		agentintegrations.NewClaudeIntegration(),
		agentintegrations.NewCodexIntegration(),
		agentintegrations.NewZcodeIntegration(),
		agentintegrations.NewGrokIntegration(),
	} {
		settingsPath, err := integ.SettingsPath("user")
		if err != nil {
			fmt.Fprintf(streams.Stdout, i18n.T("clean.skip_hooks"), integ.Name(), err)
			continue
		}
		if err := integ.Uninstall(settingsPath); err != nil {
			fmt.Fprintf(streams.Stdout, i18n.T("clean.hooks_failed"), integ.Name(), settingsPath, err)
			continue
		}
		fmt.Fprintf(streams.Stdout, i18n.T("clean.hooks_done"), integ.Name(), settingsPath)
	}

	// 保存一个干净的默认配置（所有通知都关闭）
	defaultCfg := config.Default()
	// Clear ClaudeCode channel toggles and events
	defaultCfg.Notify.ClaudeCode.Channels.Feishu.Enabled = false
	defaultCfg.Notify.ClaudeCode.Channels.System.Enabled = false
	defaultCfg.Notify.ClaudeCode.Channels.Wechat.Enabled = false
	defaultCfg.Notify.ClaudeCode.Channels.Wechat.WebhookURL = ""
	defaultCfg.Notify.ClaudeCode.Channels.WechatWork.Enabled = false
	defaultCfg.Notify.ClaudeCode.Channels.WechatWork.WebhookURL = ""
	defaultCfg.Notify.ClaudeCode.Channels.DingTalk.Enabled = false
	defaultCfg.Notify.ClaudeCode.Channels.DingTalk.WebhookURL = ""
	defaultCfg.Notify.ClaudeCode.Channels.Bark.Enabled = false
	defaultCfg.Notify.ClaudeCode.Channels.Bark.WebhookURL = ""
	defaultCfg.Notify.ClaudeCode.Channels.Ntfy.Enabled = false
	defaultCfg.Notify.ClaudeCode.Channels.Ntfy.TopicURL = ""
	defaultCfg.Notify.ClaudeCode.Channels.Slack.Enabled = false
	defaultCfg.Notify.ClaudeCode.Channels.Slack.WebhookURL = ""
	defaultCfg.Notify.ClaudeCode.Events = nil
	// Clear Codex channel toggles
	defaultCfg.Notify.Codex.Channels.Feishu.Enabled = false
	defaultCfg.Notify.Codex.Channels.System.Enabled = false
	defaultCfg.Notify.Codex.Channels.Wechat.Enabled = false
	defaultCfg.Notify.Codex.Channels.Wechat.WebhookURL = ""
	defaultCfg.Notify.Codex.Channels.WechatWork.Enabled = false
	defaultCfg.Notify.Codex.Channels.WechatWork.WebhookURL = ""
	defaultCfg.Notify.Codex.Channels.DingTalk.Enabled = false
	defaultCfg.Notify.Codex.Channels.DingTalk.WebhookURL = ""
	defaultCfg.Notify.Codex.Channels.Bark.Enabled = false
	defaultCfg.Notify.Codex.Channels.Bark.WebhookURL = ""
	defaultCfg.Notify.Codex.Channels.Ntfy.Enabled = false
	defaultCfg.Notify.Codex.Channels.Ntfy.TopicURL = ""
	defaultCfg.Notify.Codex.Channels.Slack.Enabled = false
	defaultCfg.Notify.Codex.Channels.Slack.WebhookURL = ""
	defaultCfg.Notify.Codex.Events = nil
	// Clear ZCode channel toggles
	defaultCfg.Notify.ZCode.Channels.Feishu.Enabled = false
	defaultCfg.Notify.ZCode.Channels.System.Enabled = false
	defaultCfg.Notify.ZCode.Channels.Wechat.Enabled = false
	defaultCfg.Notify.ZCode.Channels.Wechat.WebhookURL = ""
	defaultCfg.Notify.ZCode.Channels.WechatWork.Enabled = false
	defaultCfg.Notify.ZCode.Channels.WechatWork.WebhookURL = ""
	defaultCfg.Notify.ZCode.Channels.DingTalk.Enabled = false
	defaultCfg.Notify.ZCode.Channels.DingTalk.WebhookURL = ""
	defaultCfg.Notify.ZCode.Channels.Bark.Enabled = false
	defaultCfg.Notify.ZCode.Channels.Bark.WebhookURL = ""
	defaultCfg.Notify.ZCode.Channels.Ntfy.Enabled = false
	defaultCfg.Notify.ZCode.Channels.Ntfy.TopicURL = ""
	defaultCfg.Notify.ZCode.Channels.Slack.Enabled = false
	defaultCfg.Notify.ZCode.Channels.Slack.WebhookURL = ""
	defaultCfg.Notify.ZCode.Events = nil
	// Clear Grok channel toggles
	defaultCfg.Notify.Grok.Channels.Feishu.Enabled = false
	defaultCfg.Notify.Grok.Channels.System.Enabled = false
	defaultCfg.Notify.Grok.Channels.Wechat.Enabled = false
	defaultCfg.Notify.Grok.Channels.Wechat.WebhookURL = ""
	defaultCfg.Notify.Grok.Channels.WechatWork.Enabled = false
	defaultCfg.Notify.Grok.Channels.WechatWork.WebhookURL = ""
	defaultCfg.Notify.Grok.Channels.DingTalk.Enabled = false
	defaultCfg.Notify.Grok.Channels.DingTalk.WebhookURL = ""
	defaultCfg.Notify.Grok.Channels.Bark.Enabled = false
	defaultCfg.Notify.Grok.Channels.Bark.WebhookURL = ""
	defaultCfg.Notify.Grok.Channels.Ntfy.Enabled = false
	defaultCfg.Notify.Grok.Channels.Ntfy.TopicURL = ""
	defaultCfg.Notify.Grok.Channels.Slack.Enabled = false
	defaultCfg.Notify.Grok.Channels.Slack.WebhookURL = ""
	defaultCfg.Notify.Grok.Events = nil
	if err := config.Save(cfgPath, defaultCfg); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("clean.save_default_err"), err)
	}

	fmt.Fprintln(streams.Stdout, i18n.T("clean.done"))
	return nil
}

func renderBanner(streams Streams) {
	fmt.Fprint(streams.Stdout, banner)
	fmt.Fprintf(streams.Stdout, "  Version: %s  |  https://github.com/hellolib/agent-notify\n\n", Version)
}

func runSelectLanguage(streams Streams, prompter Prompter) error {
	defaultLang := "zh-CN"
	if i18n.IsEnglish() {
		defaultLang = "en-US"
	}

	choice, err := prompter.Select(i18n.T("menu.language"), []PromptOption{
		{Label: "中文", Value: "zh-CN"},
		{Label: "English", Value: "en-US"},
	}, defaultLang)
	if err != nil {
		return err
	}

	// Persist to config
	cfgPath, err := config.DefaultPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	cfg.Behavior.Locale = choice
	if err := config.Save(cfgPath, cfg); err != nil {
		return err
	}

	// Apply immediately
	i18n.Set(choice)
	return nil
}
