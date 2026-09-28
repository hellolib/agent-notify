package agentintegrations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDshIntegration_Name(t *testing.T) {
	d := NewDshIntegration()
	if got := d.Name(); got != "DeepSeek Harness" {
		t.Errorf("Name() = %q, want DeepSeek Harness", got)
	}
}

// TestDshIntegration_ImplementsInterface 是编译期断言的运行时伴侣：
// 确认它可被当作 Integration 使用（同时挡住未来签名漂移）。
func TestDshIntegration_ImplementsInterface(t *testing.T) {
	var i Integration = NewDshIntegration()
	if i == nil {
		t.Fatal("DshIntegration does not satisfy Integration")
	}
}

// TestDshIntegration_SettingsPath 断言注册落点是 profile 的 package.json。
func TestDshIntegration_SettingsPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DSH_HOME", home)
	t.Setenv("AGENT_NOTIFY_DSH_PROFILE", "web")

	got, err := NewDshIntegration().SettingsPath("user")
	if err != nil {
		t.Fatalf("SettingsPath(user) error = %v", err)
	}
	want := filepath.Join(home, "profiles", "web", "package.json")
	if got != want {
		t.Errorf("SettingsPath(user) = %q, want %q", got, want)
	}
}

// TestDshIntegration_SettingsPathProjectScope 断言 project scope 报错：
// DSH 的插件装在 profile 下，没有项目级落点。
func TestDshIntegration_SettingsPathProjectScope(t *testing.T) {
	if _, err := NewDshIntegration().SettingsPath("project"); err == nil {
		t.Fatal("SettingsPath(project) error = nil, want an error")
	}
}

// TestDshIntegration_IsHookInstalled 断言「已安装」就是 bundles 里含包名。
func TestDshIntegration_IsHookInstalled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "package.json")

	if err := os.WriteFile(path, []byte(`{"dsh":{"profile":{"bundles":["agent-notify-dsh"]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := NewDshIntegration().IsHookInstalled(path)
	if err != nil {
		t.Fatalf("IsHookInstalled() error = %v", err)
	}
	if !got {
		t.Error("IsHookInstalled() = false, want true when the bundle is listed")
	}
}

// TestDshIntegration_IsHookInstalledAbsent 断言未注册时为 false 且不报错。
func TestDshIntegration_IsHookInstalledAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "package.json")

	if err := os.WriteFile(path, []byte(`{"dsh":{"profile":{"bundles":["@deepseek-ai/dsh-base"]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := NewDshIntegration().IsHookInstalled(path)
	if err != nil {
		t.Fatalf("IsHookInstalled() error = %v", err)
	}
	if got {
		t.Error("IsHookInstalled() = true, want false when the bundle is absent")
	}
}

// TestDshIntegration_DetectInstalled 只断言不 panic：真实结果取决于本机是否
// 装了 DSH（与既有 Claude/Codex 的同类测试一致）。
func TestDshIntegration_DetectInstalled(t *testing.T) {
	_ = NewDshIntegration().DetectInstalled()
}

// TestDshIntegration_DetectInstalledViaEnvOverride 断言显式指定 dsh 路径即视为
// 已安装 —— 这覆盖了「从源码 checkout 启动、dsh 不在 PATH」的形态。
func TestDshIntegration_DetectInstalledViaEnvOverride(t *testing.T) {
	t.Setenv("AGENT_NOTIFY_DSH_BIN", "/tmp/somewhere/dsh")
	t.Setenv("DSH_HOME", filepath.Join(t.TempDir(), "does-not-exist"))

	if !NewDshIntegration().DetectInstalled() {
		t.Error("DetectInstalled() = false, want true when AGENT_NOTIFY_DSH_BIN is set")
	}
}

// TestDshIntegration_DetectInstalledViaHome 断言 DSH 家目录存在即视为已安装，
// 覆盖「装过但当前不在 PATH」。
func TestDshIntegration_DetectInstalledViaHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("DSH_HOME", home)
	t.Setenv("AGENT_NOTIFY_DSH_BIN", "")
	// 让 PATH 查找确定失败。
	t.Setenv("PATH", filepath.Join(t.TempDir(), "empty-bin"))

	if !NewDshIntegration().DetectInstalled() {
		t.Error("DetectInstalled() = false, want true when the DSH home exists")
	}
}
