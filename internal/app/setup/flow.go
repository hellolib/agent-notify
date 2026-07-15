package setup

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/i18n"
)

const (
	agentClaude     = "claude"
	agentCodex      = "codex"
	agentZcode      = "zcode"
	agentGrok       = "grok"
	channelSystem   = "system"
	channelFeishu   = "feishu"
	channelWechat   = "wechat"
	channelWXWork   = "wechat-work"
	channelDingTalk = "dingtalk"
	channelBark     = "bark"
	channelNtfy     = "ntfy"
	channelSlack    = "slack"
	installScopeUsr = "user"
	installScopePrj = "project"
)

func channelOptions() []PromptOption {
	return []PromptOption{
		{Label: i18n.T("channel.system"), Value: channelSystem},
		{Label: i18n.T("channel.feishu"), Value: channelFeishu},
		{Label: i18n.T("channel.wechat_personal"), Value: channelWechat},
		{Label: i18n.T("channel.wechat"), Value: channelWXWork},
		{Label: i18n.T("channel.dingtalk"), Value: channelDingTalk},
		{Label: i18n.T("channel.bark"), Value: channelBark},
		{Label: i18n.T("channel.ntfy"), Value: channelNtfy},
		{Label: i18n.T("channel.slack"), Value: channelSlack},
	}
}

type channelSelection struct {
	System     bool
	Feishu     bool
	Wechat     bool
	WechatWork bool
	DingTalk   bool
	Bark       bool
	Ntfy       bool
	Slack      bool
}

func (c channelSelection) hasAny() bool {
	return c.System || c.Feishu || c.Wechat || c.WechatWork || c.DingTalk || c.Bark || c.Ntfy || c.Slack
}

type configureAgentRequest struct {
	ctx        context.Context
	prompter   Prompter
	output     OutputWriter
	cfg        config.Config
	agent      string
	channels   channelSelection
	events     []string
	binaryPath string
}

type configuredAgent struct {
	cfg          config.Config
	settingsPath string
}

func (s *Service) selectAgent(prompter Prompter, cfg config.Config) (string, error) {
	agentOptions, defaultAgent := s.agentOptions(cfg)
	if len(agentOptions) == 0 {
		return "", errors.New("Claude Code, Codex, ZCode or Grok not detected; please install one first")
	}
	if defaultAgent == "" {
		defaultAgent = agentOptions[0].Value
	}
	return prompter.Select(i18n.T("setup.select_agent"), agentOptions, defaultAgent)
}

func (s *Service) agentOptions(cfg config.Config) ([]PromptOption, string) {
	var options []PromptOption
	var defaultAgent string
	if s.claudeIntegration.DetectInstalled() {
		options = append(options, PromptOption{Label: "Claude Code", Value: agentClaude})
		if cfg.Agent.ClaudeCode.Enabled {
			defaultAgent = agentClaude
		}
	}
	if s.codexIntegration.DetectInstalled() {
		options = append(options, PromptOption{Label: "Codex", Value: agentCodex})
		if cfg.Agent.Codex.Enabled && defaultAgent == "" {
			defaultAgent = agentCodex
		}
	}
	if s.zcodeIntegration != nil && s.zcodeIntegration.DetectInstalled() {
		options = append(options, PromptOption{Label: "ZCode", Value: agentZcode})
		if cfg.Agent.ZCode.Enabled && defaultAgent == "" {
			defaultAgent = agentZcode
		}
	}
	if s.grokIntegration != nil && s.grokIntegration.DetectInstalled() {
		options = append(options, PromptOption{Label: "Grok", Value: agentGrok})
		if cfg.Agent.Grok.Enabled && defaultAgent == "" {
			defaultAgent = agentGrok
		}
	}
	return options, defaultAgent
}

func promptChannelSelection(prompter Prompter, channels config.ChannelsConfig) (channelSelection, error) {
	choices, err := prompter.MultiSelect(i18n.T("setup.select_channels"), channelOptions(), currentChannelValues(channels))
	if err != nil {
		return channelSelection{}, err
	}
	return channelSelectionFromChoices(choices), nil
}

func currentChannelValues(channels config.ChannelsConfig) []string {
	opts := channelOptions()
	values := make([]string, 0, len(opts))
	if channels.System.Enabled {
		values = append(values, channelSystem)
	}
	if channels.Feishu.Enabled {
		values = append(values, channelFeishu)
	}
	if channels.Wechat.Enabled {
		values = append(values, channelWechat)
	}
	if channels.WechatWork.Enabled {
		values = append(values, channelWXWork)
	}
	if channels.DingTalk.Enabled {
		values = append(values, channelDingTalk)
	}
	if channels.Bark.Enabled {
		values = append(values, channelBark)
	}
	if channels.Ntfy.Enabled {
		values = append(values, channelNtfy)
	}
	if channels.Slack.Enabled {
		values = append(values, channelSlack)
	}
	return values
}

func channelSelectionFromChoices(choices []string) channelSelection {
	return channelSelection{
		System:     slices.Contains(choices, channelSystem),
		Feishu:     slices.Contains(choices, channelFeishu),
		Wechat:     slices.Contains(choices, channelWechat),
		WechatWork: slices.Contains(choices, channelWXWork),
		DingTalk:   slices.Contains(choices, channelDingTalk),
		Bark:       slices.Contains(choices, channelBark),
		Ntfy:       slices.Contains(choices, channelNtfy),
		Slack:      slices.Contains(choices, channelSlack),
	}
}

func promptEvents(prompter Prompter, agent string, currentEvents []string) ([]string, error) {
	return prompter.MultiSelect(i18n.T("setup.select_events"), eventOptionsForAgent(agent), currentEvents)
}

func eventOptionsForAgent(agent string) []PromptOption {
	switch agent {
	case agentClaude:
		return claudeEventOptionsFn()
	case agentZcode:
		return zcodeEventOptionsFn()
	case agentGrok:
		return grokEventOptionsFn()
	default:
		return codexEventOptionsFn()
	}
}

func channelsForAgent(cfg config.Config, agent string) config.ChannelsConfig {
	switch agent {
	case agentClaude:
		return cfg.Notify.ClaudeCode.Channels
	case agentZcode:
		return cfg.Notify.ZCode.Channels
	case agentGrok:
		return cfg.Notify.Grok.Channels
	default:
		return cfg.Notify.Codex.Channels
	}
}

func eventsForAgent(cfg config.Config, agent string) []string {
	switch agent {
	case agentClaude:
		return cfg.Notify.ClaudeCode.Events
	case agentZcode:
		return cfg.Notify.ZCode.Events
	case agentGrok:
		return cfg.Notify.Grok.Events
	default:
		return cfg.Notify.Codex.Events
	}
}

func (s *Service) configureAgent(req configureAgentRequest) (configuredAgent, error) {
	switch req.agent {
	case agentClaude:
		return s.configureClaude(req)
	case agentCodex:
		return s.configureCodex(req)
	case agentZcode:
		return s.configureZcode(req)
	case agentGrok:
		return s.configureGrok(req)
	default:
		return configuredAgent{}, fmt.Errorf("unsupported agent: %s", req.agent)
	}
}

func (s *Service) configureClaude(req configureAgentRequest) (configuredAgent, error) {
	next := req.cfg
	next.Notify.ClaudeCode.Channels = applyChannelSelection(next.Notify.ClaudeCode.Channels, req.channels)
	next.Notify.ClaudeCode.Events = dedupeStrings(req.events)
	if err := s.prepareSelectedChannels(req.ctx, req.channels); err != nil {
		return configuredAgent{}, err
	}
	channels, err := promptWebhookURLs(req.prompter, next.Notify.ClaudeCode.Channels, req.channels)
	if err != nil {
		return configuredAgent{}, err
	}
	next.Notify.ClaudeCode.Channels = channels

	agentScope := normalizedInstallScope(next.Agent.ClaudeCode.InstallScope)
	settingsPath, err := s.claudeIntegration.SettingsPath(agentScope)
	if err != nil {
		return configuredAgent{}, fmt.Errorf("%s: %w", i18n.T("setup.claude_hooks_err"), err)
	}
	resolvedBinary := common.ResolveBinaryPath(req.binaryPath)
	if err := s.claudeIntegration.Install(settingsPath, resolvedBinary); err != nil {
		return configuredAgent{}, fmt.Errorf("%s: %w", i18n.T("setup.claude_install_err"), err)
	}
	req.output.Writef(i18n.T("setup.claude_hooks_done"), settingsPath)
	next.Agent.ClaudeCode.InstallScope = agentScope
	next.Agent.ClaudeCode.Enabled = true
	return configuredAgent{cfg: next, settingsPath: settingsPath}, nil
}

func (s *Service) configureCodex(req configureAgentRequest) (configuredAgent, error) {
	next := req.cfg
	next.Notify.Codex.Channels = applyChannelSelection(next.Notify.Codex.Channels, req.channels)
	next.Notify.Codex.Events = dedupeStrings(req.events)
	if err := s.prepareSelectedChannels(req.ctx, req.channels); err != nil {
		return configuredAgent{}, err
	}
	channels, err := promptWebhookURLs(req.prompter, next.Notify.Codex.Channels, req.channels)
	if err != nil {
		return configuredAgent{}, err
	}
	next.Notify.Codex.Channels = channels

	agentScope := normalizedInstallScope(next.Agent.Codex.InstallScope)
	settingsPath, err := s.codexIntegration.SettingsPath(agentScope)
	if err != nil {
		return configuredAgent{}, fmt.Errorf("%s: %w", i18n.T("setup.codex_hooks_err"), err)
	}
	resolvedBinary := common.ResolveBinaryPath(req.binaryPath)
	if err := s.codexIntegration.Install(settingsPath, resolvedBinary); err != nil {
		return configuredAgent{}, fmt.Errorf("%s: %w", i18n.T("setup.codex_install_err"), err)
	}
	req.output.Writef(i18n.T("setup.codex_hooks_done"), settingsPath)
	req.output.Writef(i18n.T("setup.codex_tip"))
	next.Agent.Codex.InstallScope = agentScope
	next.Agent.Codex.Enabled = true
	return configuredAgent{cfg: next, settingsPath: settingsPath}, nil
}

// configureZcode 配置 ZCode 的通知渠道、事件，并把 hook 写入
// ~/.zcode/cli/config.json（user scope）或 .zcode/cli/config.json（project scope）。
func (s *Service) configureZcode(req configureAgentRequest) (configuredAgent, error) {
	next := req.cfg
	next.Notify.ZCode.Channels = applyChannelSelection(next.Notify.ZCode.Channels, req.channels)
	next.Notify.ZCode.Events = dedupeStrings(req.events)
	if err := s.prepareSelectedChannels(req.ctx, req.channels); err != nil {
		return configuredAgent{}, err
	}
	channels, err := promptWebhookURLs(req.prompter, next.Notify.ZCode.Channels, req.channels)
	if err != nil {
		return configuredAgent{}, err
	}
	next.Notify.ZCode.Channels = channels

	agentScope := normalizedInstallScope(next.Agent.ZCode.InstallScope)
	settingsPath, err := s.zcodeIntegration.SettingsPath(agentScope)
	if err != nil {
		return configuredAgent{}, fmt.Errorf("%s: %w", i18n.T("setup.zcode_hooks_err"), err)
	}
	resolvedBinary := common.ResolveBinaryPath(req.binaryPath)
	if err := s.zcodeIntegration.Install(settingsPath, resolvedBinary); err != nil {
		return configuredAgent{}, fmt.Errorf("%s: %w", i18n.T("setup.zcode_install_err"), err)
	}
	req.output.Writef(i18n.T("setup.zcode_hooks_done"), settingsPath)
	req.output.Writef(i18n.T("setup.zcode_tip"))
	next.Agent.ZCode.InstallScope = agentScope
	next.Agent.ZCode.Enabled = true
	return configuredAgent{cfg: next, settingsPath: settingsPath}, nil
}

// configureGrok 配置 Grok 的通知渠道、事件，并把 hook 写入
// ~/.grok/hooks/agent-notify.json（user scope）或 .grok/hooks/agent-notify.json（project scope）。
func (s *Service) configureGrok(req configureAgentRequest) (configuredAgent, error) {
	next := req.cfg
	next.Notify.Grok.Channels = applyChannelSelection(next.Notify.Grok.Channels, req.channels)
	next.Notify.Grok.Events = dedupeStrings(req.events)
	if err := s.prepareSelectedChannels(req.ctx, req.channels); err != nil {
		return configuredAgent{}, err
	}
	channels, err := promptWebhookURLs(req.prompter, next.Notify.Grok.Channels, req.channels)
	if err != nil {
		return configuredAgent{}, err
	}
	next.Notify.Grok.Channels = channels

	agentScope := normalizedInstallScope(next.Agent.Grok.InstallScope)
	settingsPath, err := s.grokIntegration.SettingsPath(agentScope)
	if err != nil {
		return configuredAgent{}, fmt.Errorf("%s: %w", i18n.T("setup.grok_hooks_err"), err)
	}
	resolvedBinary := common.ResolveBinaryPath(req.binaryPath)
	if err := s.grokIntegration.Install(settingsPath, resolvedBinary); err != nil {
		return configuredAgent{}, fmt.Errorf("%s: %w", i18n.T("setup.grok_install_err"), err)
	}
	req.output.Writef(i18n.T("setup.grok_hooks_done"), settingsPath)
	req.output.Writef(i18n.T("setup.grok_tip"))
	next.Agent.Grok.InstallScope = agentScope
	next.Agent.Grok.Enabled = true
	return configuredAgent{cfg: next, settingsPath: settingsPath}, nil
}

func applyChannelSelection(channels config.ChannelsConfig, selection channelSelection) config.ChannelsConfig {
	next := channels
	next.System.Enabled = selection.System
	next.Feishu.Enabled = selection.Feishu
	next.Wechat.Enabled = selection.Wechat
	next.WechatWork.Enabled = selection.WechatWork
	next.DingTalk.Enabled = selection.DingTalk
	next.Bark.Enabled = selection.Bark
	next.Ntfy.Enabled = selection.Ntfy
	next.Slack.Enabled = selection.Slack
	return next
}

func (s *Service) prepareSelectedChannels(ctx context.Context, selection channelSelection) error {
	if !selection.Feishu {
		return nil
	}
	if err := s.prepareFeishu(ctx); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("setup.feishu_init_err"), err)
	}
	return nil
}

func promptWebhookURLs(
	prompter Prompter,
	channels config.ChannelsConfig,
	selection channelSelection,
) (config.ChannelsConfig, error) {
	next := channels
	if selection.Wechat {
		webhookURL, err := prompter.Input(i18n.T("prompt.wechat_personal_webhook"), next.Wechat.WebhookURL)
		if err != nil {
			return config.ChannelsConfig{}, err
		}
		next.Wechat.WebhookURL = webhookURL
	}
	if selection.WechatWork {
		webhookURL, err := prompter.Input(i18n.T("prompt.wechat_webhook"), next.WechatWork.WebhookURL)
		if err != nil {
			return config.ChannelsConfig{}, err
		}
		next.WechatWork.WebhookURL = webhookURL
	}
	if selection.DingTalk {
		webhookURL, err := prompter.Input(i18n.T("prompt.dingtalk_webhook"), next.DingTalk.WebhookURL)
		if err != nil {
			return config.ChannelsConfig{}, err
		}
		next.DingTalk.WebhookURL = webhookURL
	}
	if selection.Bark {
		webhookURL, err := prompter.Input(i18n.T("prompt.bark_webhook"), next.Bark.WebhookURL)
		if err != nil {
			return config.ChannelsConfig{}, err
		}
		next.Bark.WebhookURL = webhookURL
	}
	if selection.Ntfy {
		topicURL, err := prompter.Input(i18n.T("prompt.ntfy_topic_url"), next.Ntfy.TopicURL)
		if err != nil {
			return config.ChannelsConfig{}, err
		}
		next.Ntfy.TopicURL = topicURL
	}
	if selection.Slack {
		webhookURL, err := prompter.Input(i18n.T("prompt.slack_webhook"), next.Slack.WebhookURL)
		if err != nil {
			return config.ChannelsConfig{}, err
		}
		next.Slack.WebhookURL = webhookURL
	}
	return next, nil
}

func normalizedInstallScope(scope string) string {
	if scope == installScopePrj {
		return installScopePrj
	}
	return installScopeUsr
}
