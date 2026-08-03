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
	if msg.Workspace != "/tmp/demo" {
		t.Fatalf("Workspace = %q, want /tmp/demo", msg.Workspace)
	}
	// default 模式命令执行（Bash，简单命令）应有 3 个按钮（y + p + esc）
	if len(msg.Actions) != 3 {
		t.Fatalf("Actions len = %d, want 3 for default mode Bash simple command", len(msg.Actions))
	}
	if msg.Actions[0].Value != "allow" {
		t.Fatalf("Actions[0].Value = %q, want allow", msg.Actions[0].Value)
	}
	if msg.Actions[1].Value != "allow_prefix" {
		t.Fatalf("Actions[1].Value = %q, want allow_prefix", msg.Actions[1].Value)
	}
	if msg.Actions[2].Value != "reject" {
		t.Fatalf("Actions[2].Value = %q, want reject", msg.Actions[2].Value)
	}
}

func TestParsePermissionRequestHeredoc(t *testing.T) {
	raw := []byte(`{"hook_event_name":"PermissionRequest","session_id":"s","cwd":"/tmp","tool_name":"Bash","permission_mode":"default","tool_input":{"command":"cat << EOF\nhello\nEOF"}}`)
	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	// heredoc 命令无文件重定向 → 2 按钮
	if len(msg.Actions) != 2 {
		t.Fatalf("Actions len = %d, want 2 for heredoc command", len(msg.Actions))
	}
}

func TestParsePermissionRequestHeredocWithRedirect(t *testing.T) {
	raw := []byte(`{"hook_event_name":"PermissionRequest","session_id":"s","cwd":"/tmp","tool_name":"Bash","permission_mode":"default","tool_input":{"command":"cat << EOF > out.txt\nhello\nEOF"}}`)
	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	// heredoc + 文件重定向 → 3 按钮
	if len(msg.Actions) != 3 {
		t.Fatalf("Actions len = %d, want 3 for heredoc+redirect command", len(msg.Actions))
	}
}

func TestParsePermissionRequestApplyPatch(t *testing.T) {
	raw := []byte(`{"hook_event_name":"PermissionRequest","session_id":"s","cwd":"/tmp","tool_name":"apply_patch","permission_mode":"default","tool_input":{"command":"some patch"}}`)
	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	// apply_patch → 3 按钮 (y + a + esc)
	if len(msg.Actions) != 3 {
		t.Fatalf("Actions len = %d, want 3 for apply_patch", len(msg.Actions))
	}
	if msg.Actions[1].Value != "allow_session" {
		t.Fatalf("Actions[1].Value = %q, want allow_session for apply_patch", msg.Actions[1].Value)
	}
}

func TestParsePermissionRequestNonDefault(t *testing.T) {
	raw := []byte(`{"hook_event_name":"PermissionRequest","session_id":"s","cwd":"/tmp","tool_name":"Bash","permission_mode":"auto_approve"}`)
	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	// 非 default 模式应有 2 个按钮
	if len(msg.Actions) != 2 {
		t.Fatalf("Actions len = %d, want 2 for non-default mode", len(msg.Actions))
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

func TestParsePermissionRequestNetworkCommandIs3Buttons(t *testing.T) {
	// 网络命令在代理环境下不触发 additional_permissions，TUI 显示 3 按钮
	raw := []byte(`{"hook_event_name":"PermissionRequest","session_id":"s","cwd":"/tmp","tool_name":"Bash","permission_mode":"default","tool_input":{"command":"sshpass -p pwd ssh -o StrictHostKeyChecking=no user@10.0.35.183 kubectl get pods"}}`)
	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if len(msg.Actions) != 3 {
		t.Fatalf("Actions len = %d, want 3 for network command (proxy env)", len(msg.Actions))
	}
}
