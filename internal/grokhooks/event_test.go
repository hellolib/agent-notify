package grokhooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSessionStart(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "grok-hooks", "session_start.json"))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Agent != "grok" {
		t.Fatalf("Agent = %q, want grok", msg.Agent)
	}
	if msg.Event != "session_start" {
		t.Fatalf("Event = %q, want session_start", msg.Event)
	}
	if msg.SessionID != "sess-grok-0" {
		t.Fatalf("SessionID = %q, want sess-grok-0", msg.SessionID)
	}
	if msg.Workspace != "/Users/demo/project" {
		t.Fatalf("Workspace = %q, want /Users/demo/project", msg.Workspace)
	}
}

func TestParseNotificationWaitingInput(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "grok-hooks", "notification_waiting_input.json"))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseMessage(data)
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

func TestParseNotificationPermission(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "grok-hooks", "notification_permission.json"))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "permission_required" {
		t.Fatalf("Event = %q, want permission_required", msg.Event)
	}
	if !strings.Contains(msg.Body, "run_terminal_command") {
		t.Fatalf("Body = %q, want tool name", msg.Body)
	}
}

func TestParseStop(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "grok-hooks", "stop.json"))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Agent != "grok" {
		t.Fatalf("Agent = %q, want grok", msg.Agent)
	}
	if msg.Event != "run_completed" {
		t.Fatalf("Event = %q, want run_completed", msg.Event)
	}
}

func TestParseStopFailure(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "grok-hooks", "stop_failure.json"))
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
	if !strings.Contains(msg.Body, "rate limit") {
		t.Fatalf("Body = %q, want rate limit error", msg.Body)
	}
}

func TestParsePostToolUseFailure(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "grok-hooks", "post_tool_use_failure.json"))
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
	if !strings.Contains(msg.Body, "run_terminal_command") {
		t.Fatalf("Body = %q, want tool name", msg.Body)
	}
	if !strings.Contains(msg.Body, "exited with code 1") {
		t.Fatalf("Body = %q, want error detail", msg.Body)
	}
}

func TestParsePascalCaseEventNames(t *testing.T) {
	raw := []byte(`{"hookEventName":"Stop","sessionId":"s1","cwd":"/tmp"}`)
	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "run_completed" {
		t.Fatalf("Event = %q, want run_completed", msg.Event)
	}
	if msg.SessionID != "s1" {
		t.Fatalf("SessionID = %q, want s1", msg.SessionID)
	}
}

func TestParseSnakeCaseFieldNames(t *testing.T) {
	raw := []byte(`{"hook_event_name":"SessionStart","session_id":"s2","cwd":"/work"}`)
	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "session_start" {
		t.Fatalf("Event = %q, want session_start", msg.Event)
	}
	if msg.SessionID != "s2" {
		t.Fatalf("SessionID = %q, want s2", msg.SessionID)
	}
}

func TestParseUnsupportedEvent(t *testing.T) {
	raw := []byte(`{"hookEventName":"PreToolUse","sessionId":"s","cwd":"/tmp"}`)
	_, err := ParseMessage(raw)
	if err == nil {
		t.Fatal("ParseMessage() expected error for unsupported event PreToolUse")
	}
}

func TestParseNotificationPrefersTypeOverBroadMessageKeywords(t *testing.T) {
	// Message casually mentions "permission" but type is idle — must stay input_required.
	raw := []byte(`{
		"hookEventName":"notification",
		"sessionId":"s",
		"cwd":"/tmp",
		"message":"Discussed file permission settings earlier",
		"notificationType":"idle_prompt"
	}`)
	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "input_required" {
		t.Fatalf("Event = %q, want input_required (type should win over bare word 'permission')", msg.Event)
	}
}

func TestParseNotificationTypePermissionPrompt(t *testing.T) {
	raw := []byte(`{
		"hookEventName":"notification",
		"sessionId":"s",
		"cwd":"/tmp",
		"toolName":"bash",
		"message":"Allow shell?",
		"notificationType":"permission_prompt"
	}`)
	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "permission_required" {
		t.Fatalf("Event = %q, want permission_required", msg.Event)
	}
	if !strings.Contains(msg.Body, "bash") {
		t.Fatalf("Body = %q, want tool name", msg.Body)
	}
}

func TestParseNotificationBarePermissionWordIsNotPermissionRequired(t *testing.T) {
	// Without notificationType, a bare "permission" word must not force permission_required.
	raw := []byte(`{
		"hookEventName":"notification",
		"sessionId":"s",
		"cwd":"/tmp",
		"message":"Check the permission model docs"
	}`)
	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "input_required" {
		t.Fatalf("Event = %q, want input_required for non-phrase permission mention", msg.Event)
	}
}

func TestParseNotificationMessagePhrasePermissionRequired(t *testing.T) {
	raw := []byte(`{
		"hookEventName":"notification",
		"sessionId":"s",
		"cwd":"/tmp",
		"toolName":"run_terminal_command",
		"message":"Permission required to run shell command"
	}`)
	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "permission_required" {
		t.Fatalf("Event = %q, want permission_required", msg.Event)
	}
}

func TestExtractInputHintStripsGrokPrefix(t *testing.T) {
	got := extractInputHint("Grok is waiting for your input: continue?")
	if got != "continue?" {
		t.Fatalf("extractInputHint() = %q, want continue?", got)
	}
	// Claude-specific prefix must not be special-cased for Grok payloads.
	got = extractInputHint("claude is waiting for your input: x")
	if got != "claude is waiting for your input: x" {
		t.Fatalf("unexpected strip of claude prefix: %q", got)
	}
}
