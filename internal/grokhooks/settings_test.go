package grokhooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/common"
)

func TestBuildHookSettingsStructure(t *testing.T) {
	got := BuildHookSettings("/tmp/agent-notify")

	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks type = %T, want map[string]any", got["hooks"])
	}

	for _, event := range managedEvents {
		items, ok := hooks[event].([]map[string]any)
		if !ok || len(items) != 1 {
			t.Fatalf("%s hooks missing or invalid", event)
		}
		entryHooks, ok := items[0]["hooks"].([]map[string]any)
		if !ok || len(entryHooks) != 1 {
			t.Fatalf("%s command hooks missing or invalid", event)
		}
		if entryHooks[0]["command"] != `"/tmp/agent-notify" handle-grok-hook` {
			t.Fatalf("%s command = %v, want /tmp/agent-notify handle-grok-hook", event, entryHooks[0]["command"])
		}
	}
}

func TestInstallCreatesHooksFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks", "agent-notify.json")

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	got := readSettingsForTest(t, path)
	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatal("hooks key missing")
	}
	for _, event := range managedEvents {
		if _, ok := hooks[event]; !ok {
			t.Fatalf("missing managed event %s", event)
		}
	}

	installed, err := IsInstalled(path)
	if err != nil {
		t.Fatalf("IsInstalled() error = %v", err)
	}
	if !installed {
		t.Fatal("IsInstalled() = false, want true")
	}
}

func TestInstallPreservesUserHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-notify.json")
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

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "echo user-stop") {
		t.Fatalf("user hook lost after install: %s", content)
	}
	if !strings.Contains(content, "handle-grok-hook") {
		t.Fatalf("managed hook missing after install: %s", content)
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-notify.json")

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatal(err)
	}
	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatal(err)
	}

	got := readSettingsForTest(t, path)
	hooks := got["hooks"].(map[string]any)
	entries := common.ToAnySlice(hooks["Stop"])
	if len(entries) != 1 {
		t.Fatalf("Stop entries = %d, want 1 after double install", len(entries))
	}
}

func TestUninstallRemovesManagedHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-notify.json")
	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatal(err)
	}

	if err := Uninstall(path); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}

	// 仅含本插件 hooks 时文件应被删除
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected hook file removed, stat err = %v", err)
	}
}

func TestUninstallPreservesUserHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-notify.json")
	existing := `{
  "hooks": {
    "Stop": [
      {"hooks": [
        {"type": "command", "command": "echo user-stop"},
        {"type": "command", "command": "/tmp/agent-notify handle-grok-hook"}
      ]}
    ]
  }
}`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Uninstall(path); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "echo user-stop") {
		t.Fatalf("user hook lost after uninstall: %s", content)
	}
	if strings.Contains(content, "handle-grok-hook") {
		t.Fatalf("managed hook still present after uninstall: %s", content)
	}
}

func readSettingsForTest(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

// TestInstallRefusesNonArrayHookValue 是 issue #39 第 6 项的回归测试。
// 旧实现里 common.ToAnySlice 对非数组返回 nil,Install 据此认为「这个事件下
// 什么都没有」而整个替换掉——用户手写成对象形式的 hook 定义无声消失。
func TestInstallRefusesNonArrayHookValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-notify.json")
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
	path := filepath.Join(dir, "agent-notify.json")
	if err := os.WriteFile(path, []byte(`{"hooks":{"Stop":{"mine":true},"Notification":[{"hooks":[{"type":"command","command":"/x handle-grok-hook"}]}]}}`), 0o644); err != nil {
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
