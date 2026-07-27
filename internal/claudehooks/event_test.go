package claudehooks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/common"
	"github.com/hellolib/agent-notify/internal/notify"
)

func TestParsePermissionRequest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "hooks", "permission_request.json"))
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
	if msg.Agent != "claude_code" {
		t.Fatalf("Agent = %q, want claude_code", msg.Agent)
	}
}

func TestParseNotificationWaitingInput(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "hooks", "notification_waiting_input.json"))
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
	if msg.Agent != "claude_code" {
		t.Fatalf("Agent = %q, want claude_code", msg.Agent)
	}
	if msg.Body != "提示: " {
		t.Fatalf("Body = %q, want %q", msg.Body, "提示: ")
	}
}

func TestParseNotificationNeedsInputVariant(t *testing.T) {
	data := []byte(`{"hook_event_name":"Notification","session_id":"s1","cwd":"/tmp/project","message":"needs input: please confirm"}`)

	msg, err := parseMessageBytes(data)
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
			msg, err := parseMessageBytes([]byte(tc.json))
			if err != nil {
				t.Fatalf("ParseMessage() error = %v, want run_failed event", err)
			}
			if msg.Event != "run_failed" {
				t.Fatalf("Event = %q, want run_failed", msg.Event)
			}
		})
	}
}

// parseMessageBytes 让既有用例继续以字节数组驱动 ParseMessage,
// 后者现在从 io.Reader 流式解码(见 common.DecodeHookPayload)。
func parseMessageBytes(data []byte) (notify.Message, error) {
	return ParseMessage(bytes.NewReader(data))
}

// TestParseMessageRejectsOversizedPayload 是 issue #39 第 5 项的回归测试。
// 旧实现用 io.ReadAll 无界读取 stdin,而 hook 是每事件一进程:
// agent 传来带整份文件内容的 tool_response 时会把它整个缓冲进内存。
func TestParseMessageRejectsOversizedPayload(t *testing.T) {
	oversized := `{"hook_event_name":"Stop","tool_response":"` +
		strings.Repeat("x", common.MaxHookPayloadBytes+1024) + `"}`

	_, err := ParseMessage(strings.NewReader(oversized))

	if err == nil {
		t.Fatal("超过上限的 payload 应当被拒绝")
	}
	// 日志要指出真正的原因,而不是笼统的解析失败
	if !strings.Contains(err.Error(), "上限") {
		t.Fatalf("错误应点名上限,实际是: %v", err)
	}
}

// TestParseMessageIgnoresHugeUndeclaredToolInput tool_input 从未被读取,
// 现已从 payload 结构体移除,Decoder 会跳过它而不为其分配。
func TestParseMessageIgnoresHugeUndeclaredToolInput(t *testing.T) {
	src := `{"hook_event_name":"Stop","session_id":"s1","cwd":"/w","tool_input":{"content":"` +
		strings.Repeat("x", 1<<20) + `"}}`

	msg, err := ParseMessage(strings.NewReader(src))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "run_completed" || msg.SessionID != "s1" || msg.Workspace != "/w" {
		t.Fatalf("大 tool_input 影响了其它字段: %+v", msg)
	}
}
