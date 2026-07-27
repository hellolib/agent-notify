package zcodehooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/common"
)

// TestBuildHookSettingsStructure 验证 ZCode 的配置结构：
// 必须有 hooks.enabled=true 和 hooks.events.<Event>，与 Claude Code 的扁平结构不同。
func TestBuildHookSettingsStructure(t *testing.T) {
	got := BuildHookSettings("/tmp/agent-notify")

	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks type = %T, want map[string]any", got["hooks"])
	}
	if hooks["enabled"] != true {
		t.Fatalf("hooks.enabled = %v, want true (ZCode requires enabled:true)", hooks["enabled"])
	}

	events, ok := hooks["events"].(map[string]any)
	if !ok {
		t.Fatalf("hooks.events type = %T, want map[string]any (ZCode nests events under hooks.events)", hooks["events"])
	}

	for _, event := range managedEvents {
		items, ok := events[event].([]map[string]any)
		if !ok || len(items) != 1 {
			t.Fatalf("%s hooks missing or invalid under hooks.events", event)
		}
		entryHooks, ok := items[0]["hooks"].([]map[string]any)
		if !ok || len(entryHooks) != 1 {
			t.Fatalf("%s command hooks missing or invalid", event)
		}
		if entryHooks[0]["command"] != `"/tmp/agent-notify" handle-zcode-hook` {
			t.Fatalf("%s command = %v, want /tmp/agent-notify handle-zcode-hook", event, entryHooks[0]["command"])
		}
	}
}

// TestInstallMergesExistingSettings 验证安装时不破坏 config.json 里的其它顶层键（如 mcp）。
func TestInstallMergesExistingSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	existing := `{"mcp":{"servers":{"chrome-devtools":{"type":"stdio","command":"chrome-devtools-mcp"}}}}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	got := readSettingsForTest(t, path)
	mcp, ok := got["mcp"].(map[string]any)
	if !ok {
		t.Fatalf("mcp key lost after install: %v", got["mcp"])
	}
	servers, ok := mcp["servers"].(map[string]any)
	if !ok {
		t.Fatalf("mcp.servers lost after install")
	}
	if _, ok := servers["chrome-devtools"]; !ok {
		t.Fatal("mcp.servers.chrome-devtools lost after install")
	}

	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatal("hooks key missing")
	}
	if hooks["enabled"] != true {
		t.Fatalf("hooks.enabled = %v, want true", hooks["enabled"])
	}
}

// TestInstallPreservesUserHooks 用户已在 Stop 事件下挂载自己的 hook，
// 增量安装应追加 agent-notify 条目，而不是覆盖。
func TestInstallPreservesUserHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	existing := `{
  "hooks": {
    "enabled": true,
    "events": {
      "Stop": [
        {"hooks": [{"type": "command", "command": "echo user-stop"}]}
      ]
    }
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
	events := hooks["events"].(map[string]any)
	stopEntries := events["Stop"].([]any)
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

// TestInstallIdempotent 重复安装不应产生重复的 agent-notify hook 条目。
func TestInstallIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("first install error = %v", err)
	}
	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("second install error = %v", err)
	}

	got := readSettingsForTest(t, path)
	hooks := got["hooks"].(map[string]any)
	events := hooks["events"].(map[string]any)
	for _, event := range managedEvents {
		entries := events[event].([]any)
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

// TestUninstallRemovesOnlyManagedHooks 卸载应只删 agent-notify 写入的 hook，
// 用户自定义 hook 原样保留；其它顶层键（如 mcp）也不动。
func TestUninstallRemovesOnlyManagedHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	existing := `{
  "mcp": {"servers": {"x": {}}},
  "hooks": {
    "enabled": true,
    "events": {
      "Stop": [
        {"hooks": [{"type": "command", "command": "echo user-stop"}]}
      ]
    }
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
	if _, ok := got["mcp"]; !ok {
		t.Fatal("mcp key should be preserved after uninstall")
	}

	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatal("user Stop hook should remain — hooks map missing entirely")
	}
	// agent-notify 托管的其它事件应被清掉
	events := eventsMap(hooks)
	for _, managed := range managedEvents {
		if managed == "Stop" {
			continue
		}
		if _, exists := events[managed]; exists {
			t.Fatalf("%s should be removed (no user hooks under it)", managed)
		}
	}

	stopEntries, ok := events["Stop"].([]any)
	if !ok || len(stopEntries) != 1 {
		t.Fatalf("Stop should retain 1 user hook entry, got %v", events["Stop"])
	}
	commands := collectCommandsForTest(stopEntries)
	if !containsString(commands, "echo user-stop") {
		t.Fatalf("user hook lost after uninstall: %v", commands)
	}
	if containsSubstring(commands, hookCommandMarker) {
		t.Fatalf("agent-notify hook still present after uninstall: %v", commands)
	}
}

// TestUninstallNoFileIsNoop 文件不存在时卸载是 no-op，不创建文件。
func TestUninstallNoFileIsNoop(t *testing.T) {
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

// eventsMap 从 hooks 对象中取出 events 子对象（测试辅助）。
func eventsMap(hooks map[string]any) map[string]any {
	events, _ := hooks["events"].(map[string]any)
	if events == nil {
		return map[string]any{}
	}
	return events
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
	path := filepath.Join(dir, "config.json")
	original := `{"hooks":{"enabled":true,"events":{"Stop":{"hooks":[{"type":"command","command":"echo mine"}]}}}}`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Install(path, "/tmp/agent-notify")
	if err == nil {
		t.Fatal("Install 应当拒绝写入,而不是替换掉用户的定义")
	}
	if !strings.Contains(err.Error(), "hooks.events.Stop") {
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
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(`{"hooks":{"enabled":true,"events":{"Stop":{"mine":true},"PermissionRequest":[{"hooks":[{"type":"command","command":"/x handle-zcode-hook"}]}]}}}`), 0o644); err != nil {
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
