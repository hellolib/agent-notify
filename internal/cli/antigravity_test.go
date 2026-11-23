package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunAntigravityHelpListsSubcommands(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"antigravity", "--help"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, want := range []string{"print-hooks", "install-hooks", "uninstall-hooks"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
}

func TestRunAntigravityPrintHooks(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"antigravity", "print-hooks", "--binary", "/usr/local/bin/agent-notify"},
		strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	out := stdout.String()
	for _, want := range []string{"agent-notify", "SessionStart", "PreToolUse", "Stop", "ask_question|ask_permission", "handle-antigravity-hook"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout = %q, want %q", out, want)
		}
	}
}

func TestRunAntigravityInstallAndUninstallHooks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	var stdout, stderr bytes.Buffer
	if err := Run(context.Background(), []string{"antigravity", "install-hooks", "--scope", "user", "--binary", "/usr/local/bin/agent-notify"},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("install-hooks error = %v", err)
	}

	hooksPath := filepath.Join(home, ".gemini", "config", "hooks.json")
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", hooksPath, err)
	}
	if !strings.Contains(string(data), "handle-antigravity-hook") {
		t.Fatalf("hooks.json missing marker:\n%s", string(data))
	}

	if err := Run(context.Background(), []string{"antigravity", "uninstall-hooks", "--scope", "user"},
		strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("uninstall-hooks error = %v", err)
	}
	if _, err := os.Stat(hooksPath); !os.IsNotExist(err) {
		t.Fatalf("expected hooks.json to be removed after uninstall, err = %v", err)
	}
}

func TestRunAntigravityInstallHooksRejectsUnknownScope(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), []string{"antigravity", "install-hooks", "--scope", "invalid"},
		strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run() error = nil, want scope error")
	}
}
