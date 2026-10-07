package dshhooks

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/notify"
)

// parseFixture 读取 testdata/dsh-hooks 下的样本，复刻真实的读取路径
// （ParseMessage 从 stdin 流解析）。
func parseFixture(t *testing.T, name string) notify.Message {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "dsh-hooks", name))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseMessage(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("ParseMessage(%s) error = %v", name, err)
	}
	return msg
}

// TestParseEventMapping 逐条锁住 DSH 事件 → 归一化事件的映射。
// 这张表是插件与二进制之间的契约，任何一侧改名都会在这里立刻暴露。
func TestParseEventMapping(t *testing.T) {
	// wantEventSuffix 是事件在标题里的展示名，由 notify.eventDisplayName 决定；
	// 这里只锁「映射对了、标题带着对应事件」，展示名的品牌前缀由 M2-4 的
	// appDisplayName 负责，避免本测试替另一个改动点做断言。
	cases := []struct {
		fixture         string
		wantEvent       string
		wantEventSuffix string
	}{
		{"session_start.json", "session_start", "会话开始"},
		{"approval_request.json", "permission_required", "等待授权"},
		{"user_question.json", "input_required", "等待输入"},
		{"stop.json", "run_completed", "运行完成"},
		{"run_failed.json", "run_failed", "运行失败"},
	}

	for _, c := range cases {
		t.Run(c.fixture, func(t *testing.T) {
			msg := parseFixture(t, c.fixture)

			if msg.Event != c.wantEvent {
				t.Errorf("Event = %q, want %q", msg.Event, c.wantEvent)
			}
			if msg.Agent != "dsh" {
				t.Errorf("Agent = %q, want dsh", msg.Agent)
			}
			if msg.SessionID != "sess-dsh-1" {
				t.Errorf("SessionID = %q, want sess-dsh-1", msg.SessionID)
			}
			if msg.Workspace != "/tmp/dsh-demo" {
				t.Errorf("Workspace = %q, want /tmp/dsh-demo", msg.Workspace)
			}
			if !strings.HasSuffix(msg.Title, c.wantEventSuffix) {
				t.Errorf("Title = %q, want it to end with %q", msg.Title, c.wantEventSuffix)
			}
		})
	}
}

// TestParseApprovalRequestWithReason 断言带原因时正文包含工具名与原因。
func TestParseApprovalRequestWithReason(t *testing.T) {
	msg := parseFixture(t, "approval_request.json")

	if !strings.Contains(msg.Body, "bash") {
		t.Errorf("Body = %q, want it to name the tool", msg.Body)
	}
	if !strings.Contains(msg.Body, "writes to the repo") {
		t.Errorf("Body = %q, want it to carry the reason", msg.Body)
	}
}

// TestParseApprovalRequestTruncatesReason 断言超长原因被截断，避免手机通知爆掉。
func TestParseApprovalRequestTruncatesReason(t *testing.T) {
	long := strings.Repeat("x", 500)
	payload := `{"hook_event_name":"ApprovalRequest","session_id":"s1","cwd":"/tmp/p","tool_name":"Bash","reason":"` + long + `"}`

	msg, err := ParseMessage(strings.NewReader(payload))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if !strings.Contains(msg.Body, "...") {
		t.Errorf("Body = %q, want an ellipsis marking truncation", msg.Body)
	}
	if len([]rune(msg.Body)) > 300 {
		t.Errorf("Body has %d runes, want a bounded body", len([]rune(msg.Body)))
	}
}

// TestParseApprovalRequestWithoutToolName 断言缺工具名时给出可读占位，
// 而不是产出「工具  需要授权」这样的断句。
func TestParseApprovalRequestWithoutToolName(t *testing.T) {
	payload := `{"hook_event_name":"ApprovalRequest","session_id":"s1","cwd":"/tmp/p"}`

	msg, err := ParseMessage(strings.NewReader(payload))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if !strings.Contains(msg.Body, "未知工具") {
		t.Errorf("Body = %q, want the placeholder tool name", msg.Body)
	}
}

// TestParseUserQuestion 断言问题文本进入正文——提问阻塞会话等回答，
// 问题本身才是最有信息量的内容。
func TestParseUserQuestion(t *testing.T) {
	msg := parseFixture(t, "user_question.json")

	if !strings.Contains(msg.Body, "Which environment?") {
		t.Errorf("Body = %q, want the question text", msg.Body)
	}
}

// TestParseUserQuestionWithoutQuestion 断言无问题文本时退回通用文案而非空正文。
func TestParseUserQuestionWithoutQuestion(t *testing.T) {
	payload := `{"hook_event_name":"UserQuestion","session_id":"s1","cwd":"/tmp/p"}`

	msg, err := ParseMessage(strings.NewReader(payload))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Body == "" {
		t.Error("Body is empty, want a generic fallback")
	}
}

// TestParseRunFailedCarriesError 断言失败原因进入正文。
func TestParseRunFailedCarriesError(t *testing.T) {
	msg := parseFixture(t, "run_failed.json")

	if !strings.Contains(msg.Body, "exit status 1") {
		t.Errorf("Body = %q, want the error message", msg.Body)
	}
}

// TestParseUnsupportedEvent 断言未知事件名返回错误而不是静默产出零值 Message——
// 后者会以空 agent 进入派发，掩盖插件与二进制之间的版本漂移。
func TestParseUnsupportedEvent(t *testing.T) {
	payload := `{"hook_event_name":"SomethingElse","session_id":"s1","cwd":"/tmp/p"}`

	_, err := ParseMessage(strings.NewReader(payload))
	if err == nil {
		t.Fatal("ParseMessage() error = nil, want an unsupported-event error")
	}
	if !strings.Contains(err.Error(), "SomethingElse") {
		t.Errorf("error = %v, want it to name the offending event", err)
	}
}

// TestParseMalformedJSON 断言坏 JSON 返回错误（handler 会记日志并返回 nil）。
func TestParseMalformedJSON(t *testing.T) {
	_, err := ParseMessage(strings.NewReader("{not json"))
	if err == nil {
		t.Fatal("ParseMessage() error = nil, want a decode error")
	}
}

// TestParseTrimsWhitespace 断言 session id / cwd / 文本字段两端空白被清理：
// 插件透传的值可能带换行，空白会被去重键与工作区解析当成不同值。
func TestParseTrimsWhitespace(t *testing.T) {
	payload := `{"hook_event_name":"ApprovalRequest","session_id":"  s1  ","cwd":"  /tmp/p  ","tool_name":"","reason":"   "}`

	msg, err := ParseMessage(strings.NewReader(payload))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.SessionID != "s1" {
		t.Errorf("SessionID = %q, want trimmed s1", msg.SessionID)
	}
	if msg.Workspace != "/tmp/p" {
		t.Errorf("Workspace = %q, want trimmed /tmp/p", msg.Workspace)
	}
	// reason 全是空白 → 视为未提供，正文走无原因分支。
	if strings.Contains(msg.Body, "原因") {
		t.Errorf("Body = %q, want the no-reason branch for a blank reason", msg.Body)
	}
}

// TestParseSessionStartMinimal 断言最小载荷（插件在无 cwd 时会省略字段）也能解析。
func TestParseSessionStartMinimal(t *testing.T) {
	msg := parseFixture(t, "session_start_minimal.json")

	if msg.Event != "session_start" {
		t.Errorf("Event = %q, want session_start", msg.Event)
	}
	if msg.Agent != "dsh" {
		t.Errorf("Agent = %q, want dsh", msg.Agent)
	}
}
