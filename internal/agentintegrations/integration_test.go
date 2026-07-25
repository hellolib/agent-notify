package agentintegrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeIntegration_Name(t *testing.T) {
	c := NewClaudeIntegration()
	if got := c.Name(); got != "Claude Code" {
		t.Errorf("ClaudeIntegration.Name() = %q, want %q", got, "Claude Code")
	}
}

func TestCodexIntegration_Name(t *testing.T) {
	c := NewCodexIntegration()
	if got := c.Name(); got != "Codex" {
		t.Errorf("CodexIntegration.Name() = %q, want %q", got, "Codex")
	}
}

func TestClaudeIntegration_SettingsPath(t *testing.T) {
	c := NewClaudeIntegration()

	t.Run("user scope", func(t *testing.T) {
		path, err := c.SettingsPath("user")
		if err != nil {
			t.Fatalf("SettingsPath(user) error: %v", err)
		}
		home, _ := os.UserHomeDir()
		expected := filepath.Join(home, ".claude", "settings.json")
		if path != expected {
			t.Errorf("SettingsPath(user) = %q, want %q", path, expected)
		}
	})

	t.Run("project scope", func(t *testing.T) {
		path, err := c.SettingsPath("project")
		if err != nil {
			t.Fatalf("SettingsPath(project) error: %v", err)
		}
		expected := filepath.Join(".claude", "settings.json")
		if path != expected {
			t.Errorf("SettingsPath(project) = %q, want %q", path, expected)
		}
	})

	t.Run("invalid scope", func(t *testing.T) {
		_, err := c.SettingsPath("invalid")
		if err == nil {
			t.Error("SettingsPath(invalid) expected error, got nil")
		}
	})
}

func TestCodexIntegration_SettingsPath(t *testing.T) {
	c := NewCodexIntegration()

	t.Run("user scope", func(t *testing.T) {
		path, err := c.SettingsPath("user")
		if err != nil {
			t.Fatalf("SettingsPath(user) error: %v", err)
		}
		home, _ := os.UserHomeDir()
		expected := filepath.Join(home, ".codex", "hooks.json")
		if path != expected {
			t.Errorf("SettingsPath(user) = %q, want %q", path, expected)
		}
	})

	t.Run("project scope", func(t *testing.T) {
		path, err := c.SettingsPath("project")
		if err != nil {
			t.Fatalf("SettingsPath(project) error: %v", err)
		}
		expected := filepath.Join(".codex", "hooks.json")
		if path != expected {
			t.Errorf("SettingsPath(project) = %q, want %q", path, expected)
		}
	})

	t.Run("invalid scope", func(t *testing.T) {
		_, err := c.SettingsPath("invalid")
		if err == nil {
			t.Error("SettingsPath(invalid) expected error, got nil")
		}
	})
}

func TestClaudeIntegration_Install(t *testing.T) {
	c := NewClaudeIntegration()

	t.Run("creates settings file with hooks", func(t *testing.T) {
		tmpDir := t.TempDir()
		settingsPath := filepath.Join(tmpDir, ".claude", "settings.json")

		err := c.Install(settingsPath, "/usr/local/bin/agent-notify")
		if err != nil {
			t.Fatalf("Install() error: %v", err)
		}

		// Verify file exists
		if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
			t.Fatalf("settings.json not created at %q", settingsPath)
		}

		// Verify hooks are installed
		installed, err := c.IsHookInstalled(settingsPath)
		if err != nil {
			t.Fatalf("IsHookInstalled() error: %v", err)
		}
		if !installed {
			t.Error("IsHookInstalled() = false, want true")
		}
	})

	t.Run("preserves existing settings", func(t *testing.T) {
		tmpDir := t.TempDir()
		settingsPath := filepath.Join(tmpDir, "settings.json")

		// Create existing settings
		existingSettings := `{"apiKey": "test-key", "theme": "dark"}`
		if err := os.WriteFile(settingsPath, []byte(existingSettings), 0o644); err != nil {
			t.Fatalf("failed to write existing settings: %v", err)
		}

		err := c.Install(settingsPath, "/usr/local/bin/agent-notify")
		if err != nil {
			t.Fatalf("Install() error: %v", err)
		}

		// Read the file and verify both old and new keys exist
		data, err := os.ReadFile(settingsPath)
		if err != nil {
			t.Fatalf("failed to read settings: %v", err)
		}

		content := string(data)
		if !containsAll(content, `"apiKey"`, `"theme"`, `"hooks"`) {
			t.Errorf("settings.json should contain apiKey, theme, and hooks, got:\n%s", content)
		}
	})
}

func TestCodexIntegration_Install(t *testing.T) {
	c := NewCodexIntegration()

	t.Run("creates hooks.json with codex hook", func(t *testing.T) {
		tmpDir := t.TempDir()
		settingsPath := filepath.Join(tmpDir, ".codex", "hooks.json")

		err := c.Install(settingsPath, "/usr/local/bin/agent-notify")
		if err != nil {
			t.Fatalf("Install() error: %v", err)
		}

		// Verify file exists
		if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
			t.Fatalf("hooks.json not created at %q", settingsPath)
		}

		// Verify hook is installed
		installed, err := c.IsHookInstalled(settingsPath)
		if err != nil {
			t.Fatalf("IsHookInstalled() error: %v", err)
		}
		if !installed {
			t.Error("IsHookInstalled() = false, want true")
		}
	})

	t.Run("preserves existing hooks.json keys", func(t *testing.T) {
		tmpDir := t.TempDir()
		settingsPath := filepath.Join(tmpDir, "hooks.json")

		// Pre-existing user content next to hooks key
		existing := `{"someUserKey":"value","hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"echo hi"}]}]}}`
		if err := os.WriteFile(settingsPath, []byte(existing), 0o644); err != nil {
			t.Fatalf("failed to write existing hooks.json: %v", err)
		}

		err := c.Install(settingsPath, "/usr/local/bin/agent-notify")
		if err != nil {
			t.Fatalf("Install() error: %v", err)
		}

		data, err := os.ReadFile(settingsPath)
		if err != nil {
			t.Fatalf("failed to read hooks.json: %v", err)
		}

		content := string(data)
		// user content preserved, our hooks added
		if !containsAll(content, `"someUserKey"`, `"SessionStart"`, `"PermissionRequest"`, `"PreToolUse"`, `handle-codex-hook`) {
			t.Errorf("hooks.json missing expected keys, got:\n%s", content)
		}
		if strings.Contains(content, `"Stop"`) {
			t.Errorf("hooks.json should not register Codex Stop, got:\n%s", content)
		}
	})

	t.Run("subscribes to request_user_input with an exact matcher", func(t *testing.T) {
		tmpDir := t.TempDir()
		settingsPath := filepath.Join(tmpDir, "hooks.json")

		if err := c.Install(settingsPath, "/usr/local/bin/agent-notify"); err != nil {
			t.Fatalf("Install() error: %v", err)
		}

		data, _ := os.ReadFile(settingsPath)
		content := string(data)

		if !containsAll(content, `"PermissionRequest"`, `"PreToolUse"`, `"matcher": "^request_user_input$"`) {
			t.Errorf("hooks.json should register PermissionRequest and matched PreToolUse, got:\n%s", content)
		}
		if strings.Contains(content, `"Stop"`) {
			t.Errorf("hooks.json should not register Codex Stop, got:\n%s", content)
		}
		// must NOT register events Codex doesn't support
		for _, unsupported := range []string{`"Notification"`, `"PostToolUseFailure"`} {
			if containsAll(content, unsupported) {
				t.Errorf("hooks.json should not register %s for Codex, got:\n%s", unsupported, content)
			}
		}
	})
}

func TestClaudeIntegration_DetectInstalled(t *testing.T) {
	c := NewClaudeIntegration()
	// This test just verifies the method doesn't panic
	// The actual result depends on whether claude is installed
	_ = c.DetectInstalled()
}

func TestCodexIntegration_DetectInstalled(t *testing.T) {
	c := NewCodexIntegration()
	// This test just verifies the method doesn't panic
	// The actual result depends on whether codex is installed
	_ = c.DetectInstalled()
}

func containsAll(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if !contains(s, substr) {
			return false
		}
	}
	return true
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
