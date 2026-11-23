package antigravityhooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildHookSettingsShape(t *testing.T) {
	settings := BuildHookSettings("/usr/local/bin/agent-notify")
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var parsed map[string]map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	hookObj, ok := parsed[hookName]
	if !ok {
		t.Fatalf("missing top-level %q key in %s", hookName, string(data))
	}
	for _, ev := range []string{"SessionStart", "PreToolUse", "Stop"} {
		if _, ok := hookObj[ev]; !ok {
			t.Errorf("missing %q event in %q hook", ev, hookName)
		}
	}
	if !strings.Contains(string(data), "ask_question|ask_permission") {
		t.Errorf("expected PreToolUse matcher in %s", string(data))
	}
	if !strings.Contains(string(data), hookCommandMarker) {
		t.Errorf("expected command marker %q in %s", hookCommandMarker, string(data))
	}
}

func TestInstallPreservesUserHooksAndUpdatesStaleBinary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	existing := `{
  "lint-checker": {
    "PostToolUse": [
      {
        "matcher": "run_command",
        "hooks": [
          {
            "type": "command",
            "command": "./scripts/lint.sh",
            "timeout": 10
          }
        ]
      }
    ]
  }
}
`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Install(path, "/old/agent-notify"); err != nil {
		t.Fatalf("Install(old) error = %v", err)
	}
	installed, err := IsInstalled(path)
	if err != nil || !installed {
		t.Fatalf("IsInstalled() = %v, %v; want true, nil", installed, err)
	}

	// Reinstall with new binary path refreshes the command.
	if err := Install(path, "/new/agent-notify"); err != nil {
		t.Fatalf("Install(new) error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "lint-checker") || !strings.Contains(content, "./scripts/lint.sh") {
		t.Fatalf("user hook was not preserved:\n%s", content)
	}
	if strings.Contains(content, "/old/agent-notify") {
		t.Fatalf("stale binary path was not replaced:\n%s", content)
	}
	if !strings.Contains(content, "/new/agent-notify") {
		t.Fatalf("new binary path missing:\n%s", content)
	}
}

func TestInstallRefusesNonObjectAgentNotifyKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	if err := os.WriteFile(path, []byte(`{"agent-notify": ["invalid"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Install(path, "/usr/local/bin/agent-notify"); err == nil {
		t.Fatal("expected error when agent-notify key is not an object")
	}
}

func TestIsInstalledReturnsFalseWhenDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	disabled := `{
  "agent-notify": {
    "enabled": false,
    "Stop": [{"type": "command", "command": "/bin/agent-notify handle-antigravity-hook"}]
  }
}`
	if err := os.WriteFile(path, []byte(disabled), 0o644); err != nil {
		t.Fatal(err)
	}
	installed, err := IsInstalled(path)
	if err != nil {
		t.Fatalf("IsInstalled() error = %v", err)
	}
	if installed {
		t.Fatal("IsInstalled() = true for disabled hook, want false")
	}
}

func TestUninstallRemovesOnlyManagedHooksAndDeletesEmptyFile(t *testing.T) {
	t.Run("deletes file when only agent-notify existed", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hooks.json")
		if err := Install(path, "/usr/local/bin/agent-notify"); err != nil {
			t.Fatal(err)
		}
		if err := Uninstall(path); err != nil {
			t.Fatalf("Uninstall() error = %v", err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected empty hooks.json to be removed, stat err = %v", err)
		}
	})

	t.Run("preserves user hooks", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hooks.json")
		existing := `{"user-hook":{"Stop":[{"type":"command","command":"./done.sh"}]}}`
		if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Install(path, "/usr/local/bin/agent-notify"); err != nil {
			t.Fatal(err)
		}
		if err := Uninstall(path); err != nil {
			t.Fatalf("Uninstall() error = %v", err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("expected hooks.json to remain for user-hook: %v", err)
		}
		if strings.Contains(string(data), hookCommandMarker) {
			t.Fatalf("managed hook still present after Uninstall:\n%s", string(data))
		}
		if !strings.Contains(string(data), "user-hook") || !strings.Contains(string(data), "./done.sh") {
			t.Fatalf("user hook lost after Uninstall:\n%s", string(data))
		}
	})

	t.Run("no-op when file missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.json")
		if err := Uninstall(path); err != nil {
			t.Fatalf("Uninstall(missing) error = %v", err)
		}
	})
}
