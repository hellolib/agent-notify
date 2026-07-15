package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the root configuration structure for agent-notify.
type Config struct {
	Version  int            `yaml:"version"`  // 配置版本号
	Agent    AgentConfig    `yaml:"agent"`    // Agent 安装配置
	Notify   NotifyConfig   `yaml:"notify"`   // 通知配置
	Behavior BehaviorConfig `yaml:"behavior"` // 行为配置
}

// AgentConfig holds configuration for supported agents.
type AgentConfig struct {
	ClaudeCode AgentTargetConfig `yaml:"claude_code"` // Claude Code 配置
	Codex      AgentTargetConfig `yaml:"codex"`       // Codex 配置
	ZCode      AgentTargetConfig `yaml:"zcode"`       // ZCode 配置
	Grok       AgentTargetConfig `yaml:"grok"`        // Grok 配置
}

// AgentTargetConfig holds configuration for a specific agent.
type AgentTargetConfig struct {
	Enabled      bool   `yaml:"enabled"`       // 是否启用该 Agent 的通知
	InstallScope string `yaml:"install_scope"` // 安装范围: user 或 project
}

// NotifyConfig holds notification configuration for all agents.
type NotifyConfig struct {
	ClaudeCode AgentNotifyConfig `yaml:"claude_code"` // Claude Code 通知配置
	Codex      AgentNotifyConfig `yaml:"codex"`       // Codex 通知配置
	ZCode      AgentNotifyConfig `yaml:"zcode"`       // ZCode 通知配置
	Grok       AgentNotifyConfig `yaml:"grok"`        // Grok 通知配置
}

// AgentNotifyConfig holds notification configuration for a single agent.
type AgentNotifyConfig struct {
	Events   []string       `yaml:"events,omitempty"` // 通知事件列表，如: permission_required, input_required, run_completed, run_failed
	Channels ChannelsConfig `yaml:"channels"`         // 通知渠道配置
}

// ChannelsConfig holds configuration for notification channels.
type ChannelsConfig struct {
	Feishu     ChannelConfig           `yaml:"feishu"`      // 飞书通知配置
	System     SystemChannelConfig     `yaml:"system"`      // 系统通知配置
	Wechat     WechatChannelConfig     `yaml:"wechat"`      // 微信个人/网关 Webhook 通知配置
	WechatWork WechatWorkChannelConfig `yaml:"wechat_work"` // 企业微信通知配置
	DingTalk   DingTalkChannelConfig   `yaml:"dingtalk"`    // 钉钉通知配置
	Bark       BarkChannelConfig       `yaml:"bark"`        // Bark 通知配置
	Ntfy       NtfyChannelConfig       `yaml:"ntfy"`        // Ntfy 通知配置
	Slack      SlackChannelConfig      `yaml:"slack"`       // Slack 通知配置
}

// ChannelConfig holds configuration for a single notification channel.
type ChannelConfig struct {
	Enabled bool `yaml:"enabled"` // 是否启用该通知渠道
}

// SystemChannelConfig holds configuration for OS-native system notifications.
type SystemChannelConfig struct {
	Enabled      bool `yaml:"enabled"`        // 是否启用系统通知渠道
	ClickToFocus bool `yaml:"click_to_focus"` // 点击通知是否激活宿主应用；识别不到 BundleID 时自动降级
}

// WechatChannelConfig holds configuration for personal WeChat / notify-gateway webhooks.
// Payload: {"msgType":"text","title":"...","content":"..."}.
type WechatChannelConfig struct {
	Enabled    bool   `yaml:"enabled"`     // 是否启用微信通知
	WebhookURL string `yaml:"webhook_url"` // 推送 API URL，如 https://host/api/notify/xxx
}

// WechatWorkChannelConfig holds configuration for WeChat Work (企业微信) webhook notifications.
type WechatWorkChannelConfig struct {
	Enabled    bool   `yaml:"enabled"`     // 是否启用企业微信通知
	WebhookURL string `yaml:"webhook_url"` // 群机器人 Webhook URL
}

// DingTalkChannelConfig holds configuration for DingTalk (钉钉) webhook notifications.
type DingTalkChannelConfig struct {
	Enabled    bool   `yaml:"enabled"`     // 是否启用钉钉通知
	WebhookURL string `yaml:"webhook_url"` // 群机器人 Webhook URL
}

// BarkChannelConfig holds configuration for Bark webhook notifications.
type BarkChannelConfig struct {
	Enabled    bool   `yaml:"enabled"`     // 是否启用 Bark 通知
	WebhookURL string `yaml:"webhook_url"` // Bark 推送 URL
}

// NtfyChannelConfig holds configuration for ntfy.sh topic notifications.
type NtfyChannelConfig struct {
	Enabled  bool   `yaml:"enabled"`   // 是否启用 Ntfy 通知
	TopicURL string `yaml:"topic_url"` // Ntfy topic URL, e.g. https://ntfy.sh/mytopic
}

// SlackChannelConfig holds configuration for Slack Incoming Webhook notifications.
type SlackChannelConfig struct {
	Enabled    bool   `yaml:"enabled"`     // 是否启用 Slack 通知
	WebhookURL string `yaml:"webhook_url"` // Slack Incoming Webhook URL
}

// BehaviorConfig holds behavior configuration.
type BehaviorConfig struct {
	DedupeSeconds      int    `yaml:"dedupe_seconds"`       // 去重时间窗口（秒），同一事件在此时间内不重复发送
	SendTimeoutSeconds int    `yaml:"send_timeout_seconds"` // 发送超时时间（秒）
	Locale             string `yaml:"locale"`               // 语言设置，如: zh-CN, en-US
}

func Default() Config {
	allEvents := []string{"permission_required", "input_required", "run_completed", "run_failed"}
	// Codex hooks 当前可靠支持的两个事件
	codexEvents := []string{"permission_required", "run_completed"}
	// ZCode hooks 支持的事件：与 Claude Code 基本一致，但没有 input_required
	// （ZCode 没有 Notification 事件），并新增 session_start。
	zcodeEvents := []string{"session_start", "permission_required", "run_completed", "run_failed"}
	// Grok hooks 支持 SessionStart / Notification / Stop / StopFailure / PostToolUseFailure。
	// 无 PermissionRequest；授权等待通过 Notification 映射为 permission_required。
	grokEvents := []string{"session_start", "permission_required", "input_required", "run_completed", "run_failed"}

	// BREAKING (vs pre-Grok defaults): Claude Code is no longer enabled by default,
	// and System notification is no longer pre-enabled for any agent.
	//
	// Previously Default() set Agent.ClaudeCode.Enabled=true and
	// Notify.ClaudeCode/ZCode/Grok Channels.System.Enabled=true. That made
	// "view config" show unconfigured agents as ready after a single-agent setup,
	// and channel-menu init could enable webhooks on agents the user never chose.
	//
	// Channels and agents now start disabled. Enabling happens only when the user
	// runs the setup wizard or channel-menu init for agents that are already enabled.
	// Existing ~/.agent-notify/config.yaml files are unaffected (Load preserves values).
	// New installs must run `agent-notify` / the setup wizard once.
	disabledChannels := func() ChannelsConfig {
		return ChannelsConfig{
			System:     SystemChannelConfig{Enabled: false, ClickToFocus: true},
			Feishu:     ChannelConfig{Enabled: false},
			Wechat:     WechatChannelConfig{Enabled: false, WebhookURL: ""},
			WechatWork: WechatWorkChannelConfig{Enabled: false, WebhookURL: ""},
			DingTalk:   DingTalkChannelConfig{Enabled: false, WebhookURL: ""},
			Bark:       BarkChannelConfig{Enabled: false, WebhookURL: ""},
			Ntfy:       NtfyChannelConfig{Enabled: false, TopicURL: ""},
			Slack:      SlackChannelConfig{Enabled: false, WebhookURL: ""},
		}
	}

	return Config{
		Version: 1,
		Agent: AgentConfig{
			ClaudeCode: AgentTargetConfig{
				Enabled:      false,
				InstallScope: "user",
			},
			Codex: AgentTargetConfig{
				Enabled:      false,
				InstallScope: "user",
			},
			ZCode: AgentTargetConfig{
				Enabled:      false,
				InstallScope: "user",
			},
			Grok: AgentTargetConfig{
				Enabled:      false,
				InstallScope: "user",
			},
		},
		Notify: NotifyConfig{
			ClaudeCode: AgentNotifyConfig{
				Events:   append([]string(nil), allEvents...),
				Channels: disabledChannels(),
			},
			Codex: AgentNotifyConfig{
				Events:   append([]string(nil), codexEvents...),
				Channels: disabledChannels(),
			},
			ZCode: AgentNotifyConfig{
				Events:   append([]string(nil), zcodeEvents...),
				Channels: disabledChannels(),
			},
			Grok: AgentNotifyConfig{
				Events:   append([]string(nil), grokEvents...),
				Channels: disabledChannels(),
			},
		},
		Behavior: BehaviorConfig{
			DedupeSeconds:      60,
			SendTimeoutSeconds: 5,
			Locale:             "zh-CN",
		},
	}
}

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agent-notify", "config.yaml"), nil
}

func StatePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agent-notify", "state.json"), nil
}

func LogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".agent-notify", "agent-notify.log"), nil
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, err
	}

	// 先解析到空结构体，避免默认值干扰
	cfg := Config{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	// 填充默认值（仅对未设置的字段）
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Agent.ClaudeCode.InstallScope == "" {
		cfg.Agent.ClaudeCode.InstallScope = "user"
	}
	if cfg.Agent.Codex.InstallScope == "" {
		cfg.Agent.Codex.InstallScope = "user"
	}
	if cfg.Agent.ZCode.InstallScope == "" {
		cfg.Agent.ZCode.InstallScope = "user"
	}
	if cfg.Agent.Grok.InstallScope == "" {
		cfg.Agent.Grok.InstallScope = "user"
	}
	if cfg.Behavior.DedupeSeconds == 0 {
		cfg.Behavior.DedupeSeconds = 60
	}
	if cfg.Behavior.SendTimeoutSeconds == 0 {
		cfg.Behavior.SendTimeoutSeconds = 5
	}
	if cfg.Behavior.Locale == "" {
		cfg.Behavior.Locale = "zh-CN"
	}

	// Channel-only setup (e.g. menu → 微信) enables webhooks without writing events.
	// Empty events mean dispatch never sends; backfill defaults when any channel is on.
	def := Default()
	cfg.Notify.ClaudeCode.Events = ensureEvents(cfg.Notify.ClaudeCode, def.Notify.ClaudeCode.Events)
	cfg.Notify.Codex.Events = ensureEvents(cfg.Notify.Codex, def.Notify.Codex.Events)
	cfg.Notify.ZCode.Events = ensureEvents(cfg.Notify.ZCode, def.Notify.ZCode.Events)
	cfg.Notify.Grok.Events = ensureEvents(cfg.Notify.Grok, def.Notify.Grok.Events)

	return cfg, nil
}

// ensureEvents keeps existing events. When channels are enabled but events were never
// persisted (common after channel-menu-only setup), fill agent-specific defaults.
func ensureEvents(notifyCfg AgentNotifyConfig, defaults []string) []string {
	if len(notifyCfg.Events) > 0 {
		return notifyCfg.Events
	}
	if !anyChannelEnabled(notifyCfg.Channels) {
		return notifyCfg.Events
	}
	return append([]string(nil), defaults...)
}

func anyChannelEnabled(c ChannelsConfig) bool {
	return c.System.Enabled ||
		c.Feishu.Enabled ||
		c.Wechat.Enabled ||
		c.WechatWork.Enabled ||
		c.DingTalk.Enabled ||
		c.Bark.Enabled ||
		c.Ntfy.Enabled ||
		c.Slack.Enabled
}

func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}
