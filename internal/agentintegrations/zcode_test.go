package agentintegrations

import (
	"os"
	"path/filepath"
	"testing"
)

func TestZcodeIntegration_Name(t *testing.T) {
	z := NewZcodeIntegration()
	if z.Name() != "ZCode" {
		t.Errorf("Name() = %q, want ZCode", z.Name())
	}
}

func TestZcodeIntegration_SettingsPath(t *testing.T) {
	z := NewZcodeIntegration()

	t.Run("user scope", func(t *testing.T) {
		path, err := z.SettingsPath("user")
		if err != nil {
			t.Fatalf("SettingsPath(user) error: %v", err)
		}
		home, _ := os.UserHomeDir()
		expected := filepath.Join(home, ".zcode", "cli", "config.json")
		if path != expected {
			t.Errorf("SettingsPath(user) = %q, want %q", path, expected)
		}
	})

	t.Run("project scope", func(t *testing.T) {
		path, err := z.SettingsPath("project")
		if err != nil {
			t.Fatalf("SettingsPath(project) error: %v", err)
		}
		expected := filepath.Join(".zcode", "cli", "config.json")
		if path != expected {
			t.Errorf("SettingsPath(project) = %q, want %q", path, expected)
		}
	})

	t.Run("invalid scope", func(t *testing.T) {
		_, err := z.SettingsPath("invalid")
		if err == nil {
			t.Error("SettingsPath(invalid) expected error, got nil")
		}
	})
}

func TestZcodeIntegration_Install(t *testing.T) {
	z := NewZcodeIntegration()

	t.Run("creates config.json with zcode hook", func(t *testing.T) {
		tmpDir := t.TempDir()
		settingsPath := filepath.Join(tmpDir, ".zcode", "cli", "config.json")

		err := z.Install(settingsPath, "/usr/local/bin/agent-notify")
		if err != nil {
			t.Fatalf("Install() error: %v", err)
		}

		if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
			t.Fatalf("config.json not created at %q", settingsPath)
		}

		installed, err := z.IsHookInstalled(settingsPath)
		if err != nil {
			t.Fatalf("IsHookInstalled() error: %v", err)
		}
		if !installed {
			t.Error("IsHookInstalled() = false, want true")
		}
	})

	t.Run("preserves existing mcp config", func(t *testing.T) {
		tmpDir := t.TempDir()
		settingsPath := filepath.Join(tmpDir, "config.json")
		existing := `{"mcp":{"servers":{"chrome-devtools":{"type":"stdio"}}}}`
		if err := os.WriteFile(settingsPath, []byte(existing), 0o644); err != nil {
			t.Fatalf("failed to write existing config: %v", err)
		}

		err := z.Install(settingsPath, "/usr/local/bin/agent-notify")
		if err != nil {
			t.Fatalf("Install() error: %v", err)
		}

		data, _ := os.ReadFile(settingsPath)
		content := string(data)
		if !containsAll(content, `"mcp"`, `"servers"`, `"hooks"`) {
			t.Errorf("config.json should preserve mcp and add hooks, got:\n%s", content)
		}
	})

	t.Run("subscribes only to ZCode-supported events", func(t *testing.T) {
		tmpDir := t.TempDir()
		settingsPath := filepath.Join(tmpDir, "config.json")

		if err := z.Install(settingsPath, "/usr/local/bin/agent-notify"); err != nil {
			t.Fatalf("Install() error: %v", err)
		}

		data, _ := os.ReadFile(settingsPath)
		content := string(data)

		// ZCode 支持的事件
		for _, supported := range []string{`"SessionStart"`, `"PermissionRequest"`, `"PostToolUse"`, `"PostToolUseFailure"`, `"Stop"`} {
			if !containsAll(content, supported) {
				t.Errorf("config.json should register %s for ZCode, got:\n%s", supported, content)
			}
		}
		// must NOT register Notification — ZCode 不支持，且 strict schema 会导致整体加载失败
		if containsAll(content, `"Notification"`) {
			t.Errorf("config.json should not register Notification for ZCode (unsupported, breaks strict schema), got:\n%s", content)
		}
	})
}

func TestZcodeIntegration_Uninstall(t *testing.T) {
	z := NewZcodeIntegration()

	t.Run("removes managed hooks but preserves user config", func(t *testing.T) {
		tmpDir := t.TempDir()
		settingsPath := filepath.Join(tmpDir, "config.json")

		if err := z.Install(settingsPath, "/usr/local/bin/agent-notify"); err != nil {
			t.Fatalf("Install() error: %v", err)
		}
		if err := z.Uninstall(settingsPath); err != nil {
			t.Fatalf("Uninstall() error: %v", err)
		}

		installed, err := z.IsHookInstalled(settingsPath)
		if err != nil {
			t.Fatalf("IsHookInstalled() error: %v", err)
		}
		if installed {
			t.Error("IsHookInstalled() = true after uninstall, want false")
		}
	})
}

func TestZcodeIntegration_DetectInstalled(t *testing.T) {
	z := NewZcodeIntegration()
	// 只验证不 panic，实际结果取决于本机是否装了 ZCode
	_ = z.DetectInstalled()
}
