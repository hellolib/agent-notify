package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/codexhooks"
)

// hook handler 必须走无头路径：它们由 agent 高频拉起，不能带额外副作用。
func TestIsHeadlessInvocation(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"无参数启动 TUI", nil, false},
		{"空切片", []string{}, false},
		{"init", []string{"init"}, false},
		{"doctor", []string{"doctor"}, false},
		{"test 带子命令", []string{"test", "feishu"}, false},
		{"opencode 安装", []string{"opencode", "--scope", "user"}, false},
		{"claude hook", []string{"handle-claude-hook"}, true},
		{"codex hook", []string{"handle-codex-hook"}, true},
		{"codex notify", []string{"handle-codex-notify", `{}`}, true},
		{"zcode hook", []string{"handle-zcode-hook"}, true},
		{"grok hook", []string{"handle-grok-hook"}, true},
		{"droid hook", []string{"handle-droid-hook"}, true},
		{"opencode hook", []string{"handle-opencode-hook"}, true},
		{"linux 点击回调", []string{"linux-notify-wait"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isHeadlessInvocation(tc.args); got != tc.want {
				t.Fatalf("isHeadlessInvocation(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestShouldRefreshCodexNotify(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"interactive", nil, true},
		{"doctor", []string{"doctor"}, true},
		{"codex hook", []string{"handle-codex-hook"}, true},
		{"codex notify", []string{"handle-codex-notify", `{}`}, false},
		{"other hook", []string{"handle-claude-hook"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRefreshCodexNotify(tt.args); got != tt.want {
				t.Fatalf("shouldRefreshCodexNotify(%v) = %t, want %t", tt.args, got, tt.want)
			}
		})
	}
}

func TestRefreshCodexNotifyCommandRepairsDesktopCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hooks, err := json.Marshal(codexhooks.BuildHookSettings("/old/agent-notify"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexDir, "hooks.json"), hooks, 0o644); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(codexDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(`notify = [ "C:\\runtime\\codex-computer-use.exe", "turn-ended" ]`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	refreshCodexNotifyCommand()

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"codex-computer-use.exe", "--previous-notify", "handle-codex-notify"} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("config = %q, want %q", got, want)
		}
	}
}

func TestRefreshCodexNotifyCommandDoesNotReviveUninstalledIntegration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	codexDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(codexDir, "config.toml")
	want := `notify = [ "C:\\runtime\\codex-computer-use.exe", "turn-ended" ]` + "\n"
	if err := os.WriteFile(configPath, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}

	refreshCodexNotifyCommand()

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("uninstalled integration was revived: %q", got)
	}
}
