package grokhooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		if entryHooks[0]["command"] != "/tmp/agent-notify handle-grok-hook" {
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
	entries := toAnySlice(hooks["Stop"])
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
