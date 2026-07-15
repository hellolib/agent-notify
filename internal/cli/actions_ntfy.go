package cli

import (
	"context"
	"fmt"

	"github.com/hellolib/agent-notify/internal/app/tester"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/i18n"
)

func runTestNtfy(ctx context.Context, streams Streams) error {
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	topicURL := ntfyURLFromConfig(cfg)
	if topicURL == "" {
		return fmt.Errorf("%s", i18n.T("err.ntfy_not_configured"))
	}

	svc := tester.NewService()
	result, err := svc.TestNtfy(ctx, topicURL)
	if err != nil {
		return err
	}
	fmt.Fprintln(streams.Stdout, "✅ "+result.Message)
	return nil
}

func runInitNtfy(streams Streams, prompter Prompter) error {
	cfg, path, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	topicURL, err := prompter.Input(i18n.T("prompt.ntfy_topic_url"), ntfyURLFromConfig(cfg))
	if err != nil {
		return err
	}

	// Store URL on all agents; enable only for agents already configured.
	applyChannelToAgents(&cfg, func(agentEnabled bool, notify *config.AgentNotifyConfig) {
		notify.Channels.Ntfy.TopicURL = topicURL
		notify.Channels.Ntfy.Enabled = agentEnabled
	})

	if err := config.Save(path, cfg); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("err.save_failed"), err)
	}

	fmt.Fprintln(streams.Stdout, i18n.T("ntfy.init_done"))
	fmt.Fprintf(streams.Stdout, i18n.T("msg.config_file")+"\n", path)
	return nil
}

func ntfyURLFromConfig(cfg config.Config) string {
	if cfg.Notify.ClaudeCode.Channels.Ntfy.TopicURL != "" {
		return cfg.Notify.ClaudeCode.Channels.Ntfy.TopicURL
	}
	if cfg.Notify.Codex.Channels.Ntfy.TopicURL != "" {
		return cfg.Notify.Codex.Channels.Ntfy.TopicURL
	}
	if cfg.Notify.ZCode.Channels.Ntfy.TopicURL != "" {
		return cfg.Notify.ZCode.Channels.Ntfy.TopicURL
	}
	return cfg.Notify.Grok.Channels.Ntfy.TopicURL
}
