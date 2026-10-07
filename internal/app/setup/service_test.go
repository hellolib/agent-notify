package setup

import (
	"context"
	"path/filepath"
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
		WithDroidIntegration(&mockIntegration{name: "Droid", detectInstalled: false}),
		WithOpenCodeIntegration(&mockIntegration{name: "OpenCode", detectInstalled: false}),
		WithOmpIntegration(&mockIntegration{name: "OMP (oh-my-pi)", detectInstalled: false}),
		WithDshIntegration(&mockIntegration{name: "DeepSeek Harness", detectInstalled: false}),
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
		WithDroidIntegration(&mockIntegration{name: "Droid", detectInstalled: false}),
		WithOpenCodeIntegration(&mockIntegration{name: "OpenCode", detectInstalled: false}),
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
		WithDroidIntegration(&mockIntegration{name: "Droid", detectInstalled: false}),
		WithOpenCodeIntegration(&mockIntegration{name: "OpenCode", detectInstalled: false}),
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
		WithDroidIntegration(&mockIntegration{name: "Droid", detectInstalled: false}),
		WithOpenCodeIntegration(&mockIntegration{name: "OpenCode", detectInstalled: false}),
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

// TestService_RecordsInstalledPath 是 issue #39 第 4 项的回归测试:
// 安装成功后必须把实际写入的配置文件绝对路径记进 config.yaml,
// 否则 project scope 装出去的 hook 换个目录就再也清理不掉。
func TestService_RecordsInstalledPath(t *testing.T) {
	loader := &mockConfigLoader{defaultPath: "/tmp/cfg.yaml", loadedCfg: config.Default()}
	svc := NewService(
		WithClaudeIntegration(&mockIntegration{
			name: "Claude Code", detectInstalled: true,
			settingsPath: filepath.Join(".claude", "settings.json"),
		}),
		WithCodexIntegration(&mockIntegration{name: "Codex", detectInstalled: false}),
		WithZcodeIntegration(&mockIntegration{name: "ZCode", detectInstalled: false}),
		WithGrokIntegration(&mockIntegration{name: "Grok", detectInstalled: false}),
		WithDroidIntegration(&mockIntegration{name: "Droid", detectInstalled: false}),
		WithOpenCodeIntegration(&mockIntegration{name: "OpenCode", detectInstalled: false}),
		WithConfigLoader(loader),
	)

	_, err := svc.Run(context.Background(), &mockPrompter{selectResult: "claude", multiResult: []string{"system"}},
		&mockOutputWriter{}, "", "/tmp/agent-notify")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	paths := loader.savedCfg.Agent.ClaudeCode.InstalledPaths
	if len(paths) != 1 {
		t.Fatalf("installed_paths = %v, want 1 entry", paths)
	}
	// 相对路径必须被转成绝对路径,否则记录换个工作目录就失效
	if !filepath.IsAbs(paths[0]) {
		t.Fatalf("记录的是相对路径 %q,换个目录执行 clean 就指不到它", paths[0])
	}
	want, _ := filepath.Abs(filepath.Join(".claude", "settings.json"))
	if paths[0] != want {
		t.Fatalf("installed_paths[0] = %q, want %q", paths[0], want)
	}
}

// TestService_DoesNotDuplicateInstalledPathOnReinstall 重复跑向导不该堆积重复记录。
func TestService_DoesNotDuplicateInstalledPathOnReinstall(t *testing.T) {
	existing, _ := filepath.Abs(filepath.Join(".claude", "settings.json"))
	cfg := config.Default()
	cfg.Agent.ClaudeCode.InstalledPaths = []string{existing}

	loader := &mockConfigLoader{defaultPath: "/tmp/cfg.yaml", loadedCfg: cfg}
	svc := NewService(
		WithClaudeIntegration(&mockIntegration{
			name: "Claude Code", detectInstalled: true,
			settingsPath: filepath.Join(".claude", "settings.json"),
		}),
		WithCodexIntegration(&mockIntegration{name: "Codex", detectInstalled: false}),
		WithZcodeIntegration(&mockIntegration{name: "ZCode", detectInstalled: false}),
		WithGrokIntegration(&mockIntegration{name: "Grok", detectInstalled: false}),
		WithDroidIntegration(&mockIntegration{name: "Droid", detectInstalled: false}),
		WithOpenCodeIntegration(&mockIntegration{name: "OpenCode", detectInstalled: false}),
		WithConfigLoader(loader),
	)

	if _, err := svc.Run(context.Background(), &mockPrompter{selectResult: "claude", multiResult: []string{"system"}},
		&mockOutputWriter{}, "", "/tmp/agent-notify"); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got := loader.savedCfg.Agent.ClaudeCode.InstalledPaths; len(got) != 1 {
		t.Fatalf("重复安装后 installed_paths = %v, want 1 entry", got)
	}
}

// TestService_DSHIntegration 走完整向导，断言：agent 选项出现 DeepSeek Harness、
// 四个通知事件都可选、选择结果落到 cfg.Notify.DSH、agent 被标记启用、
// 且安装落点被记入 InstalledPaths（clean 依赖它才能可靠清理）。
func TestService_DSHIntegration(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")

	dsh := &mockIntegration{name: "DeepSeek Harness", detectInstalled: true,
		settingsPath: filepath.Join(dir, "profiles", "web", "package.json")}

	svc := NewService(
		WithDshIntegration(dsh),
		WithClaudeIntegration(&mockIntegration{name: "Claude Code", detectInstalled: false}),
		WithCodexIntegration(&mockIntegration{name: "Codex", detectInstalled: false}),
		WithZcodeIntegration(&mockIntegration{name: "ZCode", detectInstalled: false}),
		WithGrokIntegration(&mockIntegration{name: "Grok", detectInstalled: false}),
		WithDroidIntegration(&mockIntegration{name: "Droid", detectInstalled: false}),
		WithOpenCodeIntegration(&mockIntegration{name: "OpenCode", detectInstalled: false}),
		WithOmpIntegration(&mockIntegration{name: "OMP (oh-my-pi)", detectInstalled: false}),
	)

	prompter := &mockPrompter{
		selectResult: "dsh",
		multiResults: [][]string{{"system"}, {"permission_required", "input_required"}},
	}
	output := &mockOutputWriter{}

	result, err := svc.Run(context.Background(), prompter, output, configPath, "/tmp/agent-notify")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Agent != "dsh" {
		t.Fatalf("result.Agent = %q, want dsh", result.Agent)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Agent.DSH.Enabled {
		t.Error("cfg.Agent.DSH.Enabled = false, want true after a successful setup")
	}
	if len(cfg.Notify.DSH.Events) != 2 {
		t.Errorf("cfg.Notify.DSH.Events = %v, want the two selected events", cfg.Notify.DSH.Events)
	}
	if len(cfg.Agent.DSH.InstalledPaths) == 0 {
		t.Error("InstalledPaths is empty; clean could not locate the profile")
	}
	// DSH has no project scope, so the recorded scope must always be user.
	if cfg.Agent.DSH.InstallScope != "user" {
		t.Errorf("InstallScope = %q, want user", cfg.Agent.DSH.InstallScope)
	}
}

// TestDSHEventOptionsCoverAllFour 断言向导把四个事件都摆出来。
// 若照抄 Codex 的两事件默认值，permission_required 就选不到——
// 而那正是这个接入存在的理由。
func TestDSHEventOptionsCoverAllFour(t *testing.T) {
	options := dshEventOptionsFn()
	if len(options) != 4 {
		t.Fatalf("dshEventOptionsFn() returned %d options, want 4", len(options))
	}
	seen := map[string]bool{}
	for _, o := range options {
		seen[o.Value] = true
	}
	for _, want := range []string{"permission_required", "input_required", "run_completed", "run_failed"} {
		if !seen[want] {
			t.Errorf("dshEventOptionsFn() is missing %q", want)
		}
	}
}
