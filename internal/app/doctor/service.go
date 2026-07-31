// Package doctor provides the diagnostics service for agent-notify.
// It checks the current notification setup and reports status.
package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hellolib/agent-notify/internal/agentintegrations"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/feishucli"
	"github.com/hellolib/agent-notify/internal/i18n"
	"github.com/hellolib/agent-notify/internal/state"
)

// OutputWriter handles output messages.
type OutputWriter interface {
	Writef(format string, args ...any)
}

// Service handles diagnostics for agent-notify.
type Service struct {
	claudeIntegration agentintegrations.Integration
	codexIntegration  agentintegrations.Integration
	zcodeIntegration  agentintegrations.Integration
	grokIntegration   agentintegrations.Integration
	droidIntegration  agentintegrations.Integration
}

// NewService creates a new doctor service.
func NewService(opts ...Option) *Service {
	s := &Service{
		claudeIntegration: agentintegrations.NewClaudeIntegration(),
		codexIntegration:  agentintegrations.NewCodexIntegration(),
		zcodeIntegration:  agentintegrations.NewZcodeIntegration(),
		grokIntegration:   agentintegrations.NewGrokIntegration(),
		droidIntegration:  agentintegrations.NewDroidIntegration(),
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

// WithDroidIntegration sets the Droid integration.
func WithDroidIntegration(i agentintegrations.Integration) Option {
	return func(s *Service) { s.droidIntegration = i }
}

type DiagnosticStatus string

const (
	StatusInstalled          DiagnosticStatus = "installed"
	StatusAgentMissing       DiagnosticStatus = "agent_missing"
	StatusConfigMissing      DiagnosticStatus = "config_missing"
	StatusIntegrationMissing DiagnosticStatus = "integration_missing"
	// StatusBinaryMissing:hook 已注册,但 command 指向的二进制不存在
	// (典型:先本地构建安装、后改用 npx,旧路径已删除,issue #34)
	StatusBinaryMissing DiagnosticStatus = "binary_missing"
)

// DiagnosticsResult contains diagnostic results.
type DiagnosticsResult struct {
	ConfigPath                string
	ConfigExists              bool
	ClaudeInstalled           bool
	ClaudeHookInstalled       bool
	CodexInstalled            bool
	CodexHookInstalled        bool
	SystemNotifyAvailable     bool
	SystemNotifyName          string
	ClickFocusHelperAvailable bool
	FeishuCLIReady            bool
	ClaudeFeishuEnabled       bool
	ClaudeSystemEnabled       bool
	ClaudeWechatEnabled       bool
	ClaudeWechatWorkEnabled   bool
	ClaudeDingTalkEnabled     bool
	ClaudeBarkEnabled         bool
	ClaudeNtfyEnabled         bool
	ClaudeSlackEnabled        bool
	CodexFeishuEnabled        bool
	CodexSystemEnabled        bool
	CodexWechatEnabled        bool
	CodexWechatWorkEnabled    bool
	CodexDingTalkEnabled      bool
	CodexBarkEnabled          bool
	CodexNtfyEnabled          bool
	CodexSlackEnabled         bool
	ZcodeInstalled            bool
	ZcodeHookInstalled        bool
	ZcodeFeishuEnabled        bool
	ZcodeSystemEnabled        bool
	ZcodeWechatEnabled        bool
	ZcodeWechatWorkEnabled    bool
	ZcodeDingTalkEnabled      bool
	ZcodeBarkEnabled          bool
	ZcodeNtfyEnabled          bool
	ZcodeSlackEnabled         bool
	GrokInstalled             bool
	GrokHookInstalled         bool
	GrokFeishuEnabled         bool
	GrokSystemEnabled         bool
	GrokWechatEnabled         bool
	GrokWechatWorkEnabled     bool
	GrokDingTalkEnabled       bool
	GrokBarkEnabled           bool
	GrokNtfyEnabled           bool
	GrokSlackEnabled          bool
	DroidInstalled            bool
	DroidHookInstalled        bool
	DroidFeishuEnabled        bool
	DroidSystemEnabled        bool
	DroidWechatEnabled        bool
	DroidWechatWorkEnabled    bool
	DroidDingTalkEnabled      bool
	DroidBarkEnabled          bool
	DroidNtfyEnabled          bool
	DroidSlackEnabled         bool
	ClaudeIntegrationStatus   DiagnosticStatus
	CodexIntegrationStatus    DiagnosticStatus
	ZcodeIntegrationStatus    DiagnosticStatus
	GrokIntegrationStatus     DiagnosticStatus
	DroidIntegrationStatus    DiagnosticStatus

	// Per-agent system-channel focus precision (effective "app"|"window").
	ClaudeSystemFocusPrecision string
	CodexSystemFocusPrecision  string
	ZcodeSystemFocusPrecision  string
	GrokSystemFocusPrecision   string
	DroidSystemFocusPrecision  string

	// Temporary notification freeze (from freeze.json).
	FreezeActive   bool
	FreezeUntil    string
	FreezeChannels string
	FreezeRemain   string
}

// Run executes diagnostics and returns results.
func (s *Service) Run() (*DiagnosticsResult, error) {
	result := &DiagnosticsResult{}

	// Detect agents
	result.ClaudeInstalled = s.claudeIntegration.DetectInstalled()
	result.CodexInstalled = s.codexIntegration.DetectInstalled()
	result.ZcodeInstalled = s.zcodeIntegration != nil && s.zcodeIntegration.DetectInstalled()
	result.GrokInstalled = s.grokIntegration != nil && s.grokIntegration.DetectInstalled()
	result.DroidInstalled = s.droidIntegration != nil && s.droidIntegration.DetectInstalled()

	// System notification detection
	result.SystemNotifyAvailable, result.SystemNotifyName = detectSystemNotification()
	result.ClickFocusHelperAvailable = detectClickFocusHelper()

	// Config
	cfgPath, _ := config.DefaultPath()
	result.ConfigPath = cfgPath
	cfg, cfgLoadErr := config.Load(cfgPath)
	_, cfgErr := os.Stat(cfgPath)
	result.ConfigExists = cfgErr == nil

	// hook 已注册但 command 指向的二进制不存在时,集成实际不可用(issue #34)
	var claudeBinaryMissing, codexBinaryMissing, zcodeBinaryMissing, grokBinaryMissing, droidBinaryMissing bool

	// Claude hooks settings
	claudeSettingsPath, _ := s.claudeIntegration.SettingsPath("user")
	if claudeSettingsPath != "" {
		installed, err := s.claudeIntegration.IsHookInstalled(claudeSettingsPath)
		result.ClaudeHookInstalled = err == nil && installed
		claudeBinaryMissing = result.ClaudeHookInstalled && hookBinaryMissing(claudeSettingsPath, "handle-claude-hook")
	}

	// Codex hooks settings
	codexSettingsPath, _ := s.codexIntegration.SettingsPath("user")
	if codexSettingsPath != "" {
		installed, err := s.codexIntegration.IsHookInstalled(codexSettingsPath)
		result.CodexHookInstalled = err == nil && installed
		codexBinaryMissing = result.CodexHookInstalled && hookBinaryMissing(codexSettingsPath, "handle-codex-hook")
	}

	// ZCode hooks settings
	if s.zcodeIntegration != nil {
		zcodeSettingsPath, _ := s.zcodeIntegration.SettingsPath("user")
		if zcodeSettingsPath != "" {
			installed, err := s.zcodeIntegration.IsHookInstalled(zcodeSettingsPath)
			result.ZcodeHookInstalled = err == nil && installed
			zcodeBinaryMissing = result.ZcodeHookInstalled && hookBinaryMissing(zcodeSettingsPath, "handle-zcode-hook")
		}
	}

	// Grok hooks settings
	if s.grokIntegration != nil {
		grokSettingsPath, _ := s.grokIntegration.SettingsPath("user")
		if grokSettingsPath != "" {
			installed, err := s.grokIntegration.IsHookInstalled(grokSettingsPath)
			result.GrokHookInstalled = err == nil && installed
			grokBinaryMissing = result.GrokHookInstalled && hookBinaryMissing(grokSettingsPath, "handle-grok-hook")
		}
	}

	// Droid hooks settings
	if s.droidIntegration != nil {
		droidSettingsPath, _ := s.droidIntegration.SettingsPath("user")
		if droidSettingsPath != "" {
			installed, err := s.droidIntegration.IsHookInstalled(droidSettingsPath)
			result.DroidHookInstalled = err == nil && installed
			droidBinaryMissing = result.DroidHookInstalled && hookBinaryMissing(droidSettingsPath, "handle-droid-hook")
		}
	}

	// Config values
	result.ClaudeFeishuEnabled = cfgLoadErr == nil && cfg.Notify.ClaudeCode.Channels.Feishu.Enabled
	result.ClaudeSystemEnabled = cfgLoadErr == nil && cfg.Notify.ClaudeCode.Channels.System.Enabled
	result.ClaudeWechatEnabled = cfgLoadErr == nil && cfg.Notify.ClaudeCode.Channels.Wechat.Enabled
	result.ClaudeWechatWorkEnabled = cfgLoadErr == nil && cfg.Notify.ClaudeCode.Channels.WechatWork.Enabled
	result.ClaudeDingTalkEnabled = cfgLoadErr == nil && cfg.Notify.ClaudeCode.Channels.DingTalk.Enabled
	result.ClaudeBarkEnabled = cfgLoadErr == nil && cfg.Notify.ClaudeCode.Channels.Bark.Enabled
	result.ClaudeNtfyEnabled = cfgLoadErr == nil && cfg.Notify.ClaudeCode.Channels.Ntfy.Enabled
	result.ClaudeSlackEnabled = cfgLoadErr == nil && cfg.Notify.ClaudeCode.Channels.Slack.Enabled
	result.CodexFeishuEnabled = cfgLoadErr == nil && cfg.Notify.Codex.Channels.Feishu.Enabled
	result.CodexSystemEnabled = cfgLoadErr == nil && cfg.Notify.Codex.Channels.System.Enabled
	result.CodexWechatEnabled = cfgLoadErr == nil && cfg.Notify.Codex.Channels.Wechat.Enabled
	result.CodexWechatWorkEnabled = cfgLoadErr == nil && cfg.Notify.Codex.Channels.WechatWork.Enabled
	result.CodexDingTalkEnabled = cfgLoadErr == nil && cfg.Notify.Codex.Channels.DingTalk.Enabled
	result.CodexBarkEnabled = cfgLoadErr == nil && cfg.Notify.Codex.Channels.Bark.Enabled
	result.CodexNtfyEnabled = cfgLoadErr == nil && cfg.Notify.Codex.Channels.Ntfy.Enabled
	result.CodexSlackEnabled = cfgLoadErr == nil && cfg.Notify.Codex.Channels.Slack.Enabled
	result.ZcodeFeishuEnabled = cfgLoadErr == nil && cfg.Notify.ZCode.Channels.Feishu.Enabled
	result.ZcodeSystemEnabled = cfgLoadErr == nil && cfg.Notify.ZCode.Channels.System.Enabled
	result.ZcodeWechatEnabled = cfgLoadErr == nil && cfg.Notify.ZCode.Channels.Wechat.Enabled
	result.ZcodeWechatWorkEnabled = cfgLoadErr == nil && cfg.Notify.ZCode.Channels.WechatWork.Enabled
	result.ZcodeDingTalkEnabled = cfgLoadErr == nil && cfg.Notify.ZCode.Channels.DingTalk.Enabled
	result.ZcodeBarkEnabled = cfgLoadErr == nil && cfg.Notify.ZCode.Channels.Bark.Enabled
	result.ZcodeNtfyEnabled = cfgLoadErr == nil && cfg.Notify.ZCode.Channels.Ntfy.Enabled
	result.ZcodeSlackEnabled = cfgLoadErr == nil && cfg.Notify.ZCode.Channels.Slack.Enabled
	result.GrokFeishuEnabled = cfgLoadErr == nil && cfg.Notify.Grok.Channels.Feishu.Enabled
	result.GrokSystemEnabled = cfgLoadErr == nil && cfg.Notify.Grok.Channels.System.Enabled
	result.GrokWechatEnabled = cfgLoadErr == nil && cfg.Notify.Grok.Channels.Wechat.Enabled
	result.GrokWechatWorkEnabled = cfgLoadErr == nil && cfg.Notify.Grok.Channels.WechatWork.Enabled
	result.GrokDingTalkEnabled = cfgLoadErr == nil && cfg.Notify.Grok.Channels.DingTalk.Enabled
	result.GrokBarkEnabled = cfgLoadErr == nil && cfg.Notify.Grok.Channels.Bark.Enabled
	result.GrokNtfyEnabled = cfgLoadErr == nil && cfg.Notify.Grok.Channels.Ntfy.Enabled
	result.GrokSlackEnabled = cfgLoadErr == nil && cfg.Notify.Grok.Channels.Slack.Enabled
	result.DroidFeishuEnabled = cfgLoadErr == nil && cfg.Notify.Droid.Channels.Feishu.Enabled
	result.DroidSystemEnabled = cfgLoadErr == nil && cfg.Notify.Droid.Channels.System.Enabled
	result.DroidWechatEnabled = cfgLoadErr == nil && cfg.Notify.Droid.Channels.Wechat.Enabled
	result.DroidWechatWorkEnabled = cfgLoadErr == nil && cfg.Notify.Droid.Channels.WechatWork.Enabled
	result.DroidDingTalkEnabled = cfgLoadErr == nil && cfg.Notify.Droid.Channels.DingTalk.Enabled
	result.DroidBarkEnabled = cfgLoadErr == nil && cfg.Notify.Droid.Channels.Bark.Enabled
	result.DroidNtfyEnabled = cfgLoadErr == nil && cfg.Notify.Droid.Channels.Ntfy.Enabled
	result.DroidSlackEnabled = cfgLoadErr == nil && cfg.Notify.Droid.Channels.Slack.Enabled

	// Per-agent effective system focus precision, read fresh from the
	// AGENT_NOTIFY_FOCUS_PRECISION environment variable.
	result.ClaudeSystemFocusPrecision = config.FocusPrecisionFromEnv()
	result.CodexSystemFocusPrecision = config.FocusPrecisionFromEnv()
	result.ZcodeSystemFocusPrecision = config.FocusPrecisionFromEnv()
	result.GrokSystemFocusPrecision = config.FocusPrecisionFromEnv()
	result.DroidSystemFocusPrecision = config.FocusPrecisionFromEnv()

	result.ClaudeIntegrationStatus = integrationStatusWithBinary(result.ConfigExists, result.ClaudeInstalled, result.ClaudeHookInstalled, claudeBinaryMissing)
	result.CodexIntegrationStatus = integrationStatusWithBinary(result.ConfigExists, result.CodexInstalled, result.CodexHookInstalled, codexBinaryMissing)
	result.ZcodeIntegrationStatus = integrationStatusWithBinary(result.ConfigExists, result.ZcodeInstalled, result.ZcodeHookInstalled, zcodeBinaryMissing)
	result.GrokIntegrationStatus = integrationStatusWithBinary(result.ConfigExists, result.GrokInstalled, result.GrokHookInstalled, grokBinaryMissing)
	result.DroidIntegrationStatus = integrationStatusWithBinary(result.ConfigExists, result.DroidInstalled, result.DroidHookInstalled, droidBinaryMissing)

	// Feishu CLI
	_, feishuCLIConfigErr := feishucli.ParseConfig()
	result.FeishuCLIReady = feishuCLIConfigErr == nil

	// 临时冻结状态（读失败视为未冻结）
	if statePath, err := config.StatePath(); err == nil {
		now := time.Now()
		st := state.NewFreezeStore(state.FreezePath(statePath)).Load()
		if st.Active(now) {
			result.FreezeActive = true
			result.FreezeUntil = st.Until.Local().Format("2006-01-02 15:04")
			result.FreezeChannels = strings.Join(st.Channels, ",")
			remain := st.Until.Sub(now).Round(time.Second)
			if remain >= time.Hour {
				h := int(remain / time.Hour)
				m := int((remain % time.Hour) / time.Minute)
				if m == 0 {
					result.FreezeRemain = fmt.Sprintf("%dh", h)
				} else {
					result.FreezeRemain = fmt.Sprintf("%dh%dm", h, m)
				}
			} else if remain >= time.Minute {
				result.FreezeRemain = fmt.Sprintf("%dm", int(remain/time.Minute))
			} else {
				result.FreezeRemain = fmt.Sprintf("%ds", int(remain/time.Second))
			}
		}
	}

	return result, nil
}

func integrationStatus(configExists, agentInstalled, integrationInstalled bool) DiagnosticStatus {
	return integrationStatusWithBinary(configExists, agentInstalled, integrationInstalled, false)
}

// integrationStatusWithBinary 在原有判定后追加一层:hook 注册了但二进制不在,
// 集成实际不可用,必须与「已安装」区分开(issue #34)。
func integrationStatusWithBinary(configExists, agentInstalled, integrationInstalled, binaryMissing bool) DiagnosticStatus {
	if !agentInstalled {
		return StatusAgentMissing
	}
	if !configExists {
		return StatusConfigMissing
	}
	if !integrationInstalled {
		return StatusIntegrationMissing
	}
	if binaryMissing {
		return StatusBinaryMissing
	}
	return StatusInstalled
}

// Print outputs the diagnostics result.
func (s *Service) Print(output OutputWriter, result *DiagnosticsResult) {
	// Config path header
	output.Writef(i18n.T("doctor.config_file"), result.ConfigPath)

	// Agent installation status table.
	output.Writef(i18n.T("doctor.agent_status") + "\n")
	output.Writef(i18n.T("doctor.agent_sep") + "\n")
	output.Writef(i18n.T("doctor.agent_header") + "\n")
	output.Writef(i18n.T("doctor.agent_sep") + "\n")

	claudeInstallStatus := padRight(i18n.T("status.not_installed"), 8)
	if result.ClaudeInstalled {
		claudeInstallStatus = padRight(i18n.T("status.installed"), 8)
	}
	claudeHookStatus := padRight(diagnosticStatusLabel(result.ClaudeIntegrationStatus), 14)
	output.Writef(i18n.T("doctor.row_format")+"\n", "Claude Code", claudeInstallStatus, claudeHookStatus)

	codexInstallStatus := padRight(i18n.T("status.not_installed"), 8)
	if result.CodexInstalled {
		codexInstallStatus = padRight(i18n.T("status.installed"), 8)
	}
	codexNotifyStatus := padRight(diagnosticStatusLabel(result.CodexIntegrationStatus), 14)
	output.Writef(i18n.T("doctor.row_format")+"\n", "Codex", codexInstallStatus, codexNotifyStatus)

	zcodeInstallStatus := padRight(i18n.T("status.not_installed"), 8)
	if result.ZcodeInstalled {
		zcodeInstallStatus = padRight(i18n.T("status.installed"), 8)
	}
	zcodeNotifyStatus := padRight(diagnosticStatusLabel(result.ZcodeIntegrationStatus), 14)
	output.Writef(i18n.T("doctor.row_format")+"\n", "ZCode", zcodeInstallStatus, zcodeNotifyStatus)

	grokInstallStatus := padRight(i18n.T("status.not_installed"), 8)
	if result.GrokInstalled {
		grokInstallStatus = padRight(i18n.T("status.installed"), 8)
	}
	grokNotifyStatus := padRight(diagnosticStatusLabel(result.GrokIntegrationStatus), 14)
	output.Writef(i18n.T("doctor.row_format")+"\n", "Grok", grokInstallStatus, grokNotifyStatus)

	droidInstallStatus := padRight(i18n.T("status.not_installed"), 8)
	if result.DroidInstalled {
		droidInstallStatus = padRight(i18n.T("status.installed"), 8)
	}
	droidNotifyStatus := padRight(diagnosticStatusLabel(result.DroidIntegrationStatus), 14)
	output.Writef(i18n.T("doctor.row_format")+"\n", "Droid", droidInstallStatus, droidNotifyStatus)

	output.Writef(i18n.T("doctor.agent_sep") + "\n")
	output.Writef("\n")

	// Notification channels table
	output.Writef(i18n.T("doctor.channel_status") + "\n")
	output.Writef(i18n.T("doctor.channel_sep") + "\n")
	output.Writef(i18n.T("doctor.channel_header") + "\n")
	output.Writef(i18n.T("doctor.channel_sep") + "\n")
	// Columns: Feishu | System | WeChat | WeCom | DingTalk | Bark | Ntfy | Slack
	channelRow := i18n.T("view.row_format") + "\n"
	output.Writef(channelRow, "Claude Code",
		boolIcon(result.ClaudeFeishuEnabled),
		boolIcon(result.ClaudeSystemEnabled),
		boolIcon(result.ClaudeWechatEnabled),
		boolIcon(result.ClaudeWechatWorkEnabled),
		boolIcon(result.ClaudeDingTalkEnabled),
		boolIcon(result.ClaudeBarkEnabled),
		boolIcon(result.ClaudeNtfyEnabled),
		boolIcon(result.ClaudeSlackEnabled),
	)
	output.Writef(channelRow, "Codex",
		boolIcon(result.CodexFeishuEnabled),
		boolIcon(result.CodexSystemEnabled),
		boolIcon(result.CodexWechatEnabled),
		boolIcon(result.CodexWechatWorkEnabled),
		boolIcon(result.CodexDingTalkEnabled),
		boolIcon(result.CodexBarkEnabled),
		boolIcon(result.CodexNtfyEnabled),
		boolIcon(result.CodexSlackEnabled),
	)
	output.Writef(channelRow, "ZCode",
		boolIcon(result.ZcodeFeishuEnabled),
		boolIcon(result.ZcodeSystemEnabled),
		boolIcon(result.ZcodeWechatEnabled),
		boolIcon(result.ZcodeWechatWorkEnabled),
		boolIcon(result.ZcodeDingTalkEnabled),
		boolIcon(result.ZcodeBarkEnabled),
		boolIcon(result.ZcodeNtfyEnabled),
		boolIcon(result.ZcodeSlackEnabled),
	)
	output.Writef(channelRow, "Grok",
		boolIcon(result.GrokFeishuEnabled),
		boolIcon(result.GrokSystemEnabled),
		boolIcon(result.GrokWechatEnabled),
		boolIcon(result.GrokWechatWorkEnabled),
		boolIcon(result.GrokDingTalkEnabled),
		boolIcon(result.GrokBarkEnabled),
		boolIcon(result.GrokNtfyEnabled),
		boolIcon(result.GrokSlackEnabled),
	)
	output.Writef(channelRow, "Droid",
		boolIcon(result.DroidFeishuEnabled),
		boolIcon(result.DroidSystemEnabled),
		boolIcon(result.DroidWechatEnabled),
		boolIcon(result.DroidWechatWorkEnabled),
		boolIcon(result.DroidDingTalkEnabled),
		boolIcon(result.DroidBarkEnabled),
		boolIcon(result.DroidNtfyEnabled),
		boolIcon(result.DroidSlackEnabled),
	)
	output.Writef(i18n.T("doctor.channel_sep") + "\n")
	output.Writef("\n")

	// System environment table
	output.Writef(i18n.T("doctor.system_env") + "\n")
	output.Writef(i18n.T("doctor.env_sep") + "\n")
	output.Writef(i18n.T("doctor.env_header") + "\n")
	output.Writef(i18n.T("doctor.env_sep") + "\n")

	configStatus := padRight(i18n.T("status.config_missing"), 10)
	if result.ConfigExists {
		configStatus = padRight(i18n.T("status.config_present"), 10)
	}
	output.Writef(i18n.T("doctor.env_row_format")+"\n", padRight(i18n.T("doctor.item_config"), 20), configStatus)

	systemNotifyName := i18n.T("doctor.system_notify_name")
	systemNotifyStatus := padRight(i18n.T("status.unavailable"), 10)
	if result.SystemNotifyAvailable {
		systemNotifyStatus = padRight(i18n.T("status.available"), 10)
	}
	output.Writef(i18n.T("doctor.env_row_format")+"\n", padRight(systemNotifyName, 20), systemNotifyStatus)

	// 点击聚焦 helper：macOS 检测 terminal-notifier，Windows 检测 toast-focus-helper。
	clickFocusStatus := padRight(i18n.T("status.unavailable"), 10)
	if result.ClickFocusHelperAvailable {
		clickFocusStatus = padRight(i18n.T("status.available"), 10)
	}
	output.Writef(i18n.T("doctor.env_row_format")+"\n", padRight(i18n.T("doctor.item_click_focus"), 20), clickFocusStatus)

	// macOS 焦点精度状态行：仅 darwin 显示，反映首个启用系统通知渠道的 agent 的聚焦精度。
	if runtime.GOOS == "darwin" {
		precision := firstEnabledAgentSystemPrecision(result)
		status := SummarizeMacFocus(precision, detectMacFocusHelper())
		output.Writef(i18n.T("doctor.env_row_format")+"\n", padRight(i18n.T("doctor.item_focus_precision"), 20), i18n.T(focusPrecisionI18nKey(status)))
	}

	feishuCLIStatus := padRight(i18n.T("status.not_configured"), 10)
	if result.FeishuCLIReady {
		feishuCLIStatus = padRight(i18n.T("status.ready"), 10)
	}
	output.Writef(i18n.T("doctor.env_row_format")+"\n", padRight(i18n.T("doctor.item_feishu_cli"), 20), feishuCLIStatus)
	if result.FreezeActive {
		freezeStatus := fmt.Sprintf(i18n.T("doctor.freeze_active"), result.FreezeUntil, result.FreezeRemain, result.FreezeChannels)
		output.Writef(i18n.T("doctor.env_row_format")+"\n", padRight(i18n.T("doctor.item_freeze"), 20), freezeStatus)
	} else {
		output.Writef(i18n.T("doctor.env_row_format")+"\n", padRight(i18n.T("doctor.item_freeze"), 20), i18n.T("doctor.freeze_inactive"))
	}

	output.Writef(i18n.T("doctor.env_sep") + "\n")
}

// focusPrecisionI18nKey maps a SummarizeMacFocus status token to its i18n key.
// Unknown/empty statuses default to the app-level key.
func focusPrecisionI18nKey(status string) string {
	switch status {
	case "window-ready":
		return "doctor.focus_precision_window_ready"
	case "window-degrade":
		return "doctor.focus_precision_window_degrade"
	default:
		return "doctor.focus_precision_app"
	}
}

// firstEnabledAgentSystemPrecision returns the effective system focus precision
// of the first agent (Claude, Codex, ZCode, Grok, Droid) with Channels.System.Enabled,
// or "app" when no agent has system notifications enabled.
func firstEnabledAgentSystemPrecision(result *DiagnosticsResult) string {
	precision := config.FocusPrecisionApp
	switch {
	case result.ClaudeSystemEnabled:
		precision = result.ClaudeSystemFocusPrecision
	case result.CodexSystemEnabled:
		precision = result.CodexSystemFocusPrecision
	case result.ZcodeSystemEnabled:
		precision = result.ZcodeSystemFocusPrecision
	case result.GrokSystemEnabled:
		precision = result.GrokSystemFocusPrecision
	case result.DroidSystemEnabled:
		precision = result.DroidSystemFocusPrecision
	}
	if precision != config.FocusPrecisionWindow {
		return config.FocusPrecisionApp
	}
	return config.FocusPrecisionWindow
}

// boolIcon returns the ✅/❌ icon for a boolean status.
func boolIcon(enabled bool) string {
	if enabled {
		return "✅"
	}
	return "❌"
}

// detectClickFocusHelper checks whether the platform click-to-focus helper is available.
func detectClickFocusHelper() bool {
	switch runtime.GOOS {
	case "darwin":
		return detectTerminalNotifier()
	case "linux":
		return detectLinuxFocusSupport()
	case "windows":
		return detectWindowsFocusHelper()
	default:
		return false
	}
}

// SummarizeMacFocus returns a stable machine-readable status for mac focus precision.
// Values: "app", "window-ready", "window-degrade".
// This pure, cross-platform function is i18n-free so it stays unit-testable;
// the doctor display layer (Task 9) maps these tokens to localized strings.
func SummarizeMacFocus(precision string, helperPresent bool) string {
	if precision != "window" {
		return "app"
	}
	if helperPresent {
		return "window-ready"
	}
	return "window-degrade"
}

// detectTerminalNotifier checks whether terminal-notifier is available.
// 优先识别随 npx 解压到 ~/.agent-notify/terminal-notifier.app 的本地预置 bundle，
// 其次查系统 PATH。
func detectTerminalNotifier() bool {
	if home, err := os.UserHomeDir(); err == nil {
		localExe := home + "/.agent-notify/terminal-notifier.app/Contents/MacOS/terminal-notifier"
		if info, err := os.Stat(localExe); err == nil && !info.IsDir() {
			return true
		}
	}
	if _, err := exec.LookPath("terminal-notifier"); err == nil {
		return true
	}
	return false
}

// detectSystemNotification checks if system notifications are available.
// Returns (available, displayName) where displayName is platform-specific.
func detectSystemNotification() (bool, string) {
	name := i18n.T("doctor.system_notify_name")
	switch runtime.GOOS {
	case "darwin":
		_, err := exec.LookPath("osascript")
		return err == nil, name
	case "linux":
		_, err := exec.LookPath("notify-send")
		return err == nil, name
	case "windows":
		// PowerShell is always available on Windows
		return true, name
	default:
		return false, name
	}
}

// visualWidth calculates the visual width of a string, treating Chinese characters as 2 columns.
func visualWidth(s string) int {
	width := 0
	for _, r := range s {
		if utf8.RuneLen(r) > 1 {
			// Chinese and other wide characters
			width += 2
		} else {
			width += 1
		}
	}
	return width
}

// padRight pads a string to the target visual width.
func padRight(s string, targetWidth int) string {
	currentWidth := visualWidth(s)
	if currentWidth >= targetWidth {
		return s
	}
	padding := targetWidth - currentWidth
	return s + strings.Repeat(" ", padding)
}

func diagnosticStatusLabel(status DiagnosticStatus) string {
	switch status {
	case StatusInstalled:
		return i18n.T("status.integration_installed")
	case StatusAgentMissing:
		return i18n.T("status.integration_agent_missing")
	case StatusBinaryMissing:
		return i18n.T("status.integration_binary_missing")
	case StatusConfigMissing:
		return i18n.T("status.integration_config_missing")
	case StatusIntegrationMissing:
		return i18n.T("status.integration_not_integrated")
	default:
		return i18n.T("status.integration_unknown")
	}
}
