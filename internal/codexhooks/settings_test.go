package codexhooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/common"
)

func TestBuildHookSettings_RegistersManagedEvents(t *testing.T) {
	got := BuildHookSettings("/tmp/agent-notify")

	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks type = %T, want map[string]any", got["hooks"])
	}

	for _, event := range []string{"SessionStart", "PermissionRequest", "Stop"} {
		items, ok := hooks[event].([]map[string]any)
		if !ok || len(items) != 1 {
			t.Fatalf("%s entries missing or invalid: %v", event, hooks[event])
		}
		entryHooks, ok := items[0]["hooks"].([]map[string]any)
		if !ok || len(entryHooks) != 1 {
			t.Fatalf("%s command list missing or invalid", event)
		}
		if entryHooks[0]["command"] != `"/tmp/agent-notify" handle-codex-hook` && entryHooks[0]["command"] != `/tmp/agent-notify handle-codex-hook` {
			t.Fatalf("%s command = %v, want quoted or unquoted /tmp/agent-notify handle-codex-hook", event, entryHooks[0]["command"])
		}
		if entryHooks[0]["type"] != "command" {
			t.Fatalf("%s type = %v, want command", event, entryHooks[0]["type"])
		}
	}

	// 不应注册 Codex 不支持的事件（SessionStart 现在仅用于 Linux 聚焦捕获，已托管）
	for _, unsupported := range []string{"Notification", "PostToolUseFailure", "UserPromptSubmit", "PreToolUse", "PostToolUse"} {
		if _, exists := hooks[unsupported]; exists {
			t.Fatalf("hooks should not contain %s for Codex", unsupported)
		}
	}
}

func TestInstall_MergesExistingHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	existing := `{"hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"echo hi"}]}]}}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatal("hooks key missing or wrong type")
	}
	for _, key := range []string{"SessionStart", "PermissionRequest", "Stop"} {
		if _, exists := hooks[key]; !exists {
			t.Fatalf("hooks missing key %q after install", key)
		}
	}
}

func TestInstall_CreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deeper", "hooks.json")

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("hooks.json not created at %q: %v", path, err)
	}
}

// TestInstall_PreservesUserHooks 用户已经在 Stop 事件下挂了自己的 hook，
// 增量安装应当追加 agent-notify 条目而不是覆盖。
func TestInstall_PreservesUserHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	existing := `{
  "hooks": {
    "Stop": [
      {"hooks": [{"type": "command", "command": "echo user-stop"}]}
    ]
  }
}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	got := readSettingsForTest(t, path)
	hooks := got["hooks"].(map[string]any)
	stopEntries := hooks["Stop"].([]any)
	if len(stopEntries) != 2 {
		t.Fatalf("Stop entry count = %d, want 2 (user + agent-notify)", len(stopEntries))
	}

	commands := collectCommandsForTest(stopEntries)
	if !containsString(commands, "echo user-stop") {
		t.Fatalf("user hook command lost: %v", commands)
	}
	if !containsSubstring(commands, hookCommandMarker) {
		t.Fatalf("agent-notify hook command missing: %v", commands)
	}
}

// TestInstall_Idempotent 重复安装不应产生重复的 agent-notify 条目。
func TestInstall_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("first install error = %v", err)
	}
	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("second install error = %v", err)
	}

	got := readSettingsForTest(t, path)
	hooks := got["hooks"].(map[string]any)
	for _, event := range managedEvents {
		entries := hooks[event].([]any)
		marked := 0
		for _, e := range entries {
			entryMap := e.(map[string]any)
			for _, h := range entryMap["hooks"].([]any) {
				if common.IsManagedHook(h, hookCommandMarker) {
					marked++
				}
			}
		}
		if marked != 1 {
			t.Fatalf("%s has %d agent-notify hooks after re-install, want 1", event, marked)
		}
	}
}

// TestUninstall_RemovesOnlyManagedHooks 卸载只删 agent-notify 写入的 hook，
// 用户自定义 hook 原样保留。
func TestUninstall_RemovesOnlyManagedHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	existing := `{
  "hooks": {
    "Stop": [
      {"hooks": [{"type": "command", "command": "echo user-stop"}]}
    ]
  }
}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if err := Uninstall(path); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}

	got := readSettingsForTest(t, path)
	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatal("user Stop hook should remain — hooks map missing entirely")
	}
	if _, exists := hooks["PermissionRequest"]; exists {
		t.Fatal("PermissionRequest should be removed (no user hooks under it)")
	}

	stopEntries, ok := hooks["Stop"].([]any)
	if !ok || len(stopEntries) != 1 {
		t.Fatalf("Stop should retain 1 user hook entry, got %v", hooks["Stop"])
	}
	commands := collectCommandsForTest(stopEntries)
	if !containsString(commands, "echo user-stop") {
		t.Fatalf("user hook lost after uninstall: %v", commands)
	}
	if containsSubstring(commands, hookCommandMarker) {
		t.Fatalf("agent-notify hook still present after uninstall: %v", commands)
	}
}

// TestUninstall_LeavesFeaturesHooksFlag 卸载不能动 config.toml 里的 features.hooks
// 开关 —— 这是通用开关，其他 hook 可能依赖它。
func TestUninstall_LeavesFeaturesHooksFlag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	tomlPath := filepath.Join(dir, "config.toml")

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	// Install 会写入 features.hooks=true
	if _, err := os.Stat(tomlPath); err != nil {
		t.Fatalf("config.toml not written by Install: %v", err)
	}

	if err := Uninstall(path); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if _, err := os.Stat(tomlPath); err != nil {
		t.Fatalf("config.toml should still exist after Uninstall: %v", err)
	}
}

func TestUninstall_NoFileIsNoop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")
	if err := Uninstall(path); err != nil {
		t.Fatalf("Uninstall on missing file should be no-op, got error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("Uninstall should not create the file when it didn't exist")
	}
}

func readSettingsForTest(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]any{}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func collectCommandsForTest(entries []any) []string {
	var out []string
	for _, e := range entries {
		entryMap, ok := e.(map[string]any)
		if !ok {
			continue
		}
		inner, ok := entryMap["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range inner {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, ok := hm["command"].(string); ok {
				out = append(out, cmd)
			}
		}
	}
	return out
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func containsSubstring(haystack []string, needle string) bool {
	for _, s := range haystack {
		if len(s) >= len(needle) && len(needle) > 0 {
			for i := 0; i+len(needle) <= len(s); i++ {
				if s[i:i+len(needle)] == needle {
					return true
				}
			}
		}
	}
	return false
}

// TestInstallRefusesNonArrayHookValue 是 issue #39 第 6 项的回归测试。
// 旧实现里 common.ToAnySlice 对非数组返回 nil,Install 据此认为「这个事件下
// 什么都没有」而整个替换掉——用户手写成对象形式的 hook 定义无声消失。
func TestInstallRefusesNonArrayHookValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	original := `{"hooks":{"Stop":{"hooks":[{"type":"command","command":"echo mine"}]}}}`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Install(path, "/tmp/agent-notify")
	if err == nil {
		t.Fatal("Install 应当拒绝写入,而不是替换掉用户的定义")
	}
	if !strings.Contains(err.Error(), "hooks.Stop") {
		t.Fatalf("错误信息应指出是哪个事件,实际是: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Fatalf("拒绝写入时文件不该被改动:\n got %s\nwant %s", data, original)
	}
}

// TestUninstallKeepsNonArrayHookValue 卸载不该被用户的无关配置阻塞:
// 非数组形态里不可能有我们写的 entry。
func TestUninstallKeepsNonArrayHookValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	if err := os.WriteFile(path, []byte(`{"hooks":{"Stop":{"mine":true},"PermissionRequest":[{"hooks":[{"type":"command","command":"/x handle-codex-hook"}]}]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Uninstall(path); err != nil {
		t.Fatalf("Uninstall 不应报错: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"mine"`) {
		t.Fatalf("用户手写的非数组值被删掉了:\n%s", data)
	}
	if strings.Contains(string(data), hookCommandMarker) {
		t.Fatalf("托管 hook 未被移除:\n%s", data)
	}
}
