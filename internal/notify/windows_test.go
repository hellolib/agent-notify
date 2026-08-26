package notify

import (
	"context"
	"strings"
	"testing"
)

func TestWindowsSenderSendPushesToastRequest(t *testing.T) {
	var got windowsToastRequest

	sender := NewWindowsSenderWithPusher(func(_ context.Context, req windowsToastRequest) error {
		got = req
		return nil
	}, true, false)

	msg := Message{Agent: "codex", Title: "Test Title", Body: "Test Body", Workspace: "/path/to/project", ActivationURI: "codex://threads/thread-1", FocusCapture: `{"hwnd":"120a3e","title":"pwsh"}`}
	if err := sender.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if got.Title != "Test Title" {
		t.Fatalf("title = %q, want %q", got.Title, "Test Title")
	}
	if !strings.Contains(got.Body, "Test Body") {
		t.Fatalf("body = %q, want to contain %q", got.Body, "Test Body")
	}
	// Windows body uses shortened workspace (last two segments)
	if !strings.Contains(got.Body, "to/project") {
		t.Fatalf("body = %q, want to contain shortened workspace to/project", got.Body)
	}
	if !got.ClickToFocus {
		t.Fatal("ClickToFocus = false, want true")
	}
	if got.FocusCapture != msg.FocusCapture {
		t.Fatalf("FocusCapture = %q, want %q", got.FocusCapture, msg.FocusCapture)
	}
	if got.ActivationURI != msg.ActivationURI {
		t.Fatalf("ActivationURI = %q, want %q", got.ActivationURI, msg.ActivationURI)
	}
	if got.Workspace != msg.Workspace {
		t.Fatalf("Workspace = %q, want %q", got.Workspace, msg.Workspace)
	}
	if got.Agent != "codex" {
		t.Fatalf("Agent = %q, want %q", got.Agent, "codex")
	}
}

func TestWindowsSenderSendWithoutWorkspace(t *testing.T) {
	var got windowsToastRequest

	sender := NewWindowsSenderWithPusher(func(_ context.Context, req windowsToastRequest) error {
		got = req
		return nil
	}, false, false)

	msg := Message{Title: "Title", Body: "Body", Workspace: ""}
	if err := sender.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if !strings.Contains(got.Body, "Body") {
		t.Errorf("body = %q, want to contain %q", got.Body, "Body")
	}
	if got.ClickToFocus {
		t.Fatal("ClickToFocus = true, want false")
	}
}

func TestWindowsSenderSendHonorsCanceledContext(t *testing.T) {
	called := false
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sender := NewWindowsSenderWithPusher(func(_ context.Context, _ windowsToastRequest) error {
		called = true
		return nil
	}, true, false)

	if err := sender.Send(ctx, Message{Title: "Title", Body: "Body"}); err == nil {
		t.Fatal("Send() error = nil, want context cancellation")
	}
	if called {
		t.Fatal("pusher was called after context cancellation")
	}
}

func TestWindowsSenderFormatBody(t *testing.T) {
	sender := &WindowsSender{}

	tests := []struct {
		name      string
		msg       Message
		wantParts []string
		dontWant  []string
	}{
		{
			name:      "with workspace",
			msg:       Message{Body: "Test message", Workspace: "/home/user/project"},
			wantParts: []string{"user/project", "Test message"},
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

			if len(result) < 8 {
				t.Errorf("formatBody() = %q, too short to contain timestamp", result)
			}
		})
	}
}

func TestWindowsSenderName(t *testing.T) {
	sender := &WindowsSender{}
	if sender.Name() != "system" {
		t.Fatalf("Name() = %q, want system", sender.Name())
	}
}
