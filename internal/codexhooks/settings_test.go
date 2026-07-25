package codexhooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hellolib/agent-notify/internal/common"
)

func TestBuildHookSettings_RegistersManagedEvents(t *testing.T) {
	got := BuildHookSettings("/tmp/agent-notify")

	hooks, ok := got["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks type = %T, want map[string]any", got["hooks"])
	}

	for _, event := range []string{"SessionStart", "PermissionRequest", "PreToolUse"} {
		items, ok := hooks[event].([]map[string]any)
		if !ok || len(items) != 1 {
			t.Fatalf("%s entries missing or invalid: %v", event, hooks[event])
		}
		entryHooks, ok := items[0]["hooks"].([]map[string]any)
		if !ok || len(entryHooks) != 1 {
			t.Fatalf("%s command list missing or invalid", event)
		}
		if entryHooks[0]["command"] != "/tmp/agent-notify handle-codex-hook" {
			t.Fatalf("%s command = %v, want /tmp/agent-notify handle-codex-hook", event, entryHooks[0]["command"])
		}
		if entryHooks[0]["type"] != "command" {
			t.Fatalf("%s type = %v, want command", event, entryHooks[0]["type"])
		}
		if event == "PreToolUse" && items[0]["matcher"] != requestUserInputMatcher {
			t.Fatalf("PreToolUse matcher = %v, want %q", items[0]["matcher"], requestUserInputMatcher)
		}
	}

	// Stop is supported by Codex, but it means a turn stopped rather than task
	// completion, so agent-notify deliberately does not manage it.
	for _, unmanaged := range []string{"Stop", "Notification", "PostToolUseFailure", "UserPromptSubmit", "PostToolUse"} {
		if _, exists := hooks[unmanaged]; exists {
			t.Fatalf("hooks should not contain %s for Codex", unmanaged)
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
	for _, key := range []string{"SessionStart", "PermissionRequest", "PreToolUse"} {
		if _, exists := hooks[key]; !exists {
			t.Fatalf("hooks missing key %q after install", key)
		}
	}
	if _, exists := hooks["Stop"]; exists {
		t.Fatalf("hooks should not contain managed Stop after install: %v", hooks["Stop"])
	}
}

func TestInstall_PreToolUseUsesExactMatcherAndPreservesCustomHook(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	existing := `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "^shell$", "hooks": [{"type": "command", "command": "echo user-shell"}]}
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
	entries := hooks["PreToolUse"].([]any)
	if len(entries) != 2 {
		t.Fatalf("PreToolUse entry count = %d, want 2 (user + agent-notify)", len(entries))
	}

	var managedMatcher string
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if containsSubstring(collectCommandsForTest([]any{entry}), hookCommandMarker) {
			managedMatcher, _ = entry["matcher"].(string)
		}
	}
	if managedMatcher != requestUserInputMatcher {
		t.Fatalf("managed PreToolUse matcher = %q, want %q", managedMatcher, requestUserInputMatcher)
	}
	commands := collectCommandsForTest(entries)
	if !containsString(commands, "echo user-shell") {
		t.Fatalf("custom PreToolUse hook was lost: %v", commands)
	}
}

func TestInstall_UpgradesLegacyManagedHooksWithPreToolUseAndRemovesStop(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	existing := `{
  "hooks": {
    "SessionStart": [{"hooks": [{"type": "command", "command": "/tmp/agent-notify handle-codex-hook"}]}],
    "PermissionRequest": [{"hooks": [{"type": "command", "command": "/tmp/agent-notify handle-codex-hook"}]}],
    "Stop": [{"hooks": [{"type": "command", "command": "/tmp/agent-notify handle-codex-hook"}]}]
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
	entries := hooks["PreToolUse"].([]any)
	if len(entries) != 1 {
		t.Fatalf("PreToolUse entry count = %d, want 1", len(entries))
	}
	entry := entries[0].(map[string]any)
	if entry["matcher"] != requestUserInputMatcher {
		t.Fatalf("PreToolUse matcher = %v, want %q", entry["matcher"], requestUserInputMatcher)
	}
	if _, exists := hooks["Stop"]; exists {
		t.Fatalf("legacy managed Stop should be removed, got %v", hooks["Stop"])
	}
}

func TestInstall_RepairsExistingManagedPreToolUseMatcher(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	existing := `{
  "hooks": {
    "PreToolUse": [
      {"matcher": ".*", "hooks": [{"type": "command", "command": "/tmp/agent-notify handle-codex-hook"}]},
      {"matcher": "^shell$", "hooks": [{"type": "command", "command": "echo user-shell"}]}
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
	entries := hooks["PreToolUse"].([]any)
	if len(entries) != 2 {
		t.Fatalf("PreToolUse entry count = %d, want 2", len(entries))
	}
	var managedMatcher, customMatcher any
	for _, raw := range entries {
		entry := raw.(map[string]any)
		commands := collectCommandsForTest([]any{entry})
		if containsSubstring(commands, hookCommandMarker) {
			managedMatcher = entry["matcher"]
		} else {
			customMatcher = entry["matcher"]
		}
	}
	if managedMatcher != requestUserInputMatcher {
		t.Fatalf("managed matcher = %v, want %q", managedMatcher, requestUserInputMatcher)
	}
	if customMatcher != "^shell$" {
		t.Fatalf("custom matcher = %v, want ^shell$", customMatcher)
	}
}

func TestInstall_SplitsMixedManagedAndCustomPreToolUseHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	existing := `{
  "hooks": {
    "PreToolUse": [
      {"matcher": ".*", "hooks": [
        {"type": "command", "command": "echo user"},
        {"type": "command", "command": "/tmp/agent-notify handle-codex-hook"}
      ]}
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
	entries := hooks["PreToolUse"].([]any)
	if len(entries) != 2 {
		t.Fatalf("PreToolUse entry count = %d, want split custom + managed entries", len(entries))
	}
	var custom, managed map[string]any
	for _, raw := range entries {
		entry := raw.(map[string]any)
		if containsSubstring(collectCommandsForTest([]any{entry}), hookCommandMarker) {
			managed = entry
		} else {
			custom = entry
		}
	}
	if managed == nil || managed["matcher"] != requestUserInputMatcher {
		t.Fatalf("managed entry = %#v, want exact matcher", managed)
	}
	if custom == nil || custom["matcher"] != ".*" || !containsString(collectCommandsForTest([]any{custom}), "echo user") {
		t.Fatalf("custom entry was changed or lost: %#v", custom)
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

// TestInstall_RemovesLegacyManagedStopAndPreservesUserHooks covers both a
// mixed matcher group and an independent user Stop group.
func TestInstall_RemovesLegacyManagedStopAndPreservesUserHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	existing := `{
  "hooks": {
    "Stop": [
      {"matcher": "ignored-by-codex", "statusMessage": "keep me", "hooks": [
        {"type": "command", "command": "echo mixed-user-stop"},
        {"type": "command", "command": "/old/agent-notify handle-codex-hook"}
      ]},
      {"hooks": [{"type": "command", "command": "/old/agent-notify handle-codex-hook"}]},
      {"hooks": [{"type": "command", "command": "echo independent-user-stop"}]}
    ],
    "PreToolUse": [
      {"matcher": "^shell$", "hooks": [{"type": "command", "command": "echo user-shell"}]}
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
		t.Fatalf("Stop entry count = %d, want 2 user groups", len(stopEntries))
	}

	commands := collectCommandsForTest(stopEntries)
	for _, want := range []string{"echo mixed-user-stop", "echo independent-user-stop"} {
		if !containsString(commands, want) {
			t.Fatalf("user hook command %q lost: %v", want, commands)
		}
	}
	if containsSubstring(commands, hookCommandMarker) {
		t.Fatalf("legacy managed Stop still present: %v", commands)
	}
	mixed := stopEntries[0].(map[string]any)
	if mixed["matcher"] != "ignored-by-codex" || mixed["statusMessage"] != "keep me" {
		t.Fatalf("mixed user Stop metadata changed: %#v", mixed)
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
    ],
    "PreToolUse": [
      {"matcher": "^shell$", "hooks": [{"type": "command", "command": "echo user-shell"}]}
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
	if err := Uninstall(path); err != nil {
		t.Fatalf("second Uninstall() error = %v", err)
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

	preToolEntries, ok := hooks["PreToolUse"].([]any)
	if !ok || len(preToolEntries) != 1 {
		t.Fatalf("PreToolUse should retain 1 user hook entry, got %v", hooks["PreToolUse"])
	}
	preToolEntry := preToolEntries[0].(map[string]any)
	if preToolEntry["matcher"] != "^shell$" {
		t.Fatalf("custom PreToolUse matcher changed to %v", preToolEntry["matcher"])
	}
	if commands := collectCommandsForTest(preToolEntries); !containsString(commands, "echo user-shell") {
		t.Fatalf("custom PreToolUse hook lost after uninstall: %v", commands)
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
