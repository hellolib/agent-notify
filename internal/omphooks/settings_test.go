package omphooks

import (
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
