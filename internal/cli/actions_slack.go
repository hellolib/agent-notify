package cli

import (
	"context"
	"fmt"

	"github.com/hellolib/agent-notify/internal/app/tester"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/i18n"
)

func runTestSlack(ctx context.Context, streams Streams) error {
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	webhookURL := slackURLFromConfig(cfg)
	if webhookURL == "" {
		return fmt.Errorf("%s", i18n.T("err.slack_not_configured"))
	}

	svc := tester.NewService()
	result, err := svc.TestSlack(ctx, webhookURL)
	if err != nil {
		return err
	}
	fmt.Fprintln(streams.Stdout, "✅ "+result.Message)
	return nil
}

func runInitSlack(streams Streams, prompter Prompter) error {
	cfg, path, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	webhookURL, err := prompter.Input(i18n.T("prompt.slack_webhook"), slackURLFromConfig(cfg))
	if err != nil {
		return err
	}

	// Store URL on all agents; enable only for agents already configured.
	applyChannelToAgents(&cfg, func(agentEnabled bool, notify *config.AgentNotifyConfig) {
		notify.Channels.Slack.WebhookURL = webhookURL
		notify.Channels.Slack.Enabled = agentEnabled
	})

	if err := config.Save(path, cfg); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("err.save_failed"), err)
	}

	fmt.Fprintln(streams.Stdout, i18n.T("slack.init_done"))
	fmt.Fprintf(streams.Stdout, i18n.T("msg.config_file")+"\n", path)
	return nil
}

func slackURLFromConfig(cfg config.Config) string {
	if cfg.Notify.ClaudeCode.Channels.Slack.WebhookURL != "" {
		return cfg.Notify.ClaudeCode.Channels.Slack.WebhookURL
	}
	if cfg.Notify.Codex.Channels.Slack.WebhookURL != "" {
		return cfg.Notify.Codex.Channels.Slack.WebhookURL
	}
	if cfg.Notify.ZCode.Channels.Slack.WebhookURL != "" {
		return cfg.Notify.ZCode.Channels.Slack.WebhookURL
	}
	return cfg.Notify.Grok.Channels.Slack.WebhookURL
}
