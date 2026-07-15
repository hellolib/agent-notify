package agentintegrations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrokIntegration_Name(t *testing.T) {
	g := NewGrokIntegration()
	if g.Name() != "Grok" {
		t.Errorf("Name() = %q, want Grok", g.Name())
	}
}

func TestGrokIntegration_SettingsPath(t *testing.T) {
	g := NewGrokIntegration()

	t.Run("user", func(t *testing.T) {
		path, err := g.SettingsPath("user")
		if err != nil {
			t.Fatal(err)
		}
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		expected := filepath.Join(home, ".grok", "hooks", "agent-notify.json")
		if path != expected {
			t.Errorf("SettingsPath(user) = %q, want %q", path, expected)
		}
	})

	t.Run("project", func(t *testing.T) {
		path, err := g.SettingsPath("project")
		if err != nil {
			t.Fatal(err)
		}
		expected := filepath.Join(".grok", "hooks", "agent-notify.json")
		if path != expected {
			t.Errorf("SettingsPath(project) = %q, want %q", path, expected)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		if _, err := g.SettingsPath("other"); err == nil {
			t.Fatal("expected error for invalid scope")
		}
	})
}

func TestGrokIntegration_Install(t *testing.T) {
	g := NewGrokIntegration()

	t.Run("creates hooks file with grok hook", func(t *testing.T) {
		tmpDir := t.TempDir()
		settingsPath := filepath.Join(tmpDir, ".grok", "hooks", "agent-notify.json")

		if err := g.Install(settingsPath, "/tmp/agent-notify"); err != nil {
			t.Fatalf("Install() error = %v", err)
		}

		data, err := os.ReadFile(settingsPath)
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		for _, event := range []string{"SessionStart", "Notification", "Stop", "StopFailure", "PostToolUseFailure"} {
			if !strings.Contains(content, `"`+event+`"`) {
				t.Errorf("hooks file should register %s, got:\n%s", event, content)
			}
		}
		if !strings.Contains(content, "handle-grok-hook") {
			t.Errorf("hooks file should contain handle-grok-hook, got:\n%s", content)
		}

		installed, err := g.IsHookInstalled(settingsPath)
		if err != nil {
			t.Fatal(err)
		}
		if !installed {
			t.Fatal("IsHookInstalled() = false, want true")
		}
	})
}

func TestGrokIntegration_Uninstall(t *testing.T) {
	g := NewGrokIntegration()
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, ".grok", "hooks", "agent-notify.json")

	if err := g.Install(settingsPath, "/tmp/agent-notify"); err != nil {
		t.Fatal(err)
	}
	if err := g.Uninstall(settingsPath); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	installed, err := g.IsHookInstalled(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	if installed {
		t.Fatal("IsHookInstalled() = true after uninstall, want false")
	}
}

func TestGrokIntegration_DetectInstalled(t *testing.T) {
	g := NewGrokIntegration()
	// 只验证不 panic，实际结果取决于本机是否装了 Grok
	_ = g.DetectInstalled()
}
