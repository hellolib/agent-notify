package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hellolib/agent-notify/internal/agentintegrations"
	"github.com/hellolib/agent-notify/internal/app/doctor"
	"github.com/hellolib/agent-notify/internal/app/setup"
	"github.com/hellolib/agent-notify/internal/app/tester"
	"github.com/hellolib/agent-notify/internal/claudehooks"
	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/grokhooks"
	"github.com/hellolib/agent-notify/internal/i18n"
	"github.com/hellolib/agent-notify/internal/zcodehooks"
)

// cliPrompter adapts CLI Prompter to setup.Prompter
type cliPrompter struct {
	p Prompter
}

func (cp *cliPrompter) Select(message string, options []setup.PromptOption, defaultValue string) (string, error) {
	cliOptions := make([]PromptOption, len(options))
	for i, o := range options {
		cliOptions[i] = PromptOption{Label: o.Label, Value: o.Value}
	}
	return cp.p.Select(message, cliOptions, defaultValue)
}

func (cp *cliPrompter) MultiSelect(message string, options []setup.PromptOption, defaults []string) ([]string, error) {
	cliOptions := make([]PromptOption, len(options))
	for i, o := range options {
		cliOptions[i] = PromptOption{Label: o.Label, Value: o.Value}
	}
	return cp.p.MultiSelect(message, cliOptions, defaults)
}

func (cp *cliPrompter) Confirm(message string, defaultValue bool) (bool, error) {
	return cp.p.Confirm(message, defaultValue)
}

func (cp *cliPrompter) Input(message, defaultValue string) (string, error) {
	return cp.p.Input(message, defaultValue)
}

// cliOutputWriter adapts Streams to setup/doctor OutputWriter
type cliOutputWriter struct {
	streams Streams
}

func (w *cliOutputWriter) Writef(format string, args ...any) {
	fmt.Fprintf(w.streams.Stdout, format, args...)
}

// feishuPreparerAdapter adapts the prepareFeishuCLI function to setup.FeishuPreparer
type feishuPreparerAdapter struct{}

func (f *feishuPreparerAdapter) EnsureReady(ctx context.Context) error {
	return prepareFeishuCLI(ctx)
}

func runInitFlow(ctx context.Context, streams Streams, prompter Prompter, configPath, settingsPath, binaryPath string) error {
	_ = settingsPath // kept for backward compatibility

	svc := setup.NewService(
		setup.WithClaudeIntegration(agentintegrations.NewClaudeIntegration()),
		setup.WithCodexIntegration(agentintegrations.NewCodexIntegration()),
		setup.WithZcodeIntegration(agentintegrations.NewZcodeIntegration()),
		setup.WithGrokIntegration(agentintegrations.NewGrokIntegration()),
		setup.WithFeishuPreparer(&feishuPreparerAdapter{}),
	)

	cliPrompter := &cliPrompter{p: prompter}
	output := &cliOutputWriter{streams: streams}

	_, err := svc.Run(ctx, cliPrompter, output, configPath, binaryPath)
	return err
}

func runPrintClaudeHooks(streams Streams, binaryPath string) error {
	settings := claudehooks.BuildHookSettings(common.ResolveBinaryPath(binaryPath))
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(streams.Stdout, string(data))
	return err
}

func runInstallClaudeHooks(scope, binaryPath string) error {
	path, err := settingsPathForAgent("claude", scope)
	if err != nil {
		return err
	}
	return claudehooks.Install(path, common.ResolveBinaryPath(binaryPath))
}

func runPrintZcodeHooks(streams Streams, binaryPath string) error {
	settings := zcodehooks.BuildHookSettings(common.ResolveBinaryPath(binaryPath))
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(streams.Stdout, string(data))
	return err
}

func runInstallZcodeHooks(scope, binaryPath string) error {
	path, err := settingsPathForAgent("zcode", scope)
	if err != nil {
		return err
	}
	return zcodehooks.Install(path, common.ResolveBinaryPath(binaryPath))
}

func runPrintGrokHooks(streams Streams, binaryPath string) error {
	settings := grokhooks.BuildHookSettings(common.ResolveBinaryPath(binaryPath))
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(streams.Stdout, string(data))
	return err
}

func runInstallGrokHooks(scope, binaryPath string) error {
	path, err := settingsPathForAgent("grok", scope)
	if err != nil {
		return err
	}
	return grokhooks.Install(path, common.ResolveBinaryPath(binaryPath))
}

func runTestFeishu(ctx context.Context, streams Streams) error {
	svc := tester.NewService(
		tester.WithFeishuPreparer(&feishuPreparerAdapter{}),
	)
	result, err := svc.TestFeishu(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintln(streams.Stdout, "✅ "+result.Message)
	return nil
}

func runTestSystem(ctx context.Context, streams Streams) error {
	svc := tester.NewService()
	result, err := svc.TestSystem(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintln(streams.Stdout, "✅ "+result.Message)
	return nil
}

func runTestWechatWork(ctx context.Context, streams Streams) error {
	cfg, _, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	// Try claude config first, fall back to codex, zcode, then grok
	webhookURL := cfg.Notify.ClaudeCode.Channels.WechatWork.WebhookURL
	if webhookURL == "" {
		webhookURL = cfg.Notify.Codex.Channels.WechatWork.WebhookURL
	}
	if webhookURL == "" {
		webhookURL = cfg.Notify.ZCode.Channels.WechatWork.WebhookURL
	}
	if webhookURL == "" {
		webhookURL = cfg.Notify.Grok.Channels.WechatWork.WebhookURL
	}
	if webhookURL == "" {
		return fmt.Errorf("%s", i18n.T("err.wechat_not_configured"))
	}

	svc := tester.NewService()
	result, err := svc.TestWechatWork(ctx, webhookURL)
	if err != nil {
		return err
	}
	fmt.Fprintln(streams.Stdout, "✅ "+result.Message)
	return nil
}

func runInitWechatWork(streams Streams, prompter Prompter) error {
	cfg, path, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	// Get current webhook URL (use claude's if available)
	currentURL := cfg.Notify.ClaudeCode.Channels.WechatWork.WebhookURL
	if currentURL == "" {
		currentURL = cfg.Notify.Codex.Channels.WechatWork.WebhookURL
	}
	if currentURL == "" {
		currentURL = cfg.Notify.ZCode.Channels.WechatWork.WebhookURL
	}
	if currentURL == "" {
		currentURL = cfg.Notify.Grok.Channels.WechatWork.WebhookURL
	}

	webhookURL, err := prompter.Input(i18n.T("prompt.wechat_webhook"), currentURL)
	if err != nil {
		return err
	}

	// Store URL on all agents; enable only for agents already configured.
	applyChannelToAgents(&cfg, func(agentEnabled bool, notify *config.AgentNotifyConfig) {
		notify.Channels.WechatWork.WebhookURL = webhookURL
		notify.Channels.WechatWork.Enabled = agentEnabled
	})

	if err := config.Save(path, cfg); err != nil {
		return fmt.Errorf("%s: %w", i18n.T("err.save_failed"), err)
	}

	fmt.Fprintln(streams.Stdout, i18n.T("wechat.init_done"))
	fmt.Fprintf(streams.Stdout, i18n.T("msg.config_file")+"\n", path)
	return nil
}

func runDoctor(streams Streams) error {
	svc := doctor.NewService(
		doctor.WithClaudeIntegration(agentintegrations.NewClaudeIntegration()),
		doctor.WithCodexIntegration(agentintegrations.NewCodexIntegration()),
		doctor.WithZcodeIntegration(agentintegrations.NewZcodeIntegration()),
		doctor.WithGrokIntegration(agentintegrations.NewGrokIntegration()),
	)
	result, err := svc.Run()
	if err != nil {
		return err
	}
	output := &cliOutputWriter{streams: streams}
	svc.Print(output, result)
	return nil
}

func loadDefaultConfig() (config.Config, string, error) {
	path, err := config.DefaultPath()
	if err != nil {
		return config.Config{}, "", err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return config.Config{}, "", err
	}
	return cfg, path, nil
}

func printCurrentNotifyConfig(streams Streams) error {
	cfg, path, err := loadDefaultConfig()
	if err != nil {
		return err
	}

	fmt.Fprintf(streams.Stdout, i18n.T("msg.config_file")+"\n\n", path)

	statusIcon := func(enabled bool) string {
		if enabled {
			return "✅"
		}
		return "❌"
	}

	// Fixed width table with ASCII borders.
	fmt.Fprintln(streams.Stdout, i18n.T("view.separator"))
	fmt.Fprintln(streams.Stdout, i18n.T("view.header"))
	fmt.Fprintln(streams.Stdout, i18n.T("view.separator"))
	// Columns: Feishu | System | WeChat | WeCom | DingTalk | Bark | Ntfy | Slack
	fmt.Fprintf(streams.Stdout, i18n.T("view.row_format")+"\n", "Claude Code",
		statusIcon(cfg.Notify.ClaudeCode.Channels.Feishu.Enabled),
		statusIcon(cfg.Notify.ClaudeCode.Channels.System.Enabled),
		statusIcon(cfg.Notify.ClaudeCode.Channels.Wechat.Enabled),
		statusIcon(cfg.Notify.ClaudeCode.Channels.WechatWork.Enabled),
		statusIcon(cfg.Notify.ClaudeCode.Channels.DingTalk.Enabled),
		statusIcon(cfg.Notify.ClaudeCode.Channels.Bark.Enabled),
		statusIcon(cfg.Notify.ClaudeCode.Channels.Ntfy.Enabled),
		statusIcon(cfg.Notify.ClaudeCode.Channels.Slack.Enabled))
	fmt.Fprintf(streams.Stdout, i18n.T("view.row_format")+"\n", "Codex",
		statusIcon(cfg.Notify.Codex.Channels.Feishu.Enabled),
		statusIcon(cfg.Notify.Codex.Channels.System.Enabled),
		statusIcon(cfg.Notify.Codex.Channels.Wechat.Enabled),
		statusIcon(cfg.Notify.Codex.Channels.WechatWork.Enabled),
		statusIcon(cfg.Notify.Codex.Channels.DingTalk.Enabled),
		statusIcon(cfg.Notify.Codex.Channels.Bark.Enabled),
		statusIcon(cfg.Notify.Codex.Channels.Ntfy.Enabled),
		statusIcon(cfg.Notify.Codex.Channels.Slack.Enabled))
	fmt.Fprintf(streams.Stdout, i18n.T("view.row_format")+"\n", "ZCode",
		statusIcon(cfg.Notify.ZCode.Channels.Feishu.Enabled),
		statusIcon(cfg.Notify.ZCode.Channels.System.Enabled),
		statusIcon(cfg.Notify.ZCode.Channels.Wechat.Enabled),
		statusIcon(cfg.Notify.ZCode.Channels.WechatWork.Enabled),
		statusIcon(cfg.Notify.ZCode.Channels.DingTalk.Enabled),
		statusIcon(cfg.Notify.ZCode.Channels.Bark.Enabled),
		statusIcon(cfg.Notify.ZCode.Channels.Ntfy.Enabled),
		statusIcon(cfg.Notify.ZCode.Channels.Slack.Enabled))
	fmt.Fprintf(streams.Stdout, i18n.T("view.row_format")+"\n", "Grok",
		statusIcon(cfg.Notify.Grok.Channels.Feishu.Enabled),
		statusIcon(cfg.Notify.Grok.Channels.System.Enabled),
		statusIcon(cfg.Notify.Grok.Channels.Wechat.Enabled),
		statusIcon(cfg.Notify.Grok.Channels.WechatWork.Enabled),
		statusIcon(cfg.Notify.Grok.Channels.DingTalk.Enabled),
		statusIcon(cfg.Notify.Grok.Channels.Bark.Enabled),
		statusIcon(cfg.Notify.Grok.Channels.Ntfy.Enabled),
		statusIcon(cfg.Notify.Grok.Channels.Slack.Enabled))
	fmt.Fprintln(streams.Stdout, i18n.T("view.separator"))

	return nil
}

// settingsPathForAgent returns the settings path for the given agent and scope.
// Currently only Claude has manual install-hooks subcommands; the Codex path is
// handled exclusively through the init flow + CodexIntegration.
func settingsPathForAgent(agent, scope string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch agent {
	case "claude":
		switch scope {
		case "user":
			return filepath.Join(home, ".claude", "settings.json"), nil
		case "project":
			return filepath.Join(".claude", "settings.json"), nil
		default:
			return "", fmt.Errorf("unsupported scope: %s", scope)
		}
	case "zcode":
		switch scope {
		case "user":
			return filepath.Join(home, ".zcode", "cli", "config.json"), nil
		case "project":
			return filepath.Join(".zcode", "cli", "config.json"), nil
		default:
			return "", fmt.Errorf("unsupported scope: %s", scope)
		}
	case "grok":
		switch scope {
		case "user":
			return filepath.Join(home, ".grok", "hooks", "agent-notify.json"), nil
		case "project":
			return filepath.Join(".grok", "hooks", "agent-notify.json"), nil
		default:
			return "", fmt.Errorf("unsupported scope: %s", scope)
		}
	default:
		return "", fmt.Errorf("unsupported agent: %s", agent)
	}
}
