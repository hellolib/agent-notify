package claudehooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hellolib/agent-notify/internal/common"
)

func TestBuildHookSettings(t *testing.T) {
	got := BuildHookSettings("/tmp/agent-notify")

	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks type = %T, want map[string]any", got["hooks"])
	}

	events := []string{"PermissionRequest", "Notification", "Stop", "PostToolUseFailure"}
	for _, event := range events {
		items, ok := hooks[event].([]map[string]any)
		if !ok || len(items) != 1 {
			t.Fatalf("%s hooks missing or invalid", event)
		}
		entryHooks, ok := items[0]["hooks"].([]map[string]any)
		if !ok || len(entryHooks) != 1 {
			t.Fatalf("%s command hooks missing or invalid", event)
		}
		if entryHooks[0]["command"] != "/tmp/agent-notify handle-claude-hook" {
			t.Fatalf("%s command = %v, want /tmp/agent-notify handle-claude-hook", event, entryHooks[0]["command"])
		}
	}
}

func TestInstallMergesExistingSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"theme":"dark"}`), 0o644); err != nil {
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
	if got["theme"] != "dark" {
		t.Fatalf("theme = %v, want dark", got["theme"])
	}
	if _, ok := got["hooks"]; !ok {
		t.Fatal("hooks key missing")
	}
}

// TestInstallPreservesUserHooks 用户已经在 Stop 事件下挂载了自己的 hook，
// 增量安装应当追加 agent-notify 的条目，而不是覆盖。
func TestInstallPreservesUserHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	existing := `{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {"type": "command", "command": "echo user-stop"}
        ]
      }
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

// TestInstallIdempotent 重复安装不应产生重复的 agent-notify hook 条目。
func TestInstallIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

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

// TestUninstallRemovesOnlyManagedHooks 卸载应只删除 agent-notify 写入的 hook，
// 用户自定义 hook 原样保留。
func TestUninstallRemovesOnlyManagedHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	existing := `{
  "theme": "dark",
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
	if got["theme"] != "dark" {
		t.Fatalf("unrelated config lost: theme = %v", got["theme"])
	}

	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatal("user Stop hook should remain — hooks map missing entirely")
	}
	for _, unmanagedEvent := range []string{"PermissionRequest", "Notification", "PostToolUseFailure"} {
		if _, exists := hooks[unmanagedEvent]; exists {
			t.Fatalf("%s should be removed (no user hooks under it)", unmanagedEvent)
		}
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

// TestUninstallDropsEmptyHooksMap 没有任何用户 hook 时，卸载应连带删掉
// hooks 顶层 key，避免遗留空对象。
func TestUninstallDropsEmptyHooksMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"theme":"dark"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if err := Uninstall(path); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}

	got := readSettingsForTest(t, path)
	if _, exists := got["hooks"]; exists {
		t.Fatalf("hooks key should be removed when empty, got %v", got["hooks"])
	}
	if got["theme"] != "dark" {
		t.Fatalf("unrelated config lost: theme = %v", got["theme"])
	}
}

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
