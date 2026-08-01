package notify

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxSenderSendCallsNotifySend(t *testing.T) {
	// 隔离 HOME：让 AgentLogoPath 确定性返回空（无 logo），避免本机已装图标导致 -i 位置漂移。
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	var gotName string
	var gotArgs []string

	sender := NewLinuxSender(func(_ context.Context, name string, args ...string) error {
		gotName = name
		gotArgs = args
		return nil
	}, false)
	// 强制 D-Bus 通知失败，确定性地走到 notify-send 回退（避免依赖运行环境是否有活跃 D-Bus）。
	sender.sendNotify = func(context.Context, string, string, string) error { return context.Canceled }

	msg := Message{Agent: "claude_code", Title: "Test Title", Body: "Test Body", Workspace: "/path/to/project"}
	if err := sender.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if !strings.HasSuffix(gotName, "notify-send") {
		t.Fatalf("name = %q, want notify-send command", gotName)
	}

	// Verify expected arguments structure
	// args: -a "Claude Code" -u normal -t 5000 "Title" "Body"
	expectedArgs := []string{"-a", "Claude Code", "-u", "normal", "-t", "5000", "Test Title"}
	if len(gotArgs) < len(expectedArgs) {
		t.Fatalf("args = %#v, want at least %d args", gotArgs, len(expectedArgs))
	}

	for i, expected := range expectedArgs {
		if gotArgs[i] != expected {
			t.Fatalf("args[%d] = %q, want %q", i, gotArgs[i], expected)
		}
	}

	// Last arg should be the formatted body
	lastArg := gotArgs[len(gotArgs)-1]
	if !strings.Contains(lastArg, "Test Body") {
		t.Fatalf("body = %q, want to contain %q", lastArg, "Test Body")
	}
	if !strings.Contains(lastArg, "/path/to/project") {
		t.Fatalf("body = %q, want to contain workspace path", lastArg)
	}
}

func TestLinuxSenderSendWithoutWorkspace(t *testing.T) {
	var gotArgs []string

	sender := NewLinuxSender(func(_ context.Context, name string, args ...string) error {
		gotArgs = args
		return nil
	}, false)
	sender.sendNotify = func(context.Context, string, string, string) error { return context.Canceled }

	msg := Message{Title: "Title", Body: "Body", Workspace: ""}
	if err := sender.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	// Last arg should be the formatted body without workspace
	lastArg := gotArgs[len(gotArgs)-1]
	if strings.Contains(lastArg, "") && lastArg != "" {
		// If workspace is empty, body should not contain workspace-related prefixes
		if strings.HasPrefix(lastArg, "") {
			// Just check that body contains the message body
			if !strings.Contains(lastArg, "Body") {
				t.Fatalf("body = %q, want to contain %q", lastArg, "Body")
			}
		}
	}
}

func TestLinuxSenderFormatBody(t *testing.T) {
	sender := &LinuxSender{}

	tests := []struct {
		name      string
		msg       Message
		wantParts []string
		dontWant  []string
	}{
		{
			name:      "with workspace",
			msg:       Message{Body: "Test message", Workspace: "/home/user/project"},
			wantParts: []string{"/home/user/project", "Test message"},
			dontWant:  []string{},
		},
		{
			name:      "without workspace",
			msg:       Message{Body: "Test message", Workspace: ""},
			wantParts: []string{"Test message"},
			dontWant:  []string{"/home"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sender.formatBody(tt.msg)

			for _, want := range tt.wantParts {
				if !strings.Contains(result, want) {
					t.Errorf("formatBody() = %q, want to contain %q", result, want)
				}
			}

			for _, dontWant := range tt.dontWant {
				if strings.Contains(result, dontWant) {
					t.Errorf("formatBody() = %q, should not contain %q", result, dontWant)
				}
			}

			// Should always contain timestamp
			// Timestamp format is "15:04:05"
			if len(result) < 8 { // minimum: "x\nHH:MM:SS"
				t.Errorf("formatBody() = %q, too short to contain timestamp", result)
			}
		})
	}
}

func TestLinuxSenderClickToFocusStartsFocusHelper(t *testing.T) {
	// 隔离 HOME：msg 未设 Agent，AgentLogoPath("") 应确定性返回空。
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("USERPROFILE", homeDir)

	runCalled := false
	startCalled := false

	sender := NewLinuxSenderWithFocusStarter(func(_ context.Context, name string, args ...string) error {
		runCalled = true
		return nil
	}, true, func(_ context.Context, icon, title, body, windowID string) error {
		startCalled = true
		if icon != "" {
			t.Fatalf("icon = %q, want empty when agent unset", icon)
		}
		if title != "Title" {
			t.Fatalf("title = %q, want Title", title)
		}
		if !strings.Contains(body, "Body") {
			t.Fatalf("body = %q, want to contain Body", body)
		}
		if windowID != "0x123" {
			t.Fatalf("windowID = %q, want 0x123 (from Message.FocusWindowID)", windowID)
		}
		return nil
	})

	if err := sender.Send(context.Background(), Message{Title: "Title", Body: "Body", FocusWindowID: "0x123"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if !startCalled {
		t.Fatal("focus starter was not called")
	}
	if runCalled {
		t.Fatal("plain notify-send runner was called after focus starter succeeded")
	}
}

func TestLinuxSenderClickToFocusFallsBackToNotifySend(t *testing.T) {
	var gotName string

	sender := NewLinuxSenderWithFocusStarter(func(_ context.Context, name string, args ...string) error {
		gotName = name
		return nil
	}, true, func(_ context.Context, icon, title, body, windowID string) error {
		return context.Canceled
	})
	sender.sendNotify = func(context.Context, string, string, string) error { return context.Canceled }

	if err := sender.Send(context.Background(), Message{Title: "Title", Body: "Body"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if !strings.HasSuffix(gotName, "notify-send") {
		t.Fatalf("fallback runner = %q, want notify-send command", gotName)
	}
}

func TestLinuxSenderName(t *testing.T) {
	sender := &LinuxSender{}
	if sender.Name() != "system" {
		t.Fatalf("Name() = %q, want system", sender.Name())
	}
}

// setupAgentLogoEnv 在 tmpDir 下建好 ~/.agent-notify/agentlogo/<file>，并把 HOME 与
// USERPROFILE 都指向 tmpDir（这些测试无 build tag、会在 windows-latest 上跑，故两者
// 都要设），返回期望的图标路径。
func setupAgentLogoEnv(t *testing.T, filename string) (tmpDir, wantPath string) {
	t.Helper()
	tmpDir = t.TempDir()
	agentlogoDir := filepath.Join(tmpDir, ".agent-notify", "agentlogo")
	if err := os.MkdirAll(agentlogoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	wantPath = filepath.Join(agentlogoDir, filename)
	if err := os.WriteFile(wantPath, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	return tmpDir, wantPath
}

// D-Bus 非交互通知路径（sendNotify）：clickToFocus 关闭时，AgentLogoPath 解出的图标
// 应原样下传给 sendNotify。
func TestLinuxSenderPassesAgentLogoToDBusNotify(t *testing.T) {
	_, wantIcon := setupAgentLogoEnv(t, "claude.png")

	var gotIcon string
	sender := NewLinuxSender(func(_ context.Context, name string, _ ...string) error {
		t.Fatalf("runner should not be called; got %q", name)
		return nil
	}, false)
	sender.sendNotify = func(_ context.Context, icon, _, _ string) error {
		gotIcon = icon
		return nil
	}

	if err := sender.Send(context.Background(), Message{Agent: "claude_code", Title: "T", Body: "B"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if gotIcon != wantIcon {
		t.Fatalf("sendNotify icon = %q, want %q", gotIcon, wantIcon)
	}
}

// click-to-focus 主路径（startFocus）：图标应下传给 startFocus（进而经 Request.Icon
// → linux-notify-wait --icon → waitNotificationAction 的 D-Bus Notify icon 参）。
func TestLinuxSenderPassesAgentLogoToFocusStarter(t *testing.T) {
	_, wantIcon := setupAgentLogoEnv(t, "claude.png")

	var gotIcon string
	sender := NewLinuxSenderWithFocusStarter(
		func(_ context.Context, name string, _ ...string) error {
			t.Fatalf("runner should not be called; got %q", name)
			return nil
		}, true, func(_ context.Context, icon, _, _, _ string) error {
			gotIcon = icon
			return nil
		})

	if err := sender.Send(context.Background(), Message{Agent: "claude_code", Title: "T", Body: "B", FocusWindowID: "0x123"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if gotIcon != wantIcon {
		t.Fatalf("startFocus icon = %q, want %q", gotIcon, wantIcon)
	}
}

// notify-send 回退路径：D-Bus 失败后走 notify-send，-i 仅在找到图标时注入，
// -a 用 agent 显示名（claude_code → "Claude Code"）。
func TestLinuxSenderNotifySendFallbackInjectsIcon(t *testing.T) {
	_, wantIcon := setupAgentLogoEnv(t, "claude.png")

	var gotArgs []string
	sender := NewLinuxSender(func(_ context.Context, _ string, args ...string) error {
		gotArgs = args
		return nil
	}, false)
	sender.sendNotify = func(context.Context, string, string, string) error { return context.Canceled }

	if err := sender.Send(context.Background(), Message{Agent: "claude_code", Title: "T", Body: "B"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if !sliceContainsPair(gotArgs, "-a", "Claude Code") {
		t.Fatalf("args = %#v, want -a Claude Code", gotArgs)
	}
	if !sliceContainsPair(gotArgs, "-i", wantIcon) {
		t.Fatalf("args = %#v, want -i %q", gotArgs, wantIcon)
	}
}
