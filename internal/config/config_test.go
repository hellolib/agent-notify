package config

import (
	"fmt"
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

	if cfg.Version != currentConfigVersion {
		t.Fatalf("Version = %d, want %d", cfg.Version, currentConfigVersion)
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
	// Windows 无 POSIX 权限位:os.Stat 对可写文件恒返回 0666,Chmod 只能切换
	// 只读位,0600/0700 断言在 Windows 上永远不成立(issue #20 的权限收紧
	// 本身就只对 Unix 有意义)。
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not applicable on Windows")
	}
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

func TestStarPromptedRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")

	cfg := Default()
	cfg.StarPrompted = true
	if err := Save(p, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !loaded.StarPrompted {
		t.Fatal("StarPrompted did not round-trip through Save/Load")
	}
}

func TestStarPromptedDefaultsFalseForOldConfig(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(p, []byte("version: 1\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	loaded, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.StarPrompted {
		t.Fatal("expected StarPrompted=false for config without the key")
	}
}

func TestRecordInstalledPathStoresAbsolutePathAndDedupes(t *testing.T) {
	// project scope 传进来的是相对路径,必须转成绝对路径才能在别的目录清理
	got := RecordInstalledPath(nil, filepath.Join(".claude", "settings.json"))
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if !filepath.IsAbs(got[0]) {
		t.Fatalf("path %q 不是绝对路径", got[0])
	}

	// 同一路径重复安装不应产生重复记录
	again := RecordInstalledPath(got, filepath.Join(".claude", "settings.json"))
	if len(again) != 1 {
		t.Fatalf("重复安装后 len = %d, want 1: %v", len(again), again)
	}

	// 不同项目分别安装应各记一条
	other := RecordInstalledPath(again, filepath.Join(t.TempDir(), ".claude", "settings.json"))
	if len(other) != 2 {
		t.Fatalf("第二个项目未被记录: %v", other)
	}
}

func TestInstalledPathsSurviveSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := Default()
	cfg.Agent.ClaudeCode.InstalledPaths = []string{"/a/.claude/settings.json", "/b/.claude/settings.json"}

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	got := loaded.Agent.ClaudeCode.InstalledPaths
	if len(got) != 2 || got[0] != "/a/.claude/settings.json" || got[1] != "/b/.claude/settings.json" {
		t.Fatalf("installed_paths 未往返: %v", got)
	}
}

// TestNotifyConfigAllCoversEveryAgent 让「新增 agent 忘了同步 All()」在测试期就暴露:
// 只要往 NotifyConfig 加字段而不更新 All(),此用例立即失败。
func TestNotifyConfigAllCoversEveryAgent(t *testing.T) {
	got := len(NotifyConfig{}.All())
	want := reflect.TypeOf(NotifyConfig{}).NumField()
	if got != want {
		t.Fatalf("NotifyConfig.All() 返回 %d 个 agent，但 NotifyConfig 有 %d 个字段；新增 agent 后请同步 All()", got, want)
	}
}

// 回归：OpenCode 支持上线前写下的老配置（无 opencode 段）加载后，
// 各字段必须拿到 Default() 的值，而不是 Go 零值。
// 旧实现把 YAML 解析进零值结构体再手工补字段，补漏的 click_to_focus
// 就变成了 false——OpenCode 用户因此点不动通知。
func TestLoadFillsDefaultsForAgentAddedAfterConfigWasWritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	legacy := `version: 1
agent:
    claude_code:
        enabled: true
        install_scope: user
notify:
    claude_code:
        events:
            - permission_required
        channels:
            system:
                enabled: true
                click_to_focus: true
`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	def := Default()
	if !cfg.Notify.OpenCode.Channels.System.ClickToFocus {
		t.Error("opencode click_to_focus = false, want true (default)")
	}
	if cfg.Agent.OpenCode.InstallScope != def.Agent.OpenCode.InstallScope {
		t.Errorf("opencode install_scope = %q, want %q", cfg.Agent.OpenCode.InstallScope, def.Agent.OpenCode.InstallScope)
	}
	if !reflect.DeepEqual(cfg.Notify.OpenCode.Events, def.Notify.OpenCode.Events) {
		t.Errorf("opencode events = %#v, want %#v", cfg.Notify.OpenCode.Events, def.Notify.OpenCode.Events)
	}
	// 行为默认值同样不能因为改用 Default() 起手而丢失
	if cfg.Behavior.DedupeSeconds != def.Behavior.DedupeSeconds {
		t.Errorf("dedupe_seconds = %d, want %d", cfg.Behavior.DedupeSeconds, def.Behavior.DedupeSeconds)
	}
	if cfg.Behavior.Locale != def.Behavior.Locale {
		t.Errorf("locale = %q, want %q", cfg.Behavior.Locale, def.Behavior.Locale)
	}
}

// v1 里被固化到磁盘的 click_to_focus: false 应在加载时一次性翻回 true，
// 并把版本号推到 currentConfigVersion。
func TestLoadMigratesPersistedClickToFocusFalse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	v1 := `version: 1
notify:
    opencode:
        events:
            - permission_required
        channels:
            system:
                enabled: true
                click_to_focus: false
    claude_code:
        channels:
            system:
                enabled: true
                click_to_focus: false
`
	if err := os.WriteFile(path, []byte(v1), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Notify.OpenCode.Channels.System.ClickToFocus {
		t.Error("opencode click_to_focus was not migrated to true")
	}
	if !cfg.Notify.ClaudeCode.Channels.System.ClickToFocus {
		t.Error("claude_code click_to_focus was not migrated to true")
	}
	if cfg.Version != currentConfigVersion {
		t.Errorf("Version = %d, want %d after migration", cfg.Version, currentConfigVersion)
	}
}

// 迁移只跑一次：已经是当前版本的配置里，显式的 false 必须被尊重。
func TestLoadRespectsExplicitFalseAtCurrentVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	current := fmt.Sprintf(`version: %d
notify:
    opencode:
        events:
            - permission_required
        channels:
            system:
                enabled: true
                click_to_focus: false
`, currentConfigVersion)
	if err := os.WriteFile(path, []byte(current), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Notify.OpenCode.Channels.System.ClickToFocus {
		t.Error("explicit click_to_focus: false at current version was overridden by the migration")
	}
}

// 回归：DSH 接入前写下的老配置（无 dsh 段）加载后必须拿到 Default() 的值，
// 而不是 Go 零值。这条与 OpenCode 的同类回归同构——Load 把 YAML 解析进
// Default()，所以新增 agent 只要在 Default() 里给对初值就自动生效；
// 一旦有人退回「解析进零值再手工补字段」，这里会立刻失败。
func TestLoadFillsDSHDefaultsForLegacyConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	legacy := `version: 2
agent:
    claude_code:
        enabled: true
        install_scope: user
notify:
    claude_code:
        events:
            - permission_required
        channels:
            system:
                enabled: true
                click_to_focus: true
`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	def := Default()

	// DSH 的四个通知事件必须回填成默认值。
	if len(cfg.Notify.DSH.Events) != len(def.Notify.DSH.Events) {
		t.Errorf("dsh events = %v, want the defaults %v", cfg.Notify.DSH.Events, def.Notify.DSH.Events)
	}
	// click_to_focus 是历史事故字段，必须保持默认 true。
	if cfg.Notify.DSH.Channels.System.ClickToFocus != def.Notify.DSH.Channels.System.ClickToFocus {
		t.Errorf("dsh click_to_focus = %v, want %v",
			cfg.Notify.DSH.Channels.System.ClickToFocus, def.Notify.DSH.Channels.System.ClickToFocus)
	}
	if cfg.Agent.DSH.InstallScope != def.Agent.DSH.InstallScope {
		t.Errorf("dsh install_scope = %q, want %q", cfg.Agent.DSH.InstallScope, def.Agent.DSH.InstallScope)
	}
	// 新 agent 默认关闭：接入不等于替用户打开通知。
	if cfg.Agent.DSH.Enabled {
		t.Error("dsh enabled = true, want false until the user opts in")
	}
}

// TestDSHEventsMatchSupportedSurface 锁住 DSH 的默认事件面。
//
// DSH 原生插件能拿全四个通知事件（approval/request 与 user-questions/request
// 都在原生扩展点上），这是它相对 DSH 自带 Claude Code bridge 的核心增量——
// 后者只认 SessionStart / Stop。若有人照抄 Codex 的两事件默认值，这里会失败。
func TestDSHEventsMatchSupportedSurface(t *testing.T) {
	def := Default()

	want := map[string]bool{
		"permission_required": true,
		"input_required":      true,
		"run_completed":       true,
		"run_failed":          true,
	}
	if len(def.Notify.DSH.Events) != len(want) {
		t.Fatalf("dsh events = %v, want all four notification events", def.Notify.DSH.Events)
	}
	for _, e := range def.Notify.DSH.Events {
		if !want[e] {
			t.Errorf("dsh events contains unexpected %q", e)
		}
	}
	// session_start 只服务点击聚焦，不是通知事件。
	for _, e := range def.Notify.DSH.Events {
		if e == "session_start" {
			t.Error("dsh events must not list session_start: it only drives focus capture")
		}
	}
}
