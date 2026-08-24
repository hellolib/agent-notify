package codexhooks

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/common"
	toml "github.com/pelletier/go-toml/v2"
)

func TestEnsureNotifyCommandInstallsBeforeTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("model = \"gpt-5\"\n\n[features]\nhooks = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureNotifyCommand(path, `/tmp/agent-notify`); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	if !strings.Contains(text, `notify = [ "/tmp/agent-notify", "handle-codex-notify" ]`) {
		t.Fatalf("config = %q", text)
	}
	var parsed map[string]any
	if err := toml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("written TOML is invalid: %v", err)
	}
}

func TestEnsureNotifyCommandPreservesCustomNotify(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	want := "notify = [ \"python3\", \"custom.py\" ]\n"
	if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureNotifyCommand(path, `/tmp/agent-notify`); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != want {
		t.Fatalf("custom notify changed: %q", got)
	}
}

func TestEnsureNotifyCommandPreservesDesktopWrappedCustomNotify(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	want := `notify = [ "C:\\runtime\\codex-computer-use.exe", "turn-ended", "--previous-notify", "[\"python3\",\"custom.py\"]" ]` + "\n"
	if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureNotifyCommand(path, `/tmp/agent-notify`); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != want {
		t.Fatalf("wrapped custom notify changed: %q", got)
	}
}

func TestEnsureNotifyCommandReplacesCodexInternalAndManagedCommands(t *testing.T) {
	for _, initial := range []string{
		`notify = [ "C:\\runtime\\codex-computer-use.exe", "turn-ended" ]`,
		`notify = [ "C:\\runtime\\codex-computer-use.exe", "turn-ended", "--previous-notify", "[]" ]`,
		`notify = [ "/old/agent-notify", "handle-codex-notify" ]`,
	} {
		path := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(path, []byte(initial+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := EnsureNotifyCommand(path, `/new/agent-notify`); err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(path)
		if !strings.Contains(string(got), `/new/agent-notify`) || strings.Contains(string(got), "turn-ended") {
			t.Fatalf("config = %q", got)
		}
	}
}

func TestRemoveNotifyCommandOnlyRemovesManagedCommand(t *testing.T) {
	for _, tc := range []struct {
		name    string
		initial string
		removed bool
	}{
		{"managed", `notify = [ "/agent-notify", "handle-codex-notify" ]`, true},
		{"custom", `notify = [ "python3", "custom.py" ]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte(tc.initial+"\n[features]\nhooks = true\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := RemoveNotifyCommand(path); err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(path)
			contains := strings.Contains(string(got), "notify =")
			if contains == tc.removed {
				t.Fatalf("config = %q, removed=%t", got, tc.removed)
			}
		})
	}
}

func TestUninstallRemovesManagedNotifyWithoutHooksFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte(`notify = [ "/agent-notify", "handle-codex-notify" ]`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Uninstall(filepath.Join(dir, "hooks.json")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "handle-codex-notify") {
		t.Fatalf("managed notify remains after uninstall: %q", got)
	}
}

func TestRemoveNotifyCommandUnwrapsManagedDesktopPreviousNotify(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	initial := `notify = [ "C:\\runtime\\codex-computer-use.exe", "turn-ended", "--previous-notify", "[\"/agent-notify\",\"handle-codex-notify\"]" ]` + "\n"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := RemoveNotifyCommand(path); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "codex-computer-use.exe") || strings.Contains(string(got), notifyCommandMarker) || strings.Contains(string(got), "--previous-notify") {
		t.Fatalf("desktop notify was not cleanly unwrapped: %q", got)
	}
}

func TestInstallKeepsPreInstallConfigBackup(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")
	configPath := filepath.Join(dir, "config.toml")
	original := "model = \"gpt-5\"\n\n[features]\nhooks = false\n"
	if err := os.WriteFile(configPath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Install(hooksPath, "/agent-notify"); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(configPath + common.BackupSuffix)
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != original {
		t.Fatalf("backup = %q, want pre-install config %q", backup, original)
	}
	info, err := os.Stat(configPath + common.BackupSuffix)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); runtime.GOOS != "windows" && got != 0o600 {
		t.Fatalf("backup mode = %o, want 600", got)
	}
}
