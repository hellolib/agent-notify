// Package setup provides the setup/init flow service for agent-notify.
// It handles agent configuration and hook installation.
package setup

import (
	"context"

	"github.com/hellolib/agent-notify/internal/agentintegrations"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/i18n"
)

// Prompter interface for user interactions.
type Prompter interface {
	Select(message string, options []PromptOption, defaultValue string) (string, error)
	MultiSelect(message string, options []PromptOption, defaults []string) ([]string, error)
	Confirm(message string, defaultValue bool) (bool, error)
	Input(message, defaultValue string) (string, error)
}

// PromptOption represents a selectable option in prompts.
type PromptOption struct {
	Label string
	Value string
}

// FeishuPreparer prepares the Feishu CLI for use.
type FeishuPreparer interface {
	EnsureReady(ctx context.Context) error
}

// OutputWriter handles output messages.
type OutputWriter interface {
	Writef(format string, args ...any)
}

// Service handles the init/setup flow for agent-notify.
type Service struct {
	claudeIntegration agentintegrations.Integration
	codexIntegration  agentintegrations.Integration
	zcodeIntegration  agentintegrations.Integration
	grokIntegration   agentintegrations.Integration
	feishuPreparer    FeishuPreparer
	configLoader      ConfigLoader
}

// ConfigLoader loads and saves configuration.
type ConfigLoader interface {
	Load(path string) (config.Config, error)
	Save(path string, cfg config.Config) error
	DefaultPath() (string, error)
}

// SetupResult contains the result of a setup operation.
type SetupResult struct {
	Agent        string
	ConfigPath   string
	SettingsPath string
}

// NewService creates a new setup service.
func NewService(opts ...Option) *Service {
	s := &Service{
		claudeIntegration: agentintegrations.NewClaudeIntegration(),
		codexIntegration:  agentintegrations.NewCodexIntegration(),
		zcodeIntegration:  agentintegrations.NewZcodeIntegration(),
		grokIntegration:   agentintegrations.NewGrokIntegration(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Option configures the service.
type Option func(*Service)

// WithClaudeIntegration sets the Claude integration.
func WithClaudeIntegration(i agentintegrations.Integration) Option {
	return func(s *Service) { s.claudeIntegration = i }
}

// WithCodexIntegration sets the Codex integration.
func WithCodexIntegration(i agentintegrations.Integration) Option {
	return func(s *Service) { s.codexIntegration = i }
}

// WithZcodeIntegration sets the ZCode integration.
func WithZcodeIntegration(i agentintegrations.Integration) Option {
	return func(s *Service) { s.zcodeIntegration = i }
}

// WithGrokIntegration sets the Grok integration.
func WithGrokIntegration(i agentintegrations.Integration) Option {
	return func(s *Service) { s.grokIntegration = i }
}

// WithFeishuPreparer sets the Feishu preparer.
func WithFeishuPreparer(p FeishuPreparer) Option {
	return func(s *Service) { s.feishuPreparer = p }
}

// WithConfigLoader sets the config loader.
func WithConfigLoader(l ConfigLoader) Option {
	return func(s *Service) { s.configLoader = l }
}

func claudeEventOptionsFn() []PromptOption {
	return []PromptOption{
		{Label: i18n.T("event.permission_required"), Value: "permission_required"},
		{Label: i18n.T("event.input_required"), Value: "input_required"},
		{Label: i18n.T("event.run_completed"), Value: "run_completed"},
		{Label: i18n.T("event.run_failed"), Value: "run_failed"},
	}
}

// codexEventOptionsFn returns event options for Codex.
// Only PermissionRequest → permission_required and Stop → run_completed are reliably supported.
func codexEventOptionsFn() []PromptOption {
	return []PromptOption{
		{Label: i18n.T("event.permission_required"), Value: "permission_required"},
		{Label: i18n.T("event.run_completed"), Value: "run_completed"},
	}
}

// zcodeEventOptionsFn returns event options for ZCode.
// ZCode 支持 SessionStart / PermissionRequest / PostToolUseFailure / Stop，
// 映射为 session_start / permission_required / run_failed / run_completed。
// 注意 ZCode 没有 Notification 事件，因此没有 input_required。
func zcodeEventOptionsFn() []PromptOption {
	return []PromptOption{
		{Label: i18n.T("event.session_start"), Value: "session_start"},
		{Label: i18n.T("event.permission_required"), Value: "permission_required"},
		{Label: i18n.T("event.run_completed"), Value: "run_completed"},
		{Label: i18n.T("event.run_failed"), Value: "run_failed"},
	}
}

// grokEventOptionsFn returns event options for Grok.
// Grok 支持 SessionStart / Notification / Stop / StopFailure / PostToolUseFailure，
// 映射为 session_start / input_required|permission_required / run_completed / run_failed。
func grokEventOptionsFn() []PromptOption {
	return []PromptOption{
		{Label: i18n.T("event.session_start"), Value: "session_start"},
		{Label: i18n.T("event.permission_required"), Value: "permission_required"},
		{Label: i18n.T("event.input_required"), Value: "input_required"},
		{Label: i18n.T("event.run_completed"), Value: "run_completed"},
		{Label: i18n.T("event.run_failed"), Value: "run_failed"},
	}
}

// Run executes the init flow.
func (s *Service) Run(ctx context.Context, prompter Prompter, output OutputWriter, configPath, binaryPath string) (*SetupResult, error) {
	cfg, path, err := s.loadConfig(configPath)
	if err != nil {
		return nil, err
	}

	selectedAgent, err := s.selectAgent(prompter, cfg)
	if err != nil {
		return nil, err
	}

	channels, err := promptChannelSelection(prompter, channelsForAgent(cfg, selectedAgent))
	if err != nil {
		return nil, err
	}
	if !channels.hasAny() {
		return s.disableAgentNotification(cfg, path, selectedAgent, output)
	}

	events, err := promptEvents(prompter, selectedAgent, eventsForAgent(cfg, selectedAgent))
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return s.disableAgentNotification(cfg, path, selectedAgent, output)
	}

	configured, err := s.configureAgent(configureAgentRequest{
		ctx: ctx, prompter: prompter, output: output, cfg: cfg,
		agent: selectedAgent, channels: channels, events: events, binaryPath: binaryPath,
	})
	if err != nil {
		return nil, err
	}
	if err := s.saveConfig(path, configured.cfg); err != nil {
		return nil, err
	}
	output.Writef(i18n.T("setup.config_file"), path)

	return &SetupResult{Agent: selectedAgent, ConfigPath: path, SettingsPath: configured.settingsPath}, nil
}

func (s *Service) loadConfig(configPath string) (config.Config, string, error) {
	path := configPath
	var err error
	if path == "" {
		path, err = s.defaultConfigPath()
		if err != nil {
			return config.Config{}, "", err
		}
	}
	cfg, err := s.loadConfigFile(path)
	if err != nil {
		return config.Config{}, "", err
	}
	return cfg, path, nil
}

func (s *Service) saveConfig(path string, cfg config.Config) error {
	if s.configLoader != nil {
		return s.configLoader.Save(path, cfg)
	}
	return config.Save(path, cfg)
}

func (s *Service) defaultConfigPath() (string, error) {
	if s.configLoader != nil {
		return s.configLoader.DefaultPath()
	}
	return config.DefaultPath()
}

func (s *Service) loadConfigFile(path string) (config.Config, error) {
	if s.configLoader != nil {
		return s.configLoader.Load(path)
	}
	return config.Load(path)
}

func (s *Service) prepareFeishu(ctx context.Context) error {
	if s.feishuPreparer != nil {
		return s.feishuPreparer.EnsureReady(ctx)
	}
	return nil
}

func dedupeStrings(items []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0)
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	return result
}

// disableAgentNotification disables all notification channels for the given agent
// and saves the configuration. This is called when the user doesn't select any
// channels or events.
func (s *Service) disableAgentNotification(cfg config.Config, path, agent string, output OutputWriter) (*SetupResult, error) {
	switch agent {
	case "claude":
		cfg.Notify.ClaudeCode.Channels.Feishu.Enabled = false
		cfg.Notify.ClaudeCode.Channels.System.Enabled = false
		cfg.Notify.ClaudeCode.Channels.Wechat.Enabled = false
		cfg.Notify.ClaudeCode.Channels.Wechat.WebhookURL = ""
		cfg.Notify.ClaudeCode.Channels.WechatWork.Enabled = false
		cfg.Notify.ClaudeCode.Channels.DingTalk.Enabled = false
		cfg.Notify.ClaudeCode.Channels.Bark.Enabled = false
		cfg.Notify.ClaudeCode.Channels.Ntfy.Enabled = false
		cfg.Notify.ClaudeCode.Channels.Slack.Enabled = false
		cfg.Notify.ClaudeCode.Channels.Slack.WebhookURL = ""
		cfg.Notify.ClaudeCode.Events = nil
		cfg.Agent.ClaudeCode.Enabled = false
	case "codex":
		cfg.Notify.Codex.Channels.Feishu.Enabled = false
		cfg.Notify.Codex.Channels.System.Enabled = false
		cfg.Notify.Codex.Channels.Wechat.Enabled = false
		cfg.Notify.Codex.Channels.Wechat.WebhookURL = ""
		cfg.Notify.Codex.Channels.WechatWork.Enabled = false
		cfg.Notify.Codex.Channels.DingTalk.Enabled = false
		cfg.Notify.Codex.Channels.Bark.Enabled = false
		cfg.Notify.Codex.Channels.Ntfy.Enabled = false
		cfg.Notify.Codex.Channels.Slack.Enabled = false
		cfg.Notify.Codex.Channels.Slack.WebhookURL = ""
		cfg.Notify.Codex.Events = nil
		cfg.Agent.Codex.Enabled = false
	case "zcode":
		cfg.Notify.ZCode.Channels.Feishu.Enabled = false
		cfg.Notify.ZCode.Channels.System.Enabled = false
		cfg.Notify.ZCode.Channels.Wechat.Enabled = false
		cfg.Notify.ZCode.Channels.Wechat.WebhookURL = ""
		cfg.Notify.ZCode.Channels.WechatWork.Enabled = false
		cfg.Notify.ZCode.Channels.DingTalk.Enabled = false
		cfg.Notify.ZCode.Channels.Bark.Enabled = false
		cfg.Notify.ZCode.Channels.Ntfy.Enabled = false
		cfg.Notify.ZCode.Channels.Slack.Enabled = false
		cfg.Notify.ZCode.Channels.Slack.WebhookURL = ""
		cfg.Notify.ZCode.Events = nil
		cfg.Agent.ZCode.Enabled = false
	case "grok":
		cfg.Notify.Grok.Channels.Feishu.Enabled = false
		cfg.Notify.Grok.Channels.System.Enabled = false
		cfg.Notify.Grok.Channels.Wechat.Enabled = false
		cfg.Notify.Grok.Channels.Wechat.WebhookURL = ""
		cfg.Notify.Grok.Channels.WechatWork.Enabled = false
		cfg.Notify.Grok.Channels.DingTalk.Enabled = false
		cfg.Notify.Grok.Channels.Bark.Enabled = false
		cfg.Notify.Grok.Channels.Ntfy.Enabled = false
		cfg.Notify.Grok.Channels.Slack.Enabled = false
		cfg.Notify.Grok.Channels.Slack.WebhookURL = ""
		cfg.Notify.Grok.Events = nil
		cfg.Agent.Grok.Enabled = false
	}

	if err := s.saveConfig(path, cfg); err != nil {
		return nil, err
	}
	output.Writef(i18n.T("clean.agent_closed"), agentName(agent))
	output.Writef(i18n.T("setup.config_file"), path)

	return &SetupResult{
		Agent:      agent,
		ConfigPath: path,
	}, nil
}

func agentName(agent string) string {
	switch agent {
	case "claude":
		return "Claude Code"
	case "codex":
		return "Codex"
	case "zcode":
		return "ZCode"
	case "grok":
		return "Grok"
	default:
		return agent
	}
}
