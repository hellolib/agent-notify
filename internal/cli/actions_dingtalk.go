package cli

import (
	"context"
	"fmt"

	"github.com/hellolib/agent-notify/internal/app/tester"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/i18n"
)

func runTestDingTalk(ctx context.Context, streams Streams) error {
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	webhookURL := dingTalkURLFromConfig(cfg)
	if webhookURL == "" {
		return fmt.Errorf("%s", i18n.T("err.dingtalk_not_configured"))
	}

	svc := tester.NewService()
	result, err := svc.TestDingTalk(ctx, webhookURL)
	if err != nil {
		return err
	}
	fmt.Fprintln(streams.Stdout, "✅ "+result.Message)
	return nil
}

func runInitDingTalk(streams Streams, prompter Prompter) error {
	cfg, path, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	webhookURL, err := prompter.Input(i18n.T("prompt.dingtalk_webhook"), dingTalkURLFromConfig(cfg))
	if err != nil {
		return err
	}

	// Store URL on all agents; enable only for agents already configured.
	applyChannelToAgents(&cfg, func(agentEnabled bool, notify *config.AgentNotifyConfig) {
		notify.Channels.DingTalk.WebhookURL = webhookURL
		notify.Channels.DingTalk.Enabled = agentEnabled
	})

	if err := config.Save(path, cfg); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("err.save_failed"), err)
	}

	fmt.Fprintln(streams.Stdout, i18n.T("dingtalk.init_done"))
	fmt.Fprintf(streams.Stdout, i18n.T("msg.config_file")+"\n", path)
	return nil
}

func dingTalkURLFromConfig(cfg config.Config) string {
	if cfg.Notify.ClaudeCode.Channels.DingTalk.WebhookURL != "" {
		return cfg.Notify.ClaudeCode.Channels.DingTalk.WebhookURL
	}
	if cfg.Notify.Codex.Channels.DingTalk.WebhookURL != "" {
		return cfg.Notify.Codex.Channels.DingTalk.WebhookURL
	}
	if cfg.Notify.ZCode.Channels.DingTalk.WebhookURL != "" {
		return cfg.Notify.ZCode.Channels.DingTalk.WebhookURL
	}
	return cfg.Notify.Grok.Channels.DingTalk.WebhookURL
}
