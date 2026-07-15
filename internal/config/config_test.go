package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultConfigUsesAgentScopedNotifyConfig(t *testing.T) {
	cfg := Default()
	allEvents := []string{"permission_required", "input_required", "run_completed", "run_failed"}

	if cfg.Version != 1 {
		t.Fatalf("Version = %d, want 1", cfg.Version)
	}
	// No agent or channel is pre-enabled: first-time view config stays clean
	// until the user explicitly configures an agent / channel.
	if cfg.Agent.ClaudeCode.Enabled {
		t.Fatal("Claude Code should be disabled by default until setup")
	}
	if cfg.Agent.Codex.Enabled {
		t.Fatal("Codex should be disabled by default")
	}
	if cfg.Agent.Grok.Enabled {
		t.Fatal("Grok should be disabled by default")
	}
	if cfg.Notify.ClaudeCode.Channels.System.Enabled {
		t.Fatal("Claude Code system notification should be disabled by default")
	}
	if cfg.Notify.ZCode.Channels.System.Enabled {
		t.Fatal("ZCode system notification should be disabled by default")
	}
	if cfg.Notify.Grok.Channels.System.Enabled {
		t.Fatal("Grok system notification should be disabled by default")
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

func TestLoadBackfillsEventsWhenChannelsEnabledWithoutEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	// Mimic channel-menu-only setup: wechat enabled, events omitted from YAML.
	configYAML := []byte(`version: 1
agent:
  claude_code:
    enabled: true
    install_scope: user
  grok:
    enabled: true
    install_scope: user
notify:
  claude_code:
    channels:
      wechat:
        enabled: true
        webhook_url: https://push.example.com/api/notify/x
  grok:
    channels:
      wechat:
        enabled: true
        webhook_url: https://push.example.com/api/notify/x
      bark:
        enabled: true
        webhook_url: https://api.day.app/key
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

	if !got.Notify.ClaudeCode.Channels.Wechat.Enabled {
		t.Fatal("wechat should remain enabled")
	}
	if len(got.Notify.ClaudeCode.Events) == 0 {
		t.Fatal("ClaudeCode events should be backfilled when channels are enabled")
	}
	if len(got.Notify.Grok.Events) == 0 {
		t.Fatal("Grok events should be backfilled when channels are enabled")
	}
	// Bark must not replace wechat during load.
	if !got.Notify.Grok.Channels.Wechat.Enabled {
		t.Fatal("Grok wechat must not be lost when bark is also enabled")
	}
	if !got.Notify.Grok.Channels.Bark.Enabled {
		t.Fatal("Grok bark should remain enabled alongside wechat")
	}
}
