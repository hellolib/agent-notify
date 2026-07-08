package codexhooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePermissionRequest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "codex-hooks", "permission_request.json"))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Agent != "codex" {
		t.Fatalf("Agent = %q, want codex", msg.Agent)
	}
	if msg.Event != "permission_required" {
		t.Fatalf("Event = %q, want permission_required", msg.Event)
	}
	if !strings.Contains(msg.Body, "Bash") {
		t.Fatalf("Body = %q, want tool name Bash", msg.Body)
	}
	if msg.ToolName != "Bash" {
		t.Fatalf("ToolName = %q, want Bash", msg.ToolName)
	}
	if msg.Workspace != "/tmp/demo" {
		t.Fatalf("Workspace = %q, want /tmp/demo", msg.Workspace)
	}
}

func TestParsePostToolUseCancellation(t *testing.T) {
	raw := []byte(`{"hook_event_name":"PostToolUse","session_id":"s1","cwd":"/tmp/demo","tool_name":"Bash"}`)

	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "tool_completed" {
		t.Fatalf("Event = %q, want tool_completed", msg.Event)
	}
	if msg.ToolName != "Bash" {
		t.Fatalf("ToolName = %q, want Bash", msg.ToolName)
	}
	if msg.Agent != "codex" {
		t.Fatalf("Agent = %q, want codex", msg.Agent)
	}
}

func TestParseStop(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "codex-hooks", "stop.json"))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Agent != "codex" {
		t.Fatalf("Agent = %q, want codex", msg.Agent)
	}
	if msg.Event != "run_completed" {
		t.Fatalf("Event = %q, want run_completed", msg.Event)
	}
	// last_assistant_message 非空时应作为 Body
	if !strings.Contains(msg.Body, "cargo build") {
		t.Fatalf("Body = %q, want last_assistant_message content", msg.Body)
	}
}

func TestParseStopFallsBackToDefaultBody(t *testing.T) {
	raw := []byte(`{"hook_event_name":"Stop","session_id":"s","cwd":"/tmp","last_assistant_message":""}`)

	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Body == "" {
		t.Fatal("Body should fall back to default when last_assistant_message empty")
	}
}

func TestParseUnsupportedEvent(t *testing.T) {
	raw := []byte(`{"hook_event_name":"UserPromptSubmit","session_id":"s","cwd":"/tmp"}`)

	_, err := ParseMessage(raw)
	if err == nil {
		t.Fatal("ParseMessage() expected error for unsupported event")
	}
}

func TestTruncateMessage(t *testing.T) {
	tests := []struct {
		in    string
		limit int
		want  string
	}{
		{"", 10, ""},
		{"short", 10, "short"},
		{"1234567890ab", 10, "1234567..."},
	}
	for _, tt := range tests {
		got := truncateMessage(tt.in, tt.limit)
		if got != tt.want {
			t.Fatalf("truncateMessage(%q, %d) = %q, want %q", tt.in, tt.limit, got, tt.want)
		}
	}
}
