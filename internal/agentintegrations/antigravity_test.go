package agentintegrations

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hellolib/agent-notify/internal/testutil"
)

func TestAntigravityIntegration_NameAndSettingsPath(t *testing.T) {
	home := testutil.IsolateHome(t)
	ag := NewAntigravityIntegration()

	if ag.Name() != "Antigravity" {
		t.Fatalf("Name() = %q, want Antigravity", ag.Name())
	}

	userPath, err := ag.SettingsPath("user")
	if err != nil {
		t.Fatalf("SettingsPath(user) error = %v", err)
	}
	wantUser := filepath.Join(home, ".gemini", "config", "hooks.json")
	if userPath != wantUser {
		t.Errorf("SettingsPath(user) = %q, want %q", userPath, wantUser)
	}

	projectPath, err := ag.SettingsPath("project")
	if err != nil {
		t.Fatalf("SettingsPath(project) error = %v", err)
	}
	wantProject := filepath.Join(".agents", "hooks.json")
	if projectPath != wantProject {
		t.Errorf("SettingsPath(project) = %q, want %q", projectPath, wantProject)
	}

	if _, err := ag.SettingsPath("invalid"); err == nil {
		t.Fatal("SettingsPath(invalid) error = nil, want error")
	}
}

func TestAntigravityIntegration_DetectInstalled(t *testing.T) {
	home := testutil.IsolateHome(t)
	t.Setenv("PATH", t.TempDir()) // 空 PATH，排除宿主机上的 agy/antigravity 二进制

	ag := NewAntigravityIntegration()
	if ag.DetectInstalled() {
		t.Fatal("DetectInstalled() = true on empty HOME/PATH, want false")
	}

	// 仅存在 ~/.gemini 不应误判为已安装 Antigravity（其它 Gemini CLI 也共用 ~/.gemini）。
	if err := os.MkdirAll(filepath.Join(home, ".gemini"), 0o755); err != nil {
		t.Fatal(err)
	}
	if ag.DetectInstalled() {
		t.Fatal("DetectInstalled() = true with only ~/.gemini, want false")
	}

	// 存在 ~/.gemini/antigravity-cli 目录时应识别为已安装。
	if err := os.MkdirAll(filepath.Join(home, ".gemini", "antigravity-cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !ag.DetectInstalled() {
		t.Fatal("DetectInstalled() = false with ~/.gemini/antigravity-cli, want true")
	}
}

func TestAntigravityIntegration_InstallRoundTrip(t *testing.T) {
	ag := NewAntigravityIntegration()
	path := filepath.Join(t.TempDir(), "hooks.json")

	installed, err := ag.IsHookInstalled(path)
	if err != nil || installed {
		t.Fatalf("initial IsHookInstalled() = %v, %v; want false, nil", installed, err)
	}

	if err := ag.Install(path, "/usr/local/bin/agent-notify"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	installed, err = ag.IsHookInstalled(path)
	if err != nil || !installed {
		t.Fatalf("after Install IsHookInstalled() = %v, %v; want true, nil", installed, err)
	}

	if err := ag.Uninstall(path); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	installed, err = ag.IsHookInstalled(path)
	if err != nil || installed {
		t.Fatalf("after Uninstall IsHookInstalled() = %v, %v; want false, nil", installed, err)
	}
}
