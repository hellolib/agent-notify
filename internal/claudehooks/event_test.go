package claudehooks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePermissionRequest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "hooks", "permission_request.json"))
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
	if msg.Agent != "claude_code" {
		t.Fatalf("Agent = %q, want claude_code", msg.Agent)
	}
}

func TestParseNotificationWaitingInput(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "hooks", "notification_waiting_input.json"))
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
	if msg.Agent != "claude_code" {
		t.Fatalf("Agent = %q, want claude_code", msg.Agent)
	}
	if msg.Body != "提示: " {
		t.Fatalf("Body = %q, want %q", msg.Body, "提示: ")
	}
}

func TestParseNotificationNeedsInputVariant(t *testing.T) {
	data := []byte(`{"hook_event_name":"Notification","session_id":"s1","cwd":"/tmp/project","message":"needs input: please confirm"}`)

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "input_required" {
		t.Fatalf("Event = %q, want input_required", msg.Event)
	}
	if msg.Agent != "claude_code" {
		t.Fatalf("Agent = %q, want claude_code", msg.Agent)
	}
	if msg.Body != "提示: please confirm" {
		t.Fatalf("Body = %q, want %q", msg.Body, "提示: please confirm")
	}
}

// issue #32:tool_response 依工具而异(MCP 返回数组、Bash 类返回字符串),
// 不能因为不是对象就丢掉整个 run_failed 事件。
func TestParseMessagePostToolUseFailureToleratesNonObjectToolResponse(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{"mcp content array", `{"hook_event_name":"PostToolUseFailure","session_id":"s1","cwd":"/w","tool_name":"mcp__search","tool_response":[{"type":"text","text":"boom"}]}`},
		{"string response", `{"hook_event_name":"PostToolUseFailure","session_id":"s1","cwd":"/w","tool_name":"Bash","tool_response":"command failed"}`},
		{"object with error still works", `{"hook_event_name":"PostToolUseFailure","session_id":"s1","cwd":"/w","tool_name":"Write","tool_response":{"error":"denied"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := ParseMessage([]byte(tc.json))
			if err != nil {
				t.Fatalf("ParseMessage() error = %v, want run_failed event", err)
			}
			if msg.Event != "run_failed" {
				t.Fatalf("Event = %q, want run_failed", msg.Event)
			}
		})
	}
}
