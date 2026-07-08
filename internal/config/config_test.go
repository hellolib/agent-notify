package config

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultConfigUsesAgentScopedNotifyConfig(t *testing.T) {
	cfg := Default()
	allEvents := []string{"permission_required", "input_required", "run_completed", "run_failed"}

	if cfg.Version != 1 {
		t.Fatalf("Version = %d, want 1", cfg.Version)
	}
	if !cfg.Agent.ClaudeCode.Enabled {
		t.Fatal("Claude Code should be enabled by default")
	}
	if cfg.Agent.Codex.Enabled {
		t.Fatal("Codex should be disabled by default")
	}
	if !cfg.Notify.ClaudeCode.Channels.System.Enabled {
		t.Fatal("Claude Code system notification should be enabled by default")
	}
	if !reflect.DeepEqual(cfg.Notify.ClaudeCode.Events, allEvents) {
		t.Fatalf("Claude Code events = %#v, want %#v", cfg.Notify.ClaudeCode.Events, allEvents)
	}
	if cfg.Notify.ClaudeCode.Channels.Feishu.Enabled {
		t.Fatal("Claude Code feishu should be disabled by default")
	}
	if cfg.Notify.ClaudeCode.Channels.Bark.Enabled {
		t.Fatal("Claude Code bark should be disabled by default")
	}
	if cfg.Notify.Codex.Channels.System.Enabled {
		t.Fatal("Codex system notification should be disabled by default")
	}
	if cfg.Notify.Codex.Channels.Feishu.Enabled {
		t.Fatal("Codex feishu should be disabled by default")
	}
	if cfg.Notify.Codex.Channels.Bark.Enabled {
		t.Fatal("Codex bark should be disabled by default")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	want := Default()
	want.Notify.ClaudeCode.Channels.Feishu.Enabled = true
	want.Notify.ClaudeCode.Events = []string{"permission_required", "run_completed"}
	want.Notify.Codex.Channels.System.Enabled = true
	want.Notify.Codex.Channels.Feishu.Enabled = true
	want.Notify.Codex.Channels.Bark.Enabled = true
	want.Notify.Codex.Channels.Bark.WebhookURL = "https://api.day.app/key"

	if err := Save(path, want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	notifyMap, ok := raw["notify"].(map[string]any)
	if !ok {
		t.Fatalf("notify = %T, want map[string]any", raw["notify"])
	}
	claudeMap, ok := notifyMap["claude_code"].(map[string]any)
	if !ok {
		t.Fatalf("notify.claude_code = %T, want map[string]any", notifyMap["claude_code"])
	}
	if _, exists := claudeMap["channels"]; !exists {
		t.Fatalf("saved config missing notify.claude_code.channels, got %#v", claudeMap)
	}
	if _, exists := claudeMap["events"]; !exists {
		t.Fatalf("saved config missing notify.claude_code.events, got %#v", claudeMap)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() mismatch\ngot  %#v\nwant %#v", got, want)
	}
}

func TestSaveUsesPrivatePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not enforce POSIX permission bits")
	}
	dir := filepath.Join(t.TempDir(), "agent-notify")
	path := filepath.Join(dir, "config.yaml")

	if err := Save(path, Default()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(config) error = %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("config mode = %o, want 600", got)
	}

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("Stat(dir) error = %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("config dir mode = %o, want 700", got)
	}
}

func TestLoadNewConfigStructure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	configYAML := []byte(`version: 1
agent:
  claude_code:
    enabled: true
    install_scope: user
  codex:
    enabled: false
    install_scope: user
notify:
  claude_code:
    events:
      - permission_required
      - run_completed
    channels:
      feishu:
        enabled: true
      system:
        enabled: true
  codex:
    events: []
    channels:
      feishu:
        enabled: false
      system:
        enabled: false
behavior:
  dedupe_seconds: 60
  send_timeout_seconds: 5
  locale: zh-CN
`)
	if err := os.WriteFile(path, configYAML, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !got.Notify.ClaudeCode.Channels.System.Enabled {
		t.Fatal("Claude Code system should be enabled")
	}
	if !got.Notify.ClaudeCode.Channels.Feishu.Enabled {
		t.Fatal("Claude Code feishu should be enabled")
	}
	if !reflect.DeepEqual(got.Notify.ClaudeCode.Events, []string{"permission_required", "run_completed"}) {
		t.Fatalf("Claude Code events = %#v, want %#v", got.Notify.ClaudeCode.Events, []string{"permission_required", "run_completed"})
	}
}

func TestLoadMissingFileReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.yaml")

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !reflect.DeepEqual(got, Default()) {
		t.Fatalf("Load() mismatch\ngot  %#v\nwant %#v", got, Default())
	}
}

func TestXiaoduBehaviorDefaults(t *testing.T) {
	cfg := Default()
	x := cfg.Notify.Codex.Channels.Xiaodu

	if !x.ShouldSpeakCompleted() {
		t.Fatal("ShouldSpeakCompleted() = false, want true by default")
	}
	if got := x.EffectiveRepeatCount(); got != 2 {
		t.Fatalf("EffectiveRepeatCount() = %d, want 2", got)
	}
	if got := x.EffectiveRepeatIntervalSeconds(); got != 25 {
		t.Fatalf("EffectiveRepeatIntervalSeconds() = %d, want 25", got)
	}
}

func TestXiaoduSpeakCompletedCanBeDisabled(t *testing.T) {
	disabled := false
	x := XiaoduChannelConfig{SpeakCompleted: &disabled}

	if x.ShouldSpeakCompleted() {
		t.Fatal("ShouldSpeakCompleted() = true, want false when explicitly disabled")
	}
}
