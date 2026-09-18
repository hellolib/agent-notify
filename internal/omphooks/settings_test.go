package omphooks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsPathHonorsProfileAndOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Setenv("OMP_PROFILE", "work")
	path, err := SettingsPath("user")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".omp", "profiles", "work", "agent", "extensions", "agent-notify.ts")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}

	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(home, "custom-agent"))
	path, err = SettingsPath("user")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, "custom-agent", "extensions", "agent-notify.ts"); path != want {
		t.Fatalf("override path = %q, want %q", path, want)
	}
}

func TestInstallIsIdempotentAndUninstallPreservesUnownedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "extensions", "agent-notify.ts")
	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), pluginMarker) || !strings.Contains(string(data), "/tmp/agent-notify") {
		t.Fatalf("installed extension missing marker or binary: %s", data)
	}
	installed, err := IsInstalled(path)
	if err != nil || !installed {
		t.Fatalf("IsInstalled() = %v, %v", installed, err)
	}
	if err := Install(path, "/tmp/agent-notify"); err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("extension still exists after uninstall: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("export default () => {};"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("unowned extension was removed: %v", err)
	}
}

// 文件不存在时不得创建：用户可能刚卸载过，写回等于让集成复活。
func TestRefreshIfStaleSkipsMissingFile(t *testing.T) {
	extPath := filepath.Join(t.TempDir(), "extensions", "agent-notify.ts")
	refreshed, err := RefreshIfStale(extPath)
	if err != nil {
		t.Fatalf("RefreshIfStale() error = %v", err)
	}
	if refreshed {
		t.Fatal("RefreshIfStale() = true, want false for missing file")
	}
	if _, err := os.Stat(extPath); !os.IsNotExist(err) {
		t.Fatal("extension file was created, want it left absent")
	}
}

// 内容一致时不应写盘，避免每次交互命令都刷新 mtime。
func TestRefreshIfStaleNoopWhenCurrent(t *testing.T) {
	extPath := filepath.Join(t.TempDir(), "extensions", "agent-notify.ts")
	if err := Install(extPath, "/tmp/agent-notify"); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(extPath)
	if err != nil {
		t.Fatal(err)
	}
	refreshed, err := RefreshIfStale(extPath)
	if err != nil {
		t.Fatalf("RefreshIfStale() error = %v", err)
	}
	if refreshed {
		t.Fatal("RefreshIfStale() = true, want false for up-to-date file")
	}
	after, err := os.Stat(extPath)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("mtime changed: refresh wrote an unchanged file")
	}
}

// 核心场景：二进制升级后磁盘上还是旧扩展（缺 question_asked 订阅等）。
func TestRefreshIfStaleRewritesOutdated(t *testing.T) {
	extPath := filepath.Join(t.TempDir(), "extensions", "agent-notify.ts")
	binary := "/tmp/agent-notify"
	if err := Install(extPath, binary); err != nil {
		t.Fatal(err)
	}
	stale := strings.Replace(string(renderPlugin(binary)), "question_asked", "question_asked_stale", 1)
	if err := os.WriteFile(extPath, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	refreshed, err := RefreshIfStale(extPath)
	if err != nil {
		t.Fatalf("RefreshIfStale() error = %v", err)
	}
	if !refreshed {
		t.Fatal("RefreshIfStale() = false, want true for outdated extension")
	}
	got, err := os.ReadFile(extPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, renderPlugin(binary)) {
		t.Fatalf("extension not rewritten to the embedded version")
	}
}

// 认不出烘焙路径时不猜测、不覆盖，交给向导处理。
func TestRefreshIfStaleSkipsUnrecognizableFile(t *testing.T) {
	extPath := filepath.Join(t.TempDir(), "extensions", "agent-notify.ts")
	if err := os.MkdirAll(filepath.Dir(extPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(extPath, []byte("export default function agentNotify() {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	refreshed, err := RefreshIfStale(extPath)
	if err != nil {
		t.Fatalf("RefreshIfStale() error = %v", err)
	}
	if refreshed {
		t.Fatal("RefreshIfStale() = true, want false for unrecognizable file")
	}
	got, err := os.ReadFile(extPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("export default function agentNotify")) {
		t.Fatal("RefreshIfStale overwrote a file that is not agent-notify-owned")
	}
}

// 内容过时但烘焙路径不变时只更新逻辑，保留原二进制路径。
func TestRefreshIfStalePreservesBakedBinaryPath(t *testing.T) {
	extPath := filepath.Join(t.TempDir(), "extensions", "agent-notify.ts")
	installed := "/Users/demo/.agent-notify/agent-notify"
	if err := Install(extPath, installed); err != nil {
		t.Fatal(err)
	}
	stale := strings.Replace(string(renderPlugin(installed)), "question_asked", "question_asked_stale", 1)
	if err := os.WriteFile(extPath, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := RefreshIfStale(extPath); err != nil {
		t.Fatalf("RefreshIfStale() error = %v", err)
	}
	got, err := os.ReadFile(extPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte(installed)) {
		t.Fatalf("refresh dropped the baked binary path %q: %s", installed, got)
	}
}
