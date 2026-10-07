package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunDshHelpListsSubcommands(t *testing.T) {
	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"dsh", "--help"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for _, want := range []string{"print-hooks", "install-hooks", "uninstall-hooks"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout = %q, want %q", stdout.String(), want)
		}
	}
}

// TestRunDshPrintHooks 断言 print-hooks 输出的是安装计划而不是一份伪造的
// settings JSON —— DSH 没有可写的 hook 配置文件，如实报告落点与命令才有用。
func TestRunDshPrintHooks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("DSH_HOME", home)

	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"dsh", "print-hooks"}, strings.NewReader(""), &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	out := stdout.String()
	for _, want := range []string{
		"agent-notify-dsh",
		"dsh plugin --profile web add agent-notify-dsh",
		"profiles/web/package.json",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout = %q, want it to contain %q", out, want)
		}
	}
}

// TestRunDshPrintHooksProjectScopeFails 断言 project scope 明确报错：
// DSH 的插件装在 profile 下，没有项目级落点。
func TestRunDshPrintHooksRejectsUnknownScope(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), []string{"dsh", "install-hooks", "--scope", "project"},
		strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run() error = nil, want a scope error")
	}
}
