package claudehooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
		if entryHooks[0]["command"] != `"/tmp/agent-notify" handle-claude-hook` {
			t.Fatalf(`%s command = %v, want "/tmp/agent-notify" handle-claude-hook`, event, entryHooks[0]["command"])
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

// TestInstallBacksUpExistingSettings 改写已有 settings.json 前应落 .bak 备份,
// 逻辑写坏时用户有恢复路径(issue #29)。
func TestInstallBacksUpExistingSettings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	original := `{"theme":"dark"}`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("install error = %v", err)
	}

	bak, err := os.ReadFile(path + common.BackupSuffix)
	if err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	if string(bak) != original {
		t.Fatalf("backup = %q, want original content", bak)
	}
}

// issue #34:重装时应把过期的二进制路径同步为当前路径,
// 而不是因 marker 命中而原样保留失效命令。
func TestInstallRefreshesStaleBinaryPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	if err := Install(path, "/old/build/agent-notify"); err != nil {
		t.Fatalf("first install error = %v", err)
	}
	if err := Install(path, "/new/npx/agent-notify"); err != nil {
		t.Fatalf("second install error = %v", err)
	}

	got := readSettingsForTest(t, path)
	hooks := got["hooks"].(map[string]any)
	for _, event := range managedEvents {
		entries := hooks[event].([]any)
		total := 0
		for _, e := range entries {
			entryMap := e.(map[string]any)
			for _, h := range entryMap["hooks"].([]any) {
				hm := h.(map[string]any)
				cmd, _ := hm["command"].(string)
				if !strings.Contains(cmd, hookCommandMarker) {
					continue
				}
				total++
				want := `"/new/npx/agent-notify" ` + hookCommandMarker
				if cmd != want {
					t.Fatalf("%s command = %q, want %q", event, cmd, want)
				}
			}
		}
		if total != 1 {
			t.Fatalf("%s has %d managed hooks, want 1 (no duplicates)", event, total)
		}
	}
}

// TestInstallPreservesKeyOrderAndNumericPrecision 是 issue #39 第 3 项的回归测试。
// map[string]any 往返会把顶层键重排成字母序、把 >2^53 的整数改值、
// 把大数变成科学计数法、把 1.10 抹成 1.1——dotfiles 用户每次安装都拿到全文件 diff。
func TestInstallPreservesKeyOrderAndNumericPrecision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	// 顶层键刻意逆字母序,且带上真实 settings.json 里会出现的数值字段
	original := `{
  "theme": "dark",
  "statusLine": {"type": "command"},
  "maxTokens": 9007199254740993,
  "huge": 123456789012345678901234,
  "keep": 1.10,
  "env": {"ZZZ": "last", "AAA": "first"},
  "attribution": {"enabled": false}
}`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)

	// 顶层键序:原顺序保持,新增的 hooks 追加到末尾
	wantOrder := []string{"theme", "statusLine", "maxTokens", "huge", "keep", "env", "attribution", "hooks"}
	prev := -1
	for _, key := range wantOrder {
		idx := strings.Index(got, `"`+key+`"`)
		if idx < 0 {
			t.Fatalf("key %q missing from output:\n%s", key, got)
		}
		if idx < prev {
			t.Fatalf("key %q moved out of order:\n%s", key, got)
		}
		prev = idx
	}

	// 数值逐字节保留
	for _, literal := range []string{"9007199254740993", "123456789012345678901234", "1.10"} {
		if !strings.Contains(got, literal) {
			t.Fatalf("numeric literal %s was rewritten:\n%s", literal, got)
		}
	}

	// 未触碰的子树内部键序也保留
	zzz := strings.Index(got, `"ZZZ"`)
	aaa := strings.Index(got, `"AAA"`)
	if zzz < 0 || aaa < 0 || zzz > aaa {
		t.Fatalf("env 内部键序被重排:\n%s", got)
	}
}

// TestInstallLeavesUserHookEntryByteIdentical 用户手写的 entry(含 matcher、
// 自定义键、自己的缩进)在安装后应当一个字节都没变。
func TestInstallLeavesUserHookEntryByteIdentical(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	original := `{
  "hooks": {
    "UserPromptSubmit": [
      {"matcher": "*", "note": "mine", "hooks": [{"type": "command", "command": "echo hi"}]}
    ],
    "Stop": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "echo user-stop"}]}
    ]
  }
}`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	data, _ := os.ReadFile(path)
	got := string(data)

	// 用户 entry 的键序 matcher/note/hooks 必须原样,不能被重排成 hooks/matcher/note
	for _, want := range []string{
		`"matcher": "*"`,
		`"note": "mine"`,
		`"matcher": "Bash"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("user entry was rewritten, %s missing:\n%s", want, got)
		}
	}

	// 用户手写的 UserPromptSubmit 事件不该被重排到 SessionStart 之后的字母序位置
	ups := strings.Index(got, `"UserPromptSubmit"`)
	stop := strings.Index(got, `"Stop"`)
	if ups < 0 || stop < 0 || ups > stop {
		t.Fatalf("hooks 内的用户事件顺序被重排:\n%s", got)
	}
}

// TestInstallRefusesNonArrayHookValue 是 issue #39 第 6 项的回归测试。
// 旧实现里 common.ToAnySlice 对非数组返回 nil,Install 据此认为「这个事件下
// 什么都没有」而整个替换掉——用户手写成对象形式的 hook 定义无声消失。
func TestInstallRefusesNonArrayHookValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
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
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(`{"hooks":{"Stop":{"mine":true},"Notification":[{"hooks":[{"type":"command","command":"/x handle-claude-hook"}]}]}}`), 0o644); err != nil {
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
