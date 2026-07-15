package doctor

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/testutil"
)

// mockIntegration implements agentintegrations.Integration for testing
type mockIntegration struct {
	name            string
	detectInstalled bool
	settingsPath    string
	isHookInstalled bool
}

func (m *mockIntegration) Name() string {
	return m.name
}

func (m *mockIntegration) DetectInstalled() bool {
	return m.detectInstalled
}

func (m *mockIntegration) SettingsPath(scope string) (string, error) {
	return m.settingsPath, nil
}

func (m *mockIntegration) Install(settingsPath, binaryPath string) error {
	return nil
}

func (m *mockIntegration) Uninstall(settingsPath string) error {
	return nil
}

func (m *mockIntegration) IsHookInstalled(settingsPath string) (bool, error) {
	return m.isHookInstalled, nil
}

// mockOutputWriter implements OutputWriter for testing
type mockOutputWriter struct {
	output string
}

func (m *mockOutputWriter) Writef(format string, args ...any) {
	m.output += fmt.Sprintf(format, args...)
}

func TestNewService(t *testing.T) {
	svc := NewService()
	if svc == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestNewServiceWithOptions(t *testing.T) {
	svc := NewService(
		WithClaudeIntegration(&mockIntegration{name: "Claude Code"}),
		WithCodexIntegration(&mockIntegration{name: "Codex"}),
	)
	if svc == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestService_Run(t *testing.T) {
	svc := NewService(
		WithClaudeIntegration(&mockIntegration{
			name:            "Claude Code",
			detectInstalled: true,
			settingsPath:    "/tmp/.claude/settings.json",
			isHookInstalled: true,
		}),
		WithCodexIntegration(&mockIntegration{
			name:            "Codex",
			detectInstalled: false,
			settingsPath:    "/tmp/.codex/hooks.json",
			isHookInstalled: false,
		}),
		WithZcodeIntegration(&mockIntegration{name: "ZCode", detectInstalled: false}),
		WithGrokIntegration(&mockIntegration{name: "Grok", detectInstalled: false}),
	)

	result, err := svc.Run()
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.ClaudeInstalled {
		t.Error("expected ClaudeInstalled to be true")
	}
	if result.CodexInstalled {
		t.Error("expected CodexInstalled to be false")
	}
	if !result.ClaudeHookInstalled {
		t.Error("expected ClaudeHookInstalled to be true")
	}
	if result.ClaudeIntegrationStatus != StatusInstalled {
		t.Fatalf("ClaudeIntegrationStatus = %q, want %q", result.ClaudeIntegrationStatus, StatusInstalled)
	}
	if result.CodexIntegrationStatus != StatusAgentMissing {
		t.Fatalf("CodexIntegrationStatus = %q, want %q", result.CodexIntegrationStatus, StatusAgentMissing)
	}
}

func TestService_Run_ConfigPresenceAffectsIntegrationStatus(t *testing.T) {
	testutil.IsolateHome(t)

	svc := NewService(
		WithClaudeIntegration(&mockIntegration{
			name:            "Claude Code",
			detectInstalled: true,
			settingsPath:    "/tmp/.claude/settings.json",
			isHookInstalled: true,
		}),
		WithCodexIntegration(&mockIntegration{
			name:            "Codex",
			detectInstalled: true,
			settingsPath:    "/tmp/.codex/hooks.json",
			isHookInstalled: false,
		}),
		WithZcodeIntegration(&mockIntegration{name: "ZCode", detectInstalled: false}),
		WithGrokIntegration(&mockIntegration{name: "Grok", detectInstalled: false}),
	)

	result, err := svc.Run()
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	// ConfigExists depends on the real ~/.agent-notify/config.yaml; only assert
	// the integration status that follows from ConfigExists.
	if result.ConfigExists {
		if result.ClaudeIntegrationStatus != StatusInstalled {
			t.Fatalf("ClaudeIntegrationStatus = %q, want %q when config exists", result.ClaudeIntegrationStatus, StatusInstalled)
		}
		if result.CodexIntegrationStatus != StatusIntegrationMissing {
			t.Fatalf("CodexIntegrationStatus = %q, want %q when config exists", result.CodexIntegrationStatus, StatusIntegrationMissing)
		}
		return
	}
	if result.ClaudeIntegrationStatus != StatusConfigMissing {
		t.Fatalf("ClaudeIntegrationStatus = %q, want %q", result.ClaudeIntegrationStatus, StatusConfigMissing)
	}
	if result.CodexIntegrationStatus != StatusConfigMissing {
		t.Fatalf("CodexIntegrationStatus = %q, want %q", result.CodexIntegrationStatus, StatusConfigMissing)
	}
}

func TestService_Print(t *testing.T) {
	svc := NewService()
	output := &mockOutputWriter{}

	result := &DiagnosticsResult{
		ConfigPath:              "/tmp/config.yaml",
		ConfigExists:            true,
		ClaudeInstalled:         true,
		ClaudeHookInstalled:     true,
		CodexInstalled:          false,
		CodexHookInstalled:      false,
		SystemNotifyAvailable:   true,
		SystemNotifyName:        "系统通知",
		FeishuCLIReady:          true,
		ClaudeFeishuEnabled:     true,
		ClaudeSystemEnabled:     true,
		ClaudeWechatEnabled:     true,
		ClaudeWechatWorkEnabled: false,
		CodexFeishuEnabled:      false,
		CodexSystemEnabled:      false,
		ClaudeIntegrationStatus: StatusInstalled,
		CodexIntegrationStatus:  StatusAgentMissing,
	}

	svc.Print(output, result)

	if output.output == "" {
		t.Error("expected non-empty output")
	}
	if !strings.Contains(output.output, "✅ 已安装") {
		t.Fatal("expected installed status to appear in output")
	}
	if !strings.Contains(output.output, "❌ 未安装 Agent") {
		t.Fatal("expected missing-agent status to appear in output")
	}
	// Channel row must include personal WeChat + WeCom (8 status cells).
	// Regression: missing WeChat arg produced "%!s(MISSING)" under Slack.
	if strings.Contains(output.output, "%!") {
		t.Fatalf("channel table has printf formatting error:\n%s", output.output)
	}
	// Claude Code row: Feishu ✅, System ✅, WeChat ✅, WeCom ❌ ...
	// Agent name is left-padded to 12 runes via %-12s ("Claude Code ").
	if !strings.Contains(output.output, "| Claude Code  |  ✅  |  ✅  |  ✅  |  ❌  |") {
		t.Fatalf("expected Claude channel row with personal WeChat enabled:\n%s", output.output)
	}
}

func TestIntegrationStatus(t *testing.T) {
	tests := []struct {
		name                 string
		configExists         bool
		agentInstalled       bool
		integrationInstalled bool
		want                 DiagnosticStatus
	}{
		{name: "agent missing wins", configExists: true, agentInstalled: false, integrationInstalled: true, want: StatusAgentMissing},
		{name: "config missing after installed agent", configExists: false, agentInstalled: true, integrationInstalled: true, want: StatusConfigMissing},
		{name: "integration missing after config exists", configExists: true, agentInstalled: true, integrationInstalled: false, want: StatusIntegrationMissing},
		{name: "installed", configExists: true, agentInstalled: true, integrationInstalled: true, want: StatusInstalled},
	}

	for _, tt := range tests {
		got := integrationStatus(tt.configExists, tt.agentInstalled, tt.integrationInstalled)
		if got != tt.want {
			t.Fatalf("%s: integrationStatus() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestVisualWidth(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"abc", 3},
		{"中文", 4}, // Chinese characters are 2 columns each
		{"a中b文c", 7},
		{"", 0},
	}

	for _, tt := range tests {
		result := visualWidth(tt.input)
		if result != tt.expected {
			t.Errorf("visualWidth(%q) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		input       string
		targetWidth int
		expected    string
	}{
		{"abc", 5, "abc  "},
		{"abc", 3, "abc"},
		{"中文", 6, "中文  "},
		{"中文", 4, "中文"},
	}

	for _, tt := range tests {
		result := padRight(tt.input, tt.targetWidth)
		if result != tt.expected {
			t.Errorf("padRight(%q, %d) = %q, want %q", tt.input, tt.targetWidth, result, tt.expected)
		}
	}
}
