package droidhooks

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

	// Droid hooks.json 中事件直接位于顶层，不嵌套在 "hooks" 下
	if _, ok := got["hooks"]; ok {
		t.Fatal("events should be at top level, not nested under \"hooks\"")
	}

	for _, event := range managedEvents {
		items, ok := got[event].([]map[string]any)
		if !ok || len(items) != 1 {
			t.Fatalf("%s entries missing or invalid", event)
		}
		entryHooks, ok := items[0]["hooks"].([]map[string]any)
		if !ok || len(entryHooks) != 1 {
			t.Fatalf("%s command hooks missing or invalid", event)
		}
		if entryHooks[0]["command"] != `"/tmp/agent-notify" handle-droid-hook` {
			t.Fatalf("%s command = %v, want /tmp/agent-notify handle-droid-hook", event, entryHooks[0]["command"])
		}
	}
}

func TestInstallCreatesHooksFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	got := readSettingsForTest(t, path)
	// 事件应直接位于顶层
	if _, ok := got["hooks"]; ok {
		t.Fatal("events should be at top level, not nested under \"hooks\"")
	}
	for _, event := range managedEvents {
		if _, ok := got[event]; !ok {
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
	path := filepath.Join(dir, "hooks.json")
	existing := `{
  "hooksDisabled": false,
  "showHookOutput": false,
  "Stop": [
    {"hooks": [{"type": "command", "command": "echo user-stop"}]}
  ]
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
	if !strings.Contains(content, "handle-droid-hook") {
		t.Fatalf("managed hook missing after install: %s", content)
	}
	if !strings.Contains(content, "hooksDisabled") {
		t.Fatalf("hooksDisabled key lost after install: %s", content)
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatal(err)
	}
	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatal(err)
	}

	got := readSettingsForTest(t, path)
	entries := common.ToAnySlice(got["Stop"])
	if len(entries) != 1 {
		t.Fatalf("Stop entries = %d, want 1 after double install", len(entries))
	}
}

func TestUninstallRemovesManagedHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
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

func TestUninstallPreservesUserHooksAndDroidKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	existing := `{
  "hooksDisabled": false,
  "showHookOutput": false,
  "Stop": [
    {"hooks": [
      {"type": "command", "command": "echo user-stop"},
      {"type": "command", "command": "/tmp/agent-notify handle-droid-hook"}
    ]}
  ]
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
	if strings.Contains(content, "handle-droid-hook") {
		t.Fatalf("managed hook still present after uninstall: %s", content)
	}
	if !strings.Contains(content, "hooksDisabled") {
		t.Fatalf("hooksDisabled key lost after uninstall: %s", content)
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
