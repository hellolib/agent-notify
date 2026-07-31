package droidhooks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/notify"
)

func TestParseSessionStart(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "droid-hooks", "session_start.json"))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := parseMessageBytes(data)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Agent != "droid" {
		t.Fatalf("Agent = %q, want droid", msg.Agent)
	}
	if msg.Event != "session_start" {
		t.Fatalf("Event = %q, want session_start", msg.Event)
	}
	if msg.SessionID != "sess-droid-0" {
		t.Fatalf("SessionID = %q, want sess-droid-0", msg.SessionID)
	}
	if msg.Workspace != "/Users/demo/project" {
		t.Fatalf("Workspace = %q, want /Users/demo/project", msg.Workspace)
	}
}

func TestParseNotificationPermission(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "droid-hooks", "notification_permission.json"))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := parseMessageBytes(data)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "permission_required" {
		t.Fatalf("Event = %q, want permission_required", msg.Event)
	}
	if !strings.Contains(msg.Body, "permission") {
		t.Fatalf("Body = %q, want permission message", msg.Body)
	}
}

func TestParseNotificationIdle(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "droid-hooks", "notification_idle.json"))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := parseMessageBytes(data)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "input_required" {
		t.Fatalf("Event = %q, want input_required", msg.Event)
	}
	if !strings.Contains(msg.Body, "提示:") {
		t.Fatalf("Body = %q, want 提示 prefix", msg.Body)
	}
}

// TestParseNotificationAuthSuccessIgnored verifies that non-actionable
// notification types (auth_success) are dropped rather than notified.
func TestParseNotificationAuthSuccessIgnored(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "droid-hooks", "notification_auth_success.json"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := parseMessageBytes(data); err == nil {
		t.Fatal("ParseMessage() expected error for auth_success, got nil")
	}
}

func TestParseStop(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "droid-hooks", "stop.json"))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := parseMessageBytes(data)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "run_completed" {
		t.Fatalf("Event = %q, want run_completed", msg.Event)
	}
	if msg.Agent != "droid" {
		t.Fatalf("Agent = %q, want droid", msg.Agent)
	}
}

func TestParseUnsupportedEvent(t *testing.T) {
	if _, err := parseMessageBytes([]byte(`{"hook_event_name":"PreToolUse","session_id":"s","cwd":"/x"}`)); err == nil {
		t.Fatal("ParseMessage() expected error for PreToolUse, got nil")
	}
}

func TestExtractInputHintStripsPrefix(t *testing.T) {
	got := extractInputHint("Droid is waiting for your input: approve the diff")
	if got != "approve the diff" {
		t.Fatalf("extractInputHint = %q, want 'approve the diff'", got)
	}
}

func TestExtractInputHintEmpty(t *testing.T) {
	if got := extractInputHint(""); got != "等待您的操作" {
		t.Fatalf("extractInputHint(\"\") = %q, want 等待您的操作", got)
	}
}

func parseMessageBytes(data []byte) (notify.Message, error) {
	return ParseMessage(bytes.NewReader(data))
}
