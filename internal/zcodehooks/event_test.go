package zcodehooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSessionStart(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "zcode-hooks", "session_start.json"))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Agent != "zcode" {
		t.Fatalf("Agent = %q, want zcode", msg.Agent)
	}
	if msg.Event != "session_start" {
		t.Fatalf("Event = %q, want session_start", msg.Event)
	}
	if msg.SessionID != "sess-zcode-0" {
		t.Fatalf("SessionID = %q, want sess-zcode-0", msg.SessionID)
	}
}

func TestParsePermissionRequest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "zcode-hooks", "permission_request.json"))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Agent != "zcode" {
		t.Fatalf("Agent = %q, want zcode", msg.Agent)
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
	if msg.Workspace != "C:\\Users\\demo\\project" {
		t.Fatalf("Workspace = %q, want C:\\Users\\demo\\project", msg.Workspace)
	}
}

func TestParsePostToolUseCancellation(t *testing.T) {
	raw := []byte(`{"hookEventName":"PostToolUse","sessionId":"s1","cwd":"C:\\repo","tool_name":"Bash"}`)

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
	if msg.Agent != "zcode" {
		t.Fatalf("Agent = %q, want zcode", msg.Agent)
	}
}

func TestParsePostToolUseFailure(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "zcode-hooks", "post_tool_use_failure.json"))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "run_failed" {
		t.Fatalf("Event = %q, want run_failed", msg.Event)
	}
	if !strings.Contains(msg.Body, "Bash") {
		t.Fatalf("Body = %q, want tool name Bash", msg.Body)
	}
}

func TestParseStop(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "zcode-hooks", "stop.json"))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Agent != "zcode" {
		t.Fatalf("Agent = %q, want zcode", msg.Agent)
	}
	if msg.Event != "run_completed" {
		t.Fatalf("Event = %q, want run_completed", msg.Event)
	}
}

// TestParseCamelCaseFallback 验证当只有驼峰字段名 hookEventName（无下划线版本）时也能解析。
func TestParseCamelCaseFallback(t *testing.T) {
	// 只有驼峰字段，模拟部分 ZCode 版本只下发 hookEventName 的情况
	raw := []byte(`{"hookEventName":"Stop","sessionId":"s1","cwd":"/tmp"}`)

	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "run_completed" {
		t.Fatalf("Event = %q, want run_completed", msg.Event)
	}
	if msg.SessionID != "s1" {
		t.Fatalf("SessionID = %q, want s1 (from camelCase sessionId)", msg.SessionID)
	}
}

func TestParseUnsupportedEvent(t *testing.T) {
	// Notification 是 Claude Code 专有事件，ZCode 不支持
	raw := []byte(`{"hook_event_name":"Notification","session_id":"s","cwd":"/tmp"}`)

	_, err := ParseMessage(raw)
	if err == nil {
		t.Fatal("ParseMessage() expected error for unsupported event Notification")
	}
}
