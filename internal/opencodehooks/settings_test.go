package opencodehooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallCreatesPluginAndConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	pluginPath := filepath.Join(dir, "opencode-plugin.js")
	binary := "/tmp/agent-notify"

	if err := Install(configPath, pluginPath, binary); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	// 插件 JS 文件应存在
	if _, err := os.Stat(pluginPath); err != nil {
		t.Fatalf("plugin file not created: %v", err)
	}

	// 配置文件应包含 plugin 数组
	got := readJSON(t, configPath)
	plugins, ok := got["plugin"].([]any)
	if !ok || len(plugins) != 1 {
		t.Fatalf("plugin array = %v, want 1 entry", got["plugin"])
	}
	if plugins[0] != pluginPath {
		t.Fatalf("plugin[0] = %v, want %s", plugins[0], pluginPath)
	}
}

func TestInstallPreservesUserPlugins(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	pluginPath := filepath.Join(dir, "opencode-plugin.js")
	existing := `{
  "model": "gpt-4",
  "plugin": ["~/.config/opencode/plugins/my-plugin.js"]
}`
	if err := os.WriteFile(configPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Install(configPath, pluginPath, "/tmp/agent-notify"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)
	if !strings.Contains(s, "my-plugin.js") {
		t.Fatalf("user plugin lost after install: %s", s)
	}
	if !strings.Contains(s, "opencode-plugin.js") {
		t.Fatalf("agent-notify plugin missing after install: %s", s)
	}
	if !strings.Contains(s, "model") {
		t.Fatalf("user key lost after install: %s", s)
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	pluginPath := filepath.Join(dir, "opencode-plugin.js")

	if err := Install(configPath, pluginPath, "/tmp/agent-notify"); err != nil {
		t.Fatal(err)
	}
	if err := Install(configPath, pluginPath, "/tmp/agent-notify"); err != nil {
		t.Fatal(err)
	}

	got := readJSON(t, configPath)
	plugins, ok := got["plugin"].([]any)
	if !ok || len(plugins) != 1 {
		t.Fatalf("plugin entries = %v, want 1 after double install", got["plugin"])
	}
}

func TestIsInstalled(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	// IsInstalled 依赖路径中包含 "agent-notify" marker
	pluginPath := filepath.Join(dir, ".agent-notify", "opencode-plugin.js")

	if err := Install(configPath, pluginPath, "/tmp/agent-notify"); err != nil {
		t.Fatal(err)
	}
	installed, err := IsInstalled(configPath)
	if err != nil {
		t.Fatalf("IsInstalled() error = %v", err)
	}
	if !installed {
		t.Fatal("IsInstalled() = false, want true")
	}
}

func TestUninstallRemovesPlugin(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	pluginPath := filepath.Join(dir, "opencode-plugin.js")

	if err := Install(configPath, pluginPath, "/tmp/agent-notify"); err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(configPath, pluginPath); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}

	// 插件 JS 文件应被删除
	if _, err := os.Stat(pluginPath); !os.IsNotExist(err) {
		t.Fatalf("expected plugin file removed, stat err = %v", err)
	}

	// 配置中 plugin 键应被移除（因为仅含本插件条目）
	got := readJSON(t, configPath)
	if _, ok := got["plugin"]; ok {
		t.Fatalf("plugin key should be removed, got: %v", got["plugin"])
	}
}

func TestUninstallPreservesUserPlugins(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	pluginPath := filepath.Join(dir, "opencode-plugin.js")
	pluginPathJSON, err := json.Marshal(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	existing := `{
  "plugin": ["~/.config/opencode/plugins/my-plugin.js", ` + string(pluginPathJSON) + `]
}`
	if err := os.WriteFile(configPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	// 先写插件文件以便 Uninstall 能删除它
	if err := os.WriteFile(pluginPath, []byte("// stub"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Uninstall(configPath, pluginPath); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)
	if !strings.Contains(s, "my-plugin.js") {
		t.Fatalf("user plugin lost after uninstall: %s", s)
	}
	if strings.Contains(s, "opencode-plugin") {
		t.Fatalf("agent-notify plugin still present after uninstall: %s", s)
	}
}

func TestWritePluginFileBakesBinaryPath(t *testing.T) {
	dir := t.TempDir()
	pluginPath := filepath.Join(dir, "sub", "opencode-plugin.js")

	if err := WritePluginFile(pluginPath, "/custom/bin/agent-notify"); err != nil {
		t.Fatalf("WritePluginFile() error = %v", err)
	}

	data, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "/custom/bin/agent-notify") {
		t.Fatalf("plugin JS does not contain baked binary path: %s", s)
	}
}

func readJSON(t *testing.T, path string) map[string]any {
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

// 文件不存在时不得创建：用户可能刚卸载过，写回等于让集成复活。
func TestRefreshPluginIfStaleSkipsMissingFile(t *testing.T) {
	pluginPath := filepath.Join(t.TempDir(), "opencode-plugin.js")

	refreshed, err := RefreshPluginIfStale(pluginPath)
	if err != nil {
		t.Fatalf("RefreshPluginIfStale() error = %v", err)
	}
	if refreshed {
		t.Fatal("RefreshPluginIfStale() = true, want false for missing file")
	}
	if _, err := os.Stat(pluginPath); !os.IsNotExist(err) {
		t.Fatal("plugin file was created, want it left absent")
	}
}

// 内容一致时不应写盘，避免每次交互命令都刷新 mtime。
func TestRefreshPluginIfStaleNoopWhenCurrent(t *testing.T) {
	pluginPath := filepath.Join(t.TempDir(), "opencode-plugin.js")
	binary := "/tmp/agent-notify"
	if err := WritePluginFile(pluginPath, binary); err != nil {
		t.Fatalf("WritePluginFile() error = %v", err)
	}
	before, err := os.Stat(pluginPath)
	if err != nil {
		t.Fatal(err)
	}

	refreshed, err := RefreshPluginIfStale(pluginPath)
	if err != nil {
		t.Fatalf("RefreshPluginIfStale() error = %v", err)
	}
	if refreshed {
		t.Fatal("RefreshPluginIfStale() = true, want false for up-to-date file")
	}
	after, err := os.Stat(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("plugin file was rewritten despite identical content")
	}
}

// 核心场景：二进制升级后磁盘上还是旧插件（缺 question.asked 订阅）。
func TestRefreshPluginIfStaleRewritesOutdated(t *testing.T) {
	pluginPath := filepath.Join(t.TempDir(), "opencode-plugin.js")
	binary := "/tmp/agent-notify"
	stale := strings.Replace(string(renderPlugin(binary)), "\n      \"question.asked\",", "", 1)
	if err := os.WriteFile(pluginPath, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	refreshed, err := RefreshPluginIfStale(pluginPath)
	if err != nil {
		t.Fatalf("RefreshPluginIfStale() error = %v", err)
	}
	if !refreshed {
		t.Fatal("RefreshPluginIfStale() = false, want true for outdated plugin")
	}
	got, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(renderPlugin(binary)) {
		t.Fatal("plugin file content does not match embedded version after refresh")
	}
	if !strings.Contains(string(got), "question.asked") {
		t.Fatal("refreshed plugin still missing question.asked subscription")
	}
}

// 自愈只更新 JS 逻辑，绝不改动已烘焙的二进制路径——否则 dev 构建或 go test
// 的临时二进制会把用户的插件劫持到一个转瞬即逝的路径上。
func TestRefreshPluginIfStalePreservesBakedBinaryPath(t *testing.T) {
	pluginPath := filepath.Join(t.TempDir(), "opencode-plugin.js")
	installed := "/Users/demo/.agent-notify/agent-notify"
	stale := strings.Replace(string(renderPlugin(installed)), "\n      \"question.asked\",", "", 1)
	if err := os.WriteFile(pluginPath, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	refreshed, err := RefreshPluginIfStale(pluginPath)
	if err != nil {
		t.Fatalf("RefreshPluginIfStale() error = %v", err)
	}
	if !refreshed {
		t.Fatal("RefreshPluginIfStale() = false, want true for outdated plugin")
	}
	got, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), installed) {
		t.Fatalf("baked binary path was not preserved; content = %q", string(got))
	}
	if string(got) != string(renderPlugin(installed)) {
		t.Fatal("refreshed content does not match embedded version rendered with the original path")
	}
}

// 认不出烘焙路径时不猜测、不覆盖，交给向导处理。
func TestRefreshPluginIfStaleSkipsUnrecognizableFile(t *testing.T) {
	pluginPath := filepath.Join(t.TempDir(), "opencode-plugin.js")
	garbage := "// hand-written by the user\nexport const server = () => ({});\n"
	if err := os.WriteFile(pluginPath, []byte(garbage), 0o644); err != nil {
		t.Fatal(err)
	}

	refreshed, err := RefreshPluginIfStale(pluginPath)
	if err != nil {
		t.Fatalf("RefreshPluginIfStale() error = %v", err)
	}
	if refreshed {
		t.Fatal("RefreshPluginIfStale() = true, want false for unrecognizable file")
	}
	got, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != garbage {
		t.Fatal("unrecognizable file was overwritten, want it left untouched")
	}
}

// Windows 回归：旧版在 core.autocrlf=true 的 Windows 上写出的插件文件是 CRLF
// 行尾。binaryConstRe 必须容忍 \r，否则认不出烘焙路径、自愈静默失效——这正是
// CI 上 Windows job 红掉的两个用例的根因（go:embed 的 JS 被 git 转成 CRLF）。
func TestRefreshPluginIfStaleHandlesCRLFLineEndings(t *testing.T) {
	pluginPath := filepath.Join(t.TempDir(), "opencode-plugin.js")
	installed := `C:\Users\demo\.agent-notify\agent-notify.exe`
	// 用 renderPlugin 生成内容，再把 LF 全部换成 CRLF，模拟旧 Windows 落盘的文件。
	stale := strings.ReplaceAll(string(renderPlugin(installed)), "\n", "\r\n")
	if err := os.WriteFile(pluginPath, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	refreshed, err := RefreshPluginIfStale(pluginPath)
	if err != nil {
		t.Fatalf("RefreshPluginIfStale() error = %v", err)
	}
	if !refreshed {
		t.Fatal("RefreshPluginIfStale() = false, want true for CRLF plugin file (regex failed to tolerate \\r)")
	}
	got, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), installed) {
		t.Fatalf("baked binary path was not preserved; content = %q", string(got))
	}
	if string(got) != string(renderPlugin(installed)) {
		t.Fatal("refreshed content does not match embedded version rendered with the original path")
	}
}
