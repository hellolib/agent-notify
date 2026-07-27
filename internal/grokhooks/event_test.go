package grokhooks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/notify"
	"unicode/utf8"
)

func TestParseSessionStart(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "grok-hooks", "session_start.json"))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := parseMessageBytes(data)
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

func TestParseNotificationPermission(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "grok-hooks", "notification_permission.json"))
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
	if !strings.Contains(msg.Body, "run_terminal_command") {
		t.Fatalf("Body = %q, want tool name", msg.Body)
	}
}

func TestParseStop(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "grok-hooks", "stop.json"))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := parseMessageBytes(data)
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

	msg, err := parseMessageBytes(data)
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

	msg, err := parseMessageBytes(data)
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
	msg, err := parseMessageBytes(raw)
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
	msg, err := parseMessageBytes(raw)
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
	_, err := parseMessageBytes(raw)
	if err == nil {
		t.Fatal("parseMessageBytes() expected error for unsupported event PreToolUse")
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
	msg, err := parseMessageBytes(raw)
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
	msg, err := parseMessageBytes(raw)
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
	msg, err := parseMessageBytes(raw)
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
	msg, err := parseMessageBytes(raw)
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

func TestTruncateIsRuneSafe(t *testing.T) {
	// 10 CJK runes; each is 3 bytes. Byte-based truncate would split a rune.
	s := "一二三四五六七八九十"
	got := truncate(s, 6)
	want := "一二三..."
	if got != want {
		t.Fatalf("truncate() = %q, want %q", got, want)
	}
	// Must remain valid UTF-8 and exact rune length of 6 (3 + "...").
	if !utf8.ValidString(got) {
		t.Fatalf("truncate() produced invalid UTF-8: %q", got)
	}
	if n := utf8.RuneCountInString(got); n != 6 {
		t.Fatalf("rune count = %d, want 6", n)
	}
	if truncate(s, 100) != s {
		t.Fatalf("truncate short string should return original")
	}
}

func TestPermissionNotificationTypeAllowlist(t *testing.T) {
	// Positive allowlist.
	for _, tname := range []string{
		"permission_prompt", "permission", "permission_request",
		"approval", "approval_prompt", "approval_request",
	} {
		if !isPermissionNotificationType(tname) {
			t.Fatalf("isPermissionNotificationType(%q) = false, want true", tname)
		}
	}
	// Broad HasPrefix would match these; explicit allowlist must not.
	for _, tname := range []string{
		"permission_granted", "permission_revoked", "approval_rejected",
		"permission_denied", "", "other",
	} {
		if isPermissionNotificationType(tname) {
			t.Fatalf("isPermissionNotificationType(%q) = true, want false", tname)
		}
	}
}

func TestInputRequiredNotificationTypeAllowlist(t *testing.T) {
	for _, tname := range []string{"idle_prompt", "input_required", "waiting_input", "needs_input"} {
		if !isInputRequiredNotificationType(tname) {
			t.Fatalf("isInputRequiredNotificationType(%q) = false, want true", tname)
		}
	}
	for _, tname := range []string{"input_finished", "idle_done", "input_cancelled", ""} {
		if isInputRequiredNotificationType(tname) {
			t.Fatalf("isInputRequiredNotificationType(%q) = true, want false", tname)
		}
	}
}

func TestParseNotificationPermissionGrantedIsNotPermissionRequired(t *testing.T) {
	raw := []byte(`{
		"hookEventName":"notification",
		"sessionId":"s",
		"cwd":"/tmp",
		"notificationType":"permission_granted",
		"message":"Permission was granted"
	}`)
	msg, err := parseMessageBytes(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	// Without allowlist match, falls through to input_required (non-empty message).
	if msg.Event != "input_required" {
		t.Fatalf("Event = %q, want input_required for permission_granted", msg.Event)
	}
}

// parseMessageBytes 让既有用例继续以字节数组驱动 ParseMessage,
// 后者现在从 io.Reader 流式解码(见 common.DecodeHookPayload)。
func parseMessageBytes(data []byte) (notify.Message, error) {
	return ParseMessage(bytes.NewReader(data))
}
