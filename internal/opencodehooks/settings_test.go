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
