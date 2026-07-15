package setup

import (
	"context"
	"testing"

	"github.com/hellolib/agent-notify/internal/config"
)

// mockIntegration implements agentintegrations.Integration for testing
type mockIntegration struct {
	name            string
	detectInstalled bool
	settingsPath    string
	installErr      error
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
	return m.installErr
}

func (m *mockIntegration) Uninstall(settingsPath string) error {
	return nil
}

func (m *mockIntegration) IsHookInstalled(settingsPath string) (bool, error) {
	return m.isHookInstalled, nil
}

// mockPrompter implements Prompter for testing
type mockPrompter struct {
	selectIdx     int
	selectResult  string
	multiResult   []string
	multiResults  [][]string
	multiOptions  [][]PromptOption
	confirmResult bool
	inputResult   string
	inputResults  []string
}

func (m *mockPrompter) Select(message string, options []PromptOption, defaultValue string) (string, error) {
	return m.selectResult, nil
}

func (m *mockPrompter) MultiSelect(message string, options []PromptOption, defaults []string) ([]string, error) {
	m.multiOptions = append(m.multiOptions, options)
	if len(m.multiResults) > 0 {
		value := m.multiResults[0]
		m.multiResults = m.multiResults[1:]
		return value, nil
	}
	return m.multiResult, nil
}

func (m *mockPrompter) Confirm(message string, defaultValue bool) (bool, error) {
	return m.confirmResult, nil
}

func (m *mockPrompter) Input(message, defaultValue string) (string, error) {
	if len(m.inputResults) > 0 {
		value := m.inputResults[0]
		m.inputResults = m.inputResults[1:]
		return value, nil
	}
	return m.inputResult, nil
}

// mockOutputWriter implements OutputWriter for testing
type mockOutputWriter struct {
	output string
}

func (m *mockOutputWriter) Writef(format string, args ...any) {
	m.output += format
}

// mockFeishuPreparer implements FeishuPreparer for testing
type mockFeishuPreparer struct {
	called bool
	err    error
}

func (m *mockFeishuPreparer) EnsureReady(ctx context.Context) error {
	m.called = true
	return m.err
}

type mockConfigLoader struct {
	defaultPath string
	loadedPath  string
	savedPath   string
	loadedCfg   config.Config
	savedCfg    config.Config
}

func (m *mockConfigLoader) Load(path string) (config.Config, error) {
	m.loadedPath = path
	return m.loadedCfg, nil
}

func (m *mockConfigLoader) Save(path string, cfg config.Config) error {
	m.savedPath = path
	m.savedCfg = cfg
	return nil
}

func (m *mockConfigLoader) DefaultPath() (string, error) {
	return m.defaultPath, nil
}

func TestService_Name(t *testing.T) {
	svc := NewService()
	if svc == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestService_NoAgentsDetected(t *testing.T) {
	svc := NewService(
		WithClaudeIntegration(&mockIntegration{name: "Claude Code", detectInstalled: false}),
		WithCodexIntegration(&mockIntegration{name: "Codex", detectInstalled: false}),
		WithZcodeIntegration(&mockIntegration{name: "ZCode", detectInstalled: false}),
		WithGrokIntegration(&mockIntegration{name: "Grok", detectInstalled: false}),
	)

	prompter := &mockPrompter{}
	output := &mockOutputWriter{}

	_, err := svc.Run(context.Background(), prompter, output, "", "")
	if err == nil {
		t.Fatal("expected error when no agents detected")
	}
}

func TestService_ClaudeIntegration(t *testing.T) {
	svc := NewService(
		WithClaudeIntegration(&mockIntegration{
			name:            "Claude Code",
			detectInstalled: true,
			settingsPath:    "/tmp/.claude/settings.json",
			isHookInstalled: true,
		}),
		WithCodexIntegration(&mockIntegration{name: "Codex", detectInstalled: false}),
		WithZcodeIntegration(&mockIntegration{name: "ZCode", detectInstalled: false}),
		WithGrokIntegration(&mockIntegration{name: "Grok", detectInstalled: false}),
		WithFeishuPreparer(&mockFeishuPreparer{}),
	)

	prompter := &mockPrompter{
		selectResult: "claude",
		multiResult:  []string{"feishu", "system"},
	}
	output := &mockOutputWriter{}

	// Create a temp config path
	result, err := svc.Run(context.Background(), prompter, output, "/tmp/test-config.yaml", "/tmp/agent-notify")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Agent != "claude" {
		t.Errorf("expected agent 'claude', got %q", result.Agent)
	}
}

func TestService_CodexIntegration(t *testing.T) {
	svc := NewService(
		WithClaudeIntegration(&mockIntegration{name: "Claude Code", detectInstalled: false}),
		WithCodexIntegration(&mockIntegration{
			name:            "Codex",
			detectInstalled: true,
			settingsPath:    "/tmp/.codex/config.toml",
			isHookInstalled: true,
		}),
		WithZcodeIntegration(&mockIntegration{name: "ZCode", detectInstalled: false}),
		WithGrokIntegration(&mockIntegration{name: "Grok", detectInstalled: false}),
		WithFeishuPreparer(&mockFeishuPreparer{}),
	)

	prompter := &mockPrompter{
		selectResult: "codex",
		multiResult:  []string{"feishu", "system"},
	}
	output := &mockOutputWriter{}

	result, err := svc.Run(context.Background(), prompter, output, "/tmp/test-config.yaml", "/tmp/agent-notify")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if result.Agent != "codex" {
		t.Errorf("expected agent 'codex', got %q", result.Agent)
	}
}

func TestService_UsesInjectedConfigLoader(t *testing.T) {
	loader := &mockConfigLoader{
		defaultPath: "/tmp/injected-config.yaml",
		loadedCfg:   config.Default(),
	}
	svc := NewService(
		WithClaudeIntegration(&mockIntegration{name: "Claude Code", detectInstalled: true, settingsPath: "/tmp/.claude/settings.json"}),
		WithCodexIntegration(&mockIntegration{name: "Codex", detectInstalled: false}),
		WithZcodeIntegration(&mockIntegration{name: "ZCode", detectInstalled: false}),
		WithGrokIntegration(&mockIntegration{name: "Grok", detectInstalled: false}),
		WithConfigLoader(loader),
	)
	prompter := &mockPrompter{selectResult: "claude", multiResult: []string{"system"}}
	output := &mockOutputWriter{}

	_, err := svc.Run(context.Background(), prompter, output, "", "/tmp/agent-notify")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if loader.loadedPath != "/tmp/injected-config.yaml" {
		t.Fatalf("loadedPath = %q, want %q", loader.loadedPath, "/tmp/injected-config.yaml")
	}
	if loader.savedPath != "/tmp/injected-config.yaml" {
		t.Fatalf("savedPath = %q, want %q", loader.savedPath, "/tmp/injected-config.yaml")
	}
}

func TestDedupeStrings(t *testing.T) {
	tests := []struct {
		input    []string
		expected []string
	}{
		{[]string{"a", "b", "a", "c"}, []string{"a", "b", "c"}},
		{[]string{}, []string{}},
		{[]string{"a"}, []string{"a"}},
		{[]string{"a", "a", "a"}, []string{"a"}},
	}

	for _, tt := range tests {
		result := dedupeStrings(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("dedupeStrings(%v) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}
