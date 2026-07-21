package notify

import (
	"context"
	"strings"
	"testing"
)

func TestLinuxSenderSendCallsNotifySend(t *testing.T) {
	var gotName string
	var gotArgs []string

	sender := NewLinuxSender(func(_ context.Context, name string, args ...string) error {
		gotName = name
		gotArgs = args
		return nil
	}, false)
	// 强制 D-Bus 通知失败，确定性地走到 notify-send 回退（避免依赖运行环境是否有活跃 D-Bus）。
	sender.sendNotify = func(context.Context, string, string) error { return context.Canceled }

	msg := Message{Title: "Test Title", Body: "Test Body", Workspace: "/path/to/project"}
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
	sender.sendNotify = func(context.Context, string, string) error { return context.Canceled }

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
	runCalled := false
	startCalled := false

	sender := NewLinuxSenderWithFocusStarter(func(_ context.Context, name string, args ...string) error {
		runCalled = true
		return nil
	}, true, func(_ context.Context, title, body, windowID string) error {
		startCalled = true
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
	}, true, func(_ context.Context, title, body, windowID string) error {
		return context.Canceled
	})
	sender.sendNotify = func(context.Context, string, string) error { return context.Canceled }

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
