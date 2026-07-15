package cli

import (
	"context"
	"fmt"

	"github.com/hellolib/agent-notify/internal/app/tester"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/i18n"
)

func runTestBark(ctx context.Context, streams Streams) error {
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	webhookURL := barkURLFromConfig(cfg)
	if webhookURL == "" {
		return fmt.Errorf("%s", i18n.T("err.bark_not_configured"))
	}

	svc := tester.NewService()
	result, err := svc.TestBark(ctx, webhookURL)
	if err != nil {
		return err
	}
	fmt.Fprintln(streams.Stdout, "✅ "+result.Message)
	return nil
}

func runInitBark(streams Streams, prompter Prompter) error {
	cfg, path, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	webhookURL, err := prompter.Input(i18n.T("prompt.bark_webhook"), barkURLFromConfig(cfg))
	if err != nil {
		return err
	}

	// Store URL on all agents; enable only for agents already configured.
	applyChannelToAgents(&cfg, func(agentEnabled bool, notify *config.AgentNotifyConfig) {
		notify.Channels.Bark.WebhookURL = webhookURL
		notify.Channels.Bark.Enabled = agentEnabled
	})

	if err := config.Save(path, cfg); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("err.save_failed"), err)
	}

	fmt.Fprintln(streams.Stdout, i18n.T("bark.init_done"))
	fmt.Fprintf(streams.Stdout, i18n.T("msg.config_file")+"\n", path)
	return nil
}

func barkURLFromConfig(cfg config.Config) string {
	if cfg.Notify.ClaudeCode.Channels.Bark.WebhookURL != "" {
		return cfg.Notify.ClaudeCode.Channels.Bark.WebhookURL
	}
	if cfg.Notify.Codex.Channels.Bark.WebhookURL != "" {
		return cfg.Notify.Codex.Channels.Bark.WebhookURL
	}
	if cfg.Notify.ZCode.Channels.Bark.WebhookURL != "" {
		return cfg.Notify.ZCode.Channels.Bark.WebhookURL
	}
	return cfg.Notify.Grok.Channels.Bark.WebhookURL
}
