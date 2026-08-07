package opencodehooks

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseSessionCreated(t *testing.T) {
	input := `{"type":"session.created","properties":{"sessionID":"sess-oc-0","directory":"/Users/demo/project"}}`

	msg, err := ParseMessage(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Agent != "opencode" {
		t.Fatalf("Agent = %q, want opencode", msg.Agent)
	}
	if msg.Event != "session_start" {
		t.Fatalf("Event = %q, want session_start", msg.Event)
	}
	if msg.SessionID != "sess-oc-0" {
		t.Fatalf("SessionID = %q, want sess-oc-0", msg.SessionID)
	}
	if msg.Workspace != "/Users/demo/project" {
		t.Fatalf("Workspace = %q, want /Users/demo/project", msg.Workspace)
	}
}

func TestParsePermissionAsked(t *testing.T) {
	input := `{"type":"permission.asked","properties":{"sessionID":"s1","directory":"/p","permission":"bash"}}`

	msg, err := ParseMessage(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "permission_required" {
		t.Fatalf("Event = %q, want permission_required", msg.Event)
	}
	if !strings.Contains(msg.Body, "bash") {
		t.Fatalf("Body = %q, want permission text containing bash", msg.Body)
	}
}

func TestParseSessionStatusIdle(t *testing.T) {
	input := `{"type":"session.status","properties":{"sessionID":"s2","directory":"/p","status":{"type":"idle"}}}`

	msg, err := ParseMessage(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "input_required" {
		t.Fatalf("Event = %q, want input_required", msg.Event)
	}
}

func TestParseSessionStatusBusySkipped(t *testing.T) {
	input := `{"type":"session.status","properties":{"sessionID":"s2","directory":"/p","status":{"type":"busy"}}}`

	if _, err := ParseMessage(bytes.NewReader([]byte(input))); err == nil {
		t.Fatal("ParseMessage() expected error for busy status, got nil")
	}
}

func TestParseSessionIdle(t *testing.T) {
	input := `{"type":"session.idle","properties":{"sessionID":"s3","directory":"/p"}}`

	msg, err := ParseMessage(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "run_completed" {
		t.Fatalf("Event = %q, want run_completed", msg.Event)
	}
	if !strings.Contains(msg.Body, "完成") {
		t.Fatalf("Body = %q, want completion hint", msg.Body)
	}
}

func TestParseSessionError(t *testing.T) {
	input := `{"type":"session.error","properties":{"sessionID":"s4","directory":"/p","error":{"message":"boom"}}}`

	msg, err := ParseMessage(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "run_failed" {
		t.Fatalf("Event = %q, want run_failed", msg.Event)
	}
	if !strings.Contains(msg.Body, "boom") {
		t.Fatalf("Body = %q, want error text containing boom", msg.Body)
	}
}

func TestParseUnsupportedEvent(t *testing.T) {
	input := `{"type":"unknown.event","properties":{"sessionID":"s"}}`
	if _, err := ParseMessage(bytes.NewReader([]byte(input))); err == nil {
		t.Fatal("ParseMessage() expected error for unknown event, got nil")
	}
}

func TestPermissionBodyBounded(t *testing.T) {
	long := strings.Repeat("x", 500)
	got := permissionBody(long)
	if n := len([]rune(got)); n > 300 {
		t.Fatalf("permissionBody too long: %d runes", n)
	}
}

func TestPermissionBodyEmpty(t *testing.T) {
	got := permissionBody("")
	if !strings.Contains(got, "授权") {
		t.Fatalf("permissionBody(\"\") = %q, want default permission text", got)
	}
}

func TestFailedErrorBodyFallback(t *testing.T) {
	input := `{"type":"session.error","properties":{"sessionID":"s5","directory":"/p"}}`

	msg, err := ParseMessage(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "run_failed" {
		t.Fatalf("Event = %q, want run_failed", msg.Event)
	}
	if !strings.Contains(msg.Body, "错误") {
		t.Fatalf("Body = %q, want fallback error text", msg.Body)
	}
}
