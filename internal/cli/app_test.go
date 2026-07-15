package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/testutil"
)

type fakePrompter struct {
	selects []string
	multi   [][]string
	confirm []bool
	inputs  []string
	stdout  *bytes.Buffer
}

func (f *fakePrompter) Select(message string, _ []PromptOption, _ string) (string, error) {
	if f.stdout != nil && message != "" {
		f.stdout.WriteString(message + "\n")
	}
	if len(f.selects) == 0 {
		return "", nil
	}
	value := f.selects[0]
	f.selects = f.selects[1:]
	return value, nil
}

func (f *fakePrompter) MultiSelect(message string, _ []PromptOption, _ []string) ([]string, error) {
	if f.stdout != nil && message != "" {
		f.stdout.WriteString(message + "\n")
	}
	if len(f.multi) == 0 {
		return nil, nil
	}
	value := f.multi[0]
	f.multi = f.multi[1:]
	return value, nil
}

func (f *fakePrompter) Confirm(_ string, _ bool) (bool, error) {
	if len(f.confirm) == 0 {
		return false, nil
	}
	value := f.confirm[0]
	f.confirm = f.confirm[1:]
	return value, nil
}

func (f *fakePrompter) Input(_ string, defaultValue string) (string, error) {
	if len(f.inputs) == 0 {
		return defaultValue, nil
	}
	value := f.inputs[0]
	f.inputs = f.inputs[1:]
	return value, nil
}

func useFakePrompter(t *testing.T, prompter *fakePrompter) {
	t.Helper()

	oldFactory := newPrompter
	newPrompter = func(streams Streams) (Prompter, error) {
		return prompter, nil
	}
	t.Cleanup(func() {
		newPrompter = oldFactory
	})
}

func TestRunRootHelp(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"--help"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, want := range []string{"init", "claude", "test", "doctor"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
}

func TestRunWithoutArgsShowsMenuAndExits(t *testing.T) {
	useFakePrompter(t, &fakePrompter{
		selects: []string{"quit"},
	})

	var stdout bytes.Buffer
	err := Run(context.Background(), nil, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	// 检查 banner 中的关键字
	for _, want := range []string{"Agent Notify", "Claude Code"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
}

func TestRunWithoutArgsShowsNotificationTestSubmenu(t *testing.T) {
	var stdout bytes.Buffer
	useFakePrompter(t, &fakePrompter{
		selects: []string{"test", "back", "quit"},
		stdout:  &stdout,
	})

	err := Run(context.Background(), nil, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, want := range []string{"测试通知"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
}

func TestRunClaudeHelp(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"claude", "--help"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, want := range []string{"print-hooks", "install-hooks"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
}

func TestRunClaudePrintHooksHelp(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"claude", "print-hooks", "--help"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "--binary") {
		t.Fatalf("stdout = %q, want --binary flag", stdout.String())
	}
}

func TestRunVersionFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run(context.Background(), []string{"--version"}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got := strings.TrimSpace(stdout.String())
	if !strings.Contains(got, Version) {
		t.Fatalf("stdout = %q, want substring %q", got, Version)
	}

	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunInitWritesConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	settingsPath := filepath.Join(dir, "settings.json")
	calledPrepare := false

	oldPrepare := prepareFeishuCLI
	prepareFeishuCLI = func(ctx context.Context) error {
		calledPrepare = true
		return nil
	}
	defer func() {
		prepareFeishuCLI = oldPrepare
	}()

	// TDD: Single-select for agent, multi-select channels (default all), multi-select events (default all)
	useFakePrompter(t, &fakePrompter{
		selects: []string{"claude"}, // 1. Select agent (single select)
		multi: [][]string{
			{"feishu", "system"}, // 2. Select channels (default all)
			{"permission_required", "input_required", "run_completed", "run_failed"}, // 3. Select events (default all 4 for Claude)
		},
		inputs: []string{"/tmp/agent-notify"},
	})

	var stdout bytes.Buffer
	err := Run(
		context.Background(),
		[]string{"init", "--config", configPath, "--settings", settingsPath},
		strings.NewReader(""),
		&stdout,
		&bytes.Buffer{},
	)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "配置文件:") {
		t.Fatalf("stdout = %q, want config path message", stdout.String())
	}
	if !calledPrepare {
		t.Fatal("prepareFeishuCLI was not called")
	}

	got, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// Verify ClaudeCode notify config has both channels enabled with all events
	if !got.Notify.ClaudeCode.Channels.Feishu.Enabled || !got.Notify.ClaudeCode.Channels.System.Enabled {
		t.Fatalf("got %+v, want both channels enabled for ClaudeCode", got.Notify.ClaudeCode)
	}
	expectedEvents := []string{"permission_required", "input_required", "run_completed", "run_failed"}
	if !reflect.DeepEqual(got.Notify.ClaudeCode.Events, expectedEvents) {
		t.Fatalf("ClaudeCode feishu events = %#v, want %#v", got.Notify.ClaudeCode.Events, expectedEvents)
	}
	if !reflect.DeepEqual(got.Notify.ClaudeCode.Events, expectedEvents) {
		t.Fatalf("ClaudeCode system events = %#v, want %#v", got.Notify.ClaudeCode.Events, expectedEvents)
	}
}

func TestRunTestFeishuWithoutConfig(t *testing.T) {
	testutil.IsolateHome(t)

	// Test Feishu intentionally ignores channel enabled flags and always tries to send.
	// Mock the preparer so tests never launch interactive Feishu CLI login.
	oldPrepare := prepareFeishuCLI
	prepareFeishuCLI = func(ctx context.Context) error {
		return fmt.Errorf("feishu CLI not ready")
	}
	defer func() {
		prepareFeishuCLI = oldPrepare
	}()

	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"test", "feishu"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err == nil {
		t.Fatal("Run() error = nil, want preparer error")
	}
	if !strings.Contains(err.Error(), "feishu CLI not ready") {
		t.Fatalf("err = %v, want preparer error", err)
	}
}

func TestRunPrintHooks(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"claude", "print-hooks", "--binary", "/tmp/agent-notify"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(stdout.String(), "PermissionRequest") {
		t.Fatalf("stdout = %q, want PermissionRequest", stdout.String())
	}
	if !strings.Contains(stdout.String(), "/tmp/agent-notify handle-claude-hook") {
		t.Fatalf("stdout = %q, want binary command", stdout.String())
	}
}

func TestRunDoctorWithoutConfig(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"doctor"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	output := stdout.String()
	for _, want := range []string{"Claude Code", "Codex", "飞书", "系统", "配置文件"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunDoctorDetectsCodexHookConfig(t *testing.T) {
	dir := testutil.IsolateHome(t)
	if err := os.MkdirAll(filepath.Join(dir, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	hooksPath := filepath.Join(dir, ".codex", "hooks.json")
	hooksJSON := `{
  "hooks": {
    "PermissionRequest": [
      {
        "hooks": [
          {"type": "command", "command": "/tmp/agent-notify handle-codex-hook"}
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {"type": "command", "command": "/tmp/agent-notify handle-codex-hook"}
        ]
      }
    ]
  }
}`
	if err := os.WriteFile(hooksPath, []byte(hooksJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"doctor"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "Codex") {
		t.Fatalf("stdout = %q, want Codex", output)
	}
}

func TestRunInitCanDisableSystemNotification(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	settingsPath := filepath.Join(dir, "settings.json")

	useFakePrompter(t, &fakePrompter{
		selects: []string{"claude"}, // 1. Select agent (single select)
		multi: [][]string{
			{"feishu"}, // 2. Select channels (only feishu, no system)
			{"permission_required", "input_required"}, // 3. Select events (only 2 of 4)
		},
		inputs: []string{"/tmp/agent-notify"},
	})

	oldPrepare := prepareFeishuCLI
	prepareFeishuCLI = func(ctx context.Context) error { return nil }
	defer func() {
		prepareFeishuCLI = oldPrepare
	}()

	if err := Run(context.Background(), []string{"init", "--config", configPath, "--settings", settingsPath}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Notify.ClaudeCode.Channels.System.Enabled {
		t.Fatalf("ClaudeCode system enabled = true, want false")
	}
	// 验证只选择了 2 个事件
	expectedEvents := []string{"permission_required", "input_required"}
	if !reflect.DeepEqual(got.Notify.ClaudeCode.Events, expectedEvents) {
		t.Fatalf("ClaudeCode events = %#v, want %#v", got.Notify.ClaudeCode.Events, expectedEvents)
	}
}

func TestRunInitPartialEventsSelection(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	settingsPath := filepath.Join(dir, "settings.json")

	// 只选择 2 个事件（permission_required 和 run_completed）
	useFakePrompter(t, &fakePrompter{
		selects: []string{"claude"},
		multi: [][]string{
			{"feishu", "system"},
			{"permission_required", "run_completed"}, // 只选 2 个事件
		},
		inputs: []string{"/tmp/agent-notify"},
	})

	oldPrepare := prepareFeishuCLI
	prepareFeishuCLI = func(ctx context.Context) error { return nil }
	defer func() {
		prepareFeishuCLI = oldPrepare
	}()

	if err := Run(context.Background(), []string{"init", "--config", configPath, "--settings", settingsPath}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// 验证只选择了 2 个事件
	expectedEvents := []string{"permission_required", "run_completed"}
	if !reflect.DeepEqual(got.Notify.ClaudeCode.Events, expectedEvents) {
		t.Fatalf("ClaudeCode events = %#v, want %#v", got.Notify.ClaudeCode.Events, expectedEvents)
	}
}

func TestRunInitInstallsCodexHookConfig(t *testing.T) {
	dir := testutil.IsolateHome(t)
	configPath := filepath.Join(dir, "config.yaml")

	// Mock prepareFeishuCLI to avoid actual feishu CLI interaction
	oldPrepare := prepareFeishuCLI
	prepareFeishuCLI = func(ctx context.Context) error { return nil }
	defer func() {
		prepareFeishuCLI = oldPrepare
	}()

	// Codex init: agent → channels → events (只有 2 个事件可选)
	useFakePrompter(t, &fakePrompter{
		selects: []string{"codex"},
		multi: [][]string{
			{"feishu", "system"},
			{"permission_required", "run_completed"},
		},
		inputs: []string{"/tmp/agent-notify"},
	})

	var stdout bytes.Buffer
	if err := Run(context.Background(), []string{"init", "--config", configPath, "--binary", "/tmp/agent-notify"}, strings.NewReader(""), &stdout, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	hooksPath := filepath.Join(dir, ".codex", "hooks.json")
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(data)
	for _, want := range []string{`"PermissionRequest"`, `"Stop"`, "handle-codex-hook"} {
		if !strings.Contains(got, want) {
			t.Fatalf("hooks.json = %q, want substring %q", got, want)
		}
	}
	if !strings.Contains(stdout.String(), hooksPath) {
		t.Fatalf("stdout = %q, want hooks.json path", stdout.String())
	}

	// Verify Codex notify config has both channels enabled with 2 events
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Notify.Codex.Channels.Feishu.Enabled || !cfg.Notify.Codex.Channels.System.Enabled {
		t.Fatalf("got %+v, want both channels enabled for Codex", cfg.Notify.Codex)
	}
	expectedEvents := []string{"permission_required", "run_completed"}
	if !reflect.DeepEqual(cfg.Notify.Codex.Events, expectedEvents) {
		t.Fatalf("Codex events = %#v, want %#v", cfg.Notify.Codex.Events, expectedEvents)
	}
}

// TestRunInitCodexDoesNotOverwriteClaudeCodeConfig verifies that initializing Codex
// does not overwrite Claude Code's existing notify config
func TestRunInitCodexDoesNotOverwriteClaudeCodeConfig(t *testing.T) {
	dir := testutil.IsolateHome(t)
	configPath := filepath.Join(dir, "config.yaml")

	// Mock prepareFeishuCLI to avoid actual feishu CLI interaction
	oldPrepare := prepareFeishuCLI
	prepareFeishuCLI = func(ctx context.Context) error { return nil }
	defer func() {
		prepareFeishuCLI = oldPrepare
	}()

	// First, initialize Claude Code with specific config
	useFakePrompter(t, &fakePrompter{
		selects: []string{"claude"},
		multi: [][]string{
			{"system"},                               // Only system, no feishu
			{"permission_required", "run_completed"}, // Only 2 events
		},
		inputs: []string{"/tmp/agent-notify"},
	})

	if err := Run(context.Background(), []string{"init", "--config", configPath, "--binary", "/tmp/agent-notify"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("First Run() error = %v", err)
	}

	// Verify Claude Code config
	got, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Notify.ClaudeCode.Channels.Feishu.Enabled {
		t.Fatal("ClaudeCode feishu should be disabled")
	}
	if !got.Notify.ClaudeCode.Channels.System.Enabled {
		t.Fatal("ClaudeCode system should be enabled")
	}
	if !reflect.DeepEqual(got.Notify.ClaudeCode.Events, []string{"permission_required", "run_completed"}) {
		t.Fatalf("ClaudeCode system events = %#v", got.Notify.ClaudeCode.Events)
	}

	// Now initialize Codex with different config
	useFakePrompter(t, &fakePrompter{
		selects: []string{"codex"},
		multi: [][]string{
			{"feishu"}, // Only feishu, no system
			{"permission_required", "run_completed"},
		},
		inputs: []string{"/tmp/agent-notify"},
	})

	if err := Run(context.Background(), []string{"init", "--config", configPath, "--binary", "/tmp/agent-notify"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Second Run() error = %v", err)
	}

	// Verify Claude Code config is preserved
	got, err = config.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// Claude Code config should be unchanged
	if got.Notify.ClaudeCode.Channels.Feishu.Enabled {
		t.Fatal("ClaudeCode feishu should still be disabled after Codex init")
	}
	if !got.Notify.ClaudeCode.Channels.System.Enabled {
		t.Fatal("ClaudeCode system should still be enabled after Codex init")
	}
	if !reflect.DeepEqual(got.Notify.ClaudeCode.Events, []string{"permission_required", "run_completed"}) {
		t.Fatalf("ClaudeCode system events should be unchanged = %#v", got.Notify.ClaudeCode.Events)
	}
	// Codex config should have only feishu enabled
	if !got.Notify.Codex.Channels.Feishu.Enabled {
		t.Fatal("Codex feishu should be enabled")
	}
	if got.Notify.Codex.Channels.System.Enabled {
		t.Fatal("Codex system should be disabled")
	}
}

// TestRunInitEditSameAgentConfig verifies that re-configuring the same agent
// correctly updates the config (editing scenario)
func TestRunInitEditSameAgentConfig(t *testing.T) {
	dir := testutil.IsolateHome(t)
	configPath := filepath.Join(dir, "config.yaml")

	// Mock prepareFeishuCLI to avoid actual feishu CLI interaction
	oldPrepare := prepareFeishuCLI
	prepareFeishuCLI = func(ctx context.Context) error { return nil }
	defer func() {
		prepareFeishuCLI = oldPrepare
	}()

	// First, initialize Claude Code with both channels and all events
	useFakePrompter(t, &fakePrompter{
		selects: []string{"claude"},
		multi: [][]string{
			{"feishu", "system"}, // Both channels
			{"permission_required", "input_required", "run_completed", "run_failed"}, // All events
		},
		inputs: []string{"/tmp/agent-notify"},
	})

	if err := Run(context.Background(), []string{"init", "--config", configPath, "--binary", "/tmp/agent-notify"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("First Run() error = %v", err)
	}

	// Verify initial config
	got, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !got.Notify.ClaudeCode.Channels.Feishu.Enabled {
		t.Fatal("ClaudeCode feishu should be enabled after first init")
	}
	if !got.Notify.ClaudeCode.Channels.System.Enabled {
		t.Fatal("ClaudeCode system should be enabled after first init")
	}
	if len(got.Notify.ClaudeCode.Events) != 4 {
		t.Fatalf("ClaudeCode events = %d, want 4", len(got.Notify.ClaudeCode.Events))
	}

	// Now re-configure: disable system, select fewer events
	useFakePrompter(t, &fakePrompter{
		selects: []string{"claude"},
		multi: [][]string{
			{"feishu"},                               // Only feishu (deselect system)
			{"permission_required", "run_completed"}, // Only 2 events (deselect others)
		},
		inputs: []string{"/tmp/agent-notify"},
	})

	if err := Run(context.Background(), []string{"init", "--config", configPath, "--binary", "/tmp/agent-notify"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Second Run() error = %v", err)
	}

	// Verify edited config - this is where the bug should show up
	got, err = config.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// System should now be disabled
	if got.Notify.ClaudeCode.Channels.System.Enabled {
		t.Fatal("ClaudeCode system should be disabled after editing (deselecting)")
	}
	// Feishu should still be enabled
	if !got.Notify.ClaudeCode.Channels.Feishu.Enabled {
		t.Fatal("ClaudeCode feishu should still be enabled after editing")
	}
	// Events should be exactly 2
	expectedEvents := []string{"permission_required", "run_completed"}
	if !reflect.DeepEqual(got.Notify.ClaudeCode.Events, expectedEvents) {
		t.Fatalf("ClaudeCode events = %#v, want %#v", got.Notify.ClaudeCode.Events, expectedEvents)
	}
}

// TestRunInitClaudeCodeDoesNotOverwriteCodexConfig verifies that initializing Claude Code
// does not overwrite Codex's existing notify config
func TestRunInitClaudeCodeDoesNotOverwriteCodexConfig(t *testing.T) {
	dir := testutil.IsolateHome(t)
	configPath := filepath.Join(dir, "config.yaml")

	// Mock prepareFeishuCLI to avoid actual feishu CLI interaction
	oldPrepare := prepareFeishuCLI
	prepareFeishuCLI = func(ctx context.Context) error { return nil }
	defer func() {
		prepareFeishuCLI = oldPrepare
	}()

	// First, initialize Codex with specific config
	useFakePrompter(t, &fakePrompter{
		selects: []string{"codex"},
		multi: [][]string{
			{"system"}, // Only system, no feishu
			{"permission_required", "run_completed"},
		},
		inputs: []string{"/tmp/agent-notify"},
	})

	if err := Run(context.Background(), []string{"init", "--config", configPath, "--binary", "/tmp/agent-notify"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("First Run() error = %v", err)
	}

	// Verify Codex config
	got, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Notify.Codex.Channels.Feishu.Enabled {
		t.Fatal("Codex feishu should be disabled")
	}
	if !got.Notify.Codex.Channels.System.Enabled {
		t.Fatal("Codex system should be enabled")
	}

	// Now initialize Claude Code with different config
	useFakePrompter(t, &fakePrompter{
		selects: []string{"claude"},
		multi: [][]string{
			{"feishu", "system"},
			{"input_required", "run_failed"},
		},
		inputs: []string{"/tmp/agent-notify"},
	})

	if err := Run(context.Background(), []string{"init", "--config", configPath, "--binary", "/tmp/agent-notify"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Second Run() error = %v", err)
	}

	// Verify Codex config is preserved
	got, err = config.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	// Codex config should be unchanged
	if got.Notify.Codex.Channels.Feishu.Enabled {
		t.Fatal("Codex feishu should still be disabled after ClaudeCode init")
	}
	if !got.Notify.Codex.Channels.System.Enabled {
		t.Fatal("Codex system should still be enabled after ClaudeCode init")
	}
	// Claude Code config should have both channels enabled
	if !got.Notify.ClaudeCode.Channels.Feishu.Enabled || !got.Notify.ClaudeCode.Channels.System.Enabled {
		t.Fatal("ClaudeCode both channels should be enabled")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
