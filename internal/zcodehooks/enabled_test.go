package zcodehooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeZcodeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readZcodeConfig(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("config is not valid JSON: %v", err)
	}
	return out
}

// 对齐 Codex 的 EnableHooksFeature:hooks.enabled 是用户所有 ZCode hook 的总开关,
// 用户显式关闭时安装应直接替他打开(true),而不是拒绝安装。
// (撤销 issue #35「不替用户悄悄打开」的旧决策,改为与 Codex 一致。)
func TestInstallEnablesWhenExplicitlyDisabled(t *testing.T) {
	path := writeZcodeConfig(t,
		`{"hooks":{"enabled":false,"events":{"Stop":[{"hooks":[{"type":"command","command":"echo user"}]}]}}}`)

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("Install() error = %v, want success (should flip enabled false→true)", err)
	}

	got := readZcodeConfig(t, path)
	hooks := got["hooks"].(map[string]any)
	if hooks["enabled"] != true {
		t.Fatalf("enabled = %v, want true (should be flipped from false)", hooks["enabled"])
	}

	// 用户的既有 Stop hook 应保留,并与 agent-notify 的托管 hook 并存。
	stop := hooks["events"].(map[string]any)["Stop"].([]any)
	var sawUser, sawManaged bool
	for _, entry := range stop {
		for _, h := range entry.(map[string]any)["hooks"].([]any) {
			cmd, _ := h.(map[string]any)["command"].(string)
			if strings.Contains(cmd, "echo user") {
				sawUser = true
			}
			if strings.Contains(cmd, hookCommandMarker) {
				sawManaged = true
			}
		}
	}
	if !sawUser {
		t.Fatal("user's existing Stop hook was lost")
	}
	if !sawManaged {
		t.Fatal("agent-notify managed hook not installed")
	}
}

func TestInstallCreatesEnabledOnlyWhenAbsent(t *testing.T) {
	path := writeZcodeConfig(t, `{"theme":"dark"}`)

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	got := readZcodeConfig(t, path)
	hooks := got["hooks"].(map[string]any)
	if hooks["enabled"] != true {
		t.Fatalf("enabled = %v, want true when key was absent", hooks["enabled"])
	}
	if got["theme"] != "dark" {
		t.Fatal("unrelated top-level key lost")
	}
}

// 用户自己开着 enabled 时,安装不应改动该键(值相同,但语义上归属用户)。
func TestInstallKeepsUserEnabledTrue(t *testing.T) {
	path := writeZcodeConfig(t, `{"hooks":{"enabled":true}}`)

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	hooks := readZcodeConfig(t, path)["hooks"].(map[string]any)
	if hooks["enabled"] != true {
		t.Fatalf("enabled = %v, want true", hooks["enabled"])
	}
}

// issue #35:Claude 风格的扁平事件键对 ZCode 是未知键,会让整个 hooks 配置
// 被静默丢弃。安装时必须把它迁进 events,而不是留下混合形状。
func TestInstallMigratesLegacyFlatEventKeys(t *testing.T) {
	path := writeZcodeConfig(t,
		`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo legacy"}]}]}}`)

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	hooks := readZcodeConfig(t, path)["hooks"].(map[string]any)
	if _, stillFlat := hooks["Stop"]; stillFlat {
		t.Fatal("flat Stop key survived; ZCode would drop the whole hooks config")
	}
	for key := range hooks {
		if key != "enabled" && key != "events" {
			t.Fatalf("unexpected key %q in hooks object", key)
		}
	}

	events := hooks["events"].(map[string]any)
	stop := events["Stop"].([]any)
	var sawLegacy, sawManaged bool
	for _, entry := range stop {
		for _, h := range entry.(map[string]any)["hooks"].([]any) {
			cmd, _ := h.(map[string]any)["command"].(string)
			if strings.Contains(cmd, "echo legacy") {
				sawLegacy = true
			}
			if strings.Contains(cmd, hookCommandMarker) {
				sawManaged = true
			}
		}
	}
	if !sawLegacy {
		t.Fatal("user's legacy hook was lost during migration")
	}
	if !sawManaged {
		t.Fatal("agent-notify hook not installed")
	}
}

func TestInstallRefusesUnknownHooksKey(t *testing.T) {
	original := `{"hooks":{"NotAnEvent":[{"hooks":[]}]}}`
	path := writeZcodeConfig(t, original)

	err := Install(path, "/tmp/agent-notify")
	if err == nil {
		t.Fatal("Install() error = nil, want refusal on unrecognized hooks key")
	}
	data, _ := os.ReadFile(path)
	if string(data) != original {
		t.Fatal("config modified despite refusal")
	}
}

// 全新安装再卸载:agent-notify 的 hook 全部移除,但 enabled 总开关保留——
// 对齐 Codex 卸载时不碰 [features] hooks 的做法。无 events 即不触发,无害。
func TestUninstallKeepsEnabledWhenEventsEmpty(t *testing.T) {
	path := writeZcodeConfig(t, `{"theme":"dark"}`)

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if err := Uninstall(path); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}

	got := readZcodeConfig(t, path)
	if got["theme"] != "dark" {
		t.Fatal("unrelated key lost")
	}
	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatal("hooks block should remain with enabled after uninstall")
	}
	if hooks["enabled"] != true {
		t.Fatalf("enabled = %v, want true (shared switch, kept on uninstall)", hooks["enabled"])
	}
	if _, hasEvents := hooks["events"]; hasEvents {
		t.Fatalf("events should be removed after uninstall: %v", hooks["events"])
	}
}
