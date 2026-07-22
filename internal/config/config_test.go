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

func TestSaveUsesOwnerOnlyPermissions(t *testing.T) {
	dir := t.TempDir()
	// Parent is a temp dir (usually 0700); Save still creates config dir if needed.
	cfgDir := filepath.Join(dir, ".agent-notify")
	path := filepath.Join(cfgDir, "config.yaml")

	if err := Save(path, Default()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(config) error = %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config mode = %04o, want 0600", perm)
	}

	dirInfo, err := os.Stat(cfgDir)
	if err != nil {
		t.Fatalf("Stat(dir) error = %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Fatalf("config dir mode = %04o, want 0700", perm)
	}

	// Existing world-readable file must be tightened on re-save.
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("Chmod(0644) error = %v", err)
	}
	if err := Save(path, Default()); err != nil {
		t.Fatalf("re-Save() error = %v", err)
	}
	info, err = os.Stat(path)
	if err != nil {
		t.Fatalf("Stat after re-Save error = %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config mode after re-Save = %04o, want 0600", perm)
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

func TestFocusPrecisionFromEnv(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "app"}, // unset -> app
		{"window", "window"},
		{"WINDOW", "window"}, // case-insensitive
		{"app", "app"},
		{"garbage", "app"},
	}
	for _, c := range cases {
		t.Setenv("AGENT_NOTIFY_FOCUS_PRECISION", c.in)
		if got := FocusPrecisionFromEnv(); got != c.want {
			t.Fatalf("FocusPrecisionFromEnv() with %q = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEffectiveFocusDebug(t *testing.T) {
	t.Setenv("AGENT_NOTIFY_FOCUS_DEBUG", "")
	if (SystemChannelConfig{FocusDebug: false}).EffectiveFocusDebug() {
		t.Fatal("default should be false")
	}
	if !(SystemChannelConfig{FocusDebug: true}).EffectiveFocusDebug() {
		t.Fatal("config true should enable")
	}
	for _, v := range []string{"1", "true", "yes", "TRUE"} {
		t.Setenv("AGENT_NOTIFY_FOCUS_DEBUG", v)
		if !(SystemChannelConfig{FocusDebug: false}).EffectiveFocusDebug() {
			t.Fatalf("env %q should force-enable", v)
		}
	}
	t.Setenv("AGENT_NOTIFY_FOCUS_DEBUG", "0")
	if (SystemChannelConfig{FocusDebug: false}).EffectiveFocusDebug() {
		t.Fatal("env 0 should not enable")
	}
}

func TestDefaultConfigDedupeWindowIsTenSeconds(t *testing.T) {
	if got := Default().Behavior.DedupeSeconds; got != 10 {
		t.Fatalf("default DedupeSeconds = %d, want 10", got)
	}
}
