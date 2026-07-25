package codexhooks

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/notify"
)

func TestParseRequestUserInput(t *testing.T) {
	raw := []byte(`{
  "hook_event_name": "PreToolUse",
  "session_id": "session-input",
  "cwd": "/tmp/项目",
  "tool_name": "request_user_input",
  "tool_use_id": "call-input-1",
  "tool_input": {
    "autoResolutionMs": 120000,
    "questions": [
      {
        "id": "environment",
        "header": "环境",
        "question": "选择部署区域 🌏",
        "isOther": true,
        "isSecret": false,
        "options": [
          {"label": "生产", "description": "面向真实用户"},
          {"label": "测试", "description": "仅用于验证"}
        ]
      },
      {
        "id": "token",
        "header": "令牌",
        "question": "请输入令牌",
        "isOther": false,
        "isSecret": true,
        "options": null
      }
    ]
  }
}`)

	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Agent != "codex" || msg.Event != "input_required" {
		t.Fatalf("message identity = %#v, want codex/input_required", msg)
	}
	if msg.SessionID != "session-input" || msg.DedupeID != "call-input-1" {
		t.Fatalf("session/dedupe = %q/%q, want session-input/call-input-1", msg.SessionID, msg.DedupeID)
	}
	if msg.Workspace != "/tmp/项目" {
		t.Fatalf("Workspace = %q, want Unicode workspace", msg.Workspace)
	}
	if msg.AutoResolutionMs != 120000 {
		t.Fatalf("AutoResolutionMs = %d, want 120000", msg.AutoResolutionMs)
	}
	if len(msg.Questions) != 2 {
		t.Fatalf("Questions = %#v, want 2 questions", msg.Questions)
	}
	first := msg.Questions[0]
	if first.ID != "environment" || first.Header != "环境" || first.Question != "选择部署区域 🌏" {
		t.Fatalf("first question = %#v", first)
	}
	if !first.IsOther || first.IsSecret || len(first.Options) != 2 {
		t.Fatalf("first question flags/options = %#v", first)
	}
	if first.Options[0].Label != "生产" || first.Options[0].Description != "面向真实用户" {
		t.Fatalf("first option = %#v", first.Options[0])
	}
	second := msg.Questions[1]
	if !second.IsSecret || second.Options != nil {
		t.Fatalf("second question = %#v, want secret with nil options", second)
	}
	for _, want := range []string{"选择部署区域 🌏", "生产", "面向真实用户", "其他（自由输入）", "请回到 Codex 终端提交答案"} {
		if !strings.Contains(msg.Body, want) {
			t.Errorf("Body = %q, missing %q", msg.Body, want)
		}
	}
}

func TestParseRequestUserInputWithoutOptions(t *testing.T) {
	raw := []byte(`{"hook_event_name":"PreToolUse","tool_name":"request_user_input","tool_input":{"questions":[{"id":"free","header":"自由输入","question":"请说明原因","isOther":true}]}}`)

	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if len(msg.Questions) != 1 || msg.Questions[0].Options != nil || !msg.Questions[0].IsOther {
		t.Fatalf("Questions = %#v, want one no-option Other question", msg.Questions)
	}
}

func TestParsePreToolUseIgnoresOtherTools(t *testing.T) {
	raw := []byte(`{"hook_event_name":"PreToolUse","session_id":"s","tool_name":"Bash","tool_use_id":"call-shell","tool_input":{"command":"echo hi"}}`)

	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v, want safe ignore", err)
	}
	if msg.Event != "" || msg.Agent != "" {
		t.Fatalf("ignored message = %#v, want empty message", msg)
	}
}

func TestParseRequestUserInputRejectsMalformedPayloads(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"missing tool input", `{"hook_event_name":"PreToolUse","tool_name":"request_user_input"}`},
		{"missing questions", `{"hook_event_name":"PreToolUse","tool_name":"request_user_input","tool_input":{}}`},
		{"empty questions", `{"hook_event_name":"PreToolUse","tool_name":"request_user_input","tool_input":{"questions":[]}}`},
		{"questions wrong type", `{"hook_event_name":"PreToolUse","tool_name":"request_user_input","tool_input":{"questions":"nope"}}`},
		{"option wrong type", `{"hook_event_name":"PreToolUse","tool_name":"request_user_input","tool_input":{"questions":[{"question":"q","options":"nope"}]}}`},
		{"timeout wrong type", `{"hook_event_name":"PreToolUse","tool_name":"request_user_input","tool_input":{"questions":[{"question":"q"}],"autoResolutionMs":"later"}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseMessage([]byte(tt.raw))
			if err == nil {
				t.Fatalf("ParseMessage() = %#v, nil error; want malformed payload error", msg)
			}
		})
	}
}

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
}

func TestParseStopIsIgnored(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "codex-hooks", "stop.json"))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseMessage(data)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if !reflect.DeepEqual(msg, notify.Message{}) {
		t.Fatalf("ParseMessage() = %#v, want zero message", msg)
	}
}

func TestParsePlanModeStopIsIgnored(t *testing.T) {
	raw := []byte(`{"hook_event_name":"Stop","permission_mode":"plan","session_id":"s","cwd":"/tmp","last_assistant_message":"plan ready"}`)

	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if !reflect.DeepEqual(msg, notify.Message{}) {
		t.Fatalf("ParseMessage() = %#v, want zero message", msg)
	}
}

func TestParseRequestUserInputPreservesQuestionsAndMetadata(t *testing.T) {
	raw := []byte(`{
  "hook_event_name": "PreToolUse",
  "session_id": "session-input",
  "cwd": "/tmp/demo",
  "permission_mode": "plan",
  "tool_name": "request_user_input",
  "tool_use_id": "call-input-1",
  "tool_input": {
    "questions": [
      {
        "id": "environment",
        "header": "环境",
        "question": "选择部署环境 🌏",
        "isOther": true,
        "isSecret": false,
        "options": [
          {"label": "生产", "description": "面向真实用户"},
          {"label": "测试", "description": "仅用于验证"}
        ]
      },
      {
        "id": "token",
        "header": "令牌",
        "question": "请输入访问令牌",
        "isSecret": true,
        "options": []
      }
    ],
    "autoResolutionMs": 120000
  }
}`)

	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Agent != "codex" || msg.Event != "input_required" {
		t.Fatalf("message identity = (%q, %q), want (codex, input_required)", msg.Agent, msg.Event)
	}
	if msg.SessionID != "session-input" || msg.DedupeID != "call-input-1" {
		t.Fatalf("message IDs = (%q, %q), want (session-input, call-input-1)", msg.SessionID, msg.DedupeID)
	}
	if msg.Workspace != "/tmp/demo" {
		t.Fatalf("Workspace = %q, want /tmp/demo", msg.Workspace)
	}
	if msg.AutoResolutionMs != 120000 {
		t.Fatalf("AutoResolutionMs = %d, want 120000", msg.AutoResolutionMs)
	}
	if len(msg.Questions) != 2 {
		t.Fatalf("question count = %d, want 2", len(msg.Questions))
	}

	first := msg.Questions[0]
	if first.ID != "environment" || first.Header != "环境" || first.Question != "选择部署环境 🌏" {
		t.Fatalf("first question = %#v", first)
	}
	if !first.IsOther || first.IsSecret {
		t.Fatalf("first question flags = (isOther=%v, isSecret=%v), want (true, false)", first.IsOther, first.IsSecret)
	}
	if len(first.Options) != 2 {
		t.Fatalf("first option count = %d, want 2", len(first.Options))
	}
	if first.Options[0].Label != "生产" || first.Options[0].Description != "面向真实用户" {
		t.Fatalf("first option = %#v", first.Options[0])
	}

	second := msg.Questions[1]
	if !second.IsSecret || len(second.Options) != 0 {
		t.Fatalf("second question = %#v, want secret question with no options", second)
	}
	for _, want := range []string{"选择部署环境 🌏", "生产", "面向真实用户", "其他（自由输入）", "敏感输入", "120000", "请回到 Codex 终端提交答案"} {
		if !strings.Contains(msg.Body, want) {
			t.Errorf("Body = %q, missing %q", msg.Body, want)
		}
	}
}

func TestParseRequestUserInputWorksAcrossPermissionModes(t *testing.T) {
	for _, mode := range []string{"default", "acceptEdits", "plan", "dontAsk", "bypassPermissions"} {
		t.Run(mode, func(t *testing.T) {
			raw := []byte(`{"hook_event_name":"PreToolUse","permission_mode":"` + mode + `","tool_name":"request_user_input","tool_use_id":"call-` + mode + `","tool_input":{"questions":[{"id":"q","question":"继续？","options":[{"label":"是","description":"继续执行"}]}]}}`)
			msg, err := ParseMessage(raw)
			if err != nil {
				t.Fatalf("ParseMessage() error = %v", err)
			}
			if msg.Event != "input_required" || msg.DedupeID != "call-"+mode {
				t.Fatalf("message = %#v, want input_required with matching DedupeID", msg)
			}
		})
	}
}

func TestParseRequestUserInputAllowsQuestionWithoutOptions(t *testing.T) {
	raw := []byte(`{"hook_event_name":"PreToolUse","tool_name":"request_user_input","tool_input":{"questions":[{"id":"freeform","question":"请说明原因"}]}}`)

	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if len(msg.Questions) != 1 || len(msg.Questions[0].Options) != 0 {
		t.Fatalf("Questions = %#v, want one question with no options", msg.Questions)
	}
}

func TestParsePreToolUseIgnoresNonRequestUserInput(t *testing.T) {
	raw := []byte(`{"hook_event_name":"PreToolUse","tool_name":"shell","tool_input":"not an object"}`)

	msg, err := ParseMessage(raw)
	if err != nil {
		t.Fatalf("ParseMessage() error = %v, want ignored event", err)
	}
	if msg.Agent != "" || msg.Event != "" || msg.SessionID != "" || msg.DedupeID != "" || msg.Body != "" || len(msg.Questions) != 0 {
		t.Fatalf("message = %#v, want zero message for non-request_user_input", msg)
	}
}

func TestParseRequestUserInputRejectsMalformedPayload(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{
			name: "missing tool input",
			raw:  `{"hook_event_name":"PreToolUse","tool_name":"request_user_input"}`,
		},
		{
			name: "questions wrong type",
			raw:  `{"hook_event_name":"PreToolUse","tool_name":"request_user_input","tool_input":{"questions":"nope"}}`,
		},
		{
			name: "option wrong type",
			raw:  `{"hook_event_name":"PreToolUse","tool_name":"request_user_input","tool_input":{"questions":[{"question":"q","options":"nope"}]}}`,
		},
		{
			name: "secret wrong type",
			raw:  `{"hook_event_name":"PreToolUse","tool_name":"request_user_input","tool_input":{"questions":[{"question":"q","isSecret":"yes"}]}}`,
		},
		{
			name: "auto resolution wrong type",
			raw:  `{"hook_event_name":"PreToolUse","tool_name":"request_user_input","tool_input":{"questions":[{"question":"q"}],"autoResolutionMs":"120000"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseMessage([]byte(tt.raw)); err == nil {
				t.Fatal("ParseMessage() error = nil, want malformed payload error")
			}
		})
	}
}

func TestParseUnsupportedEvent(t *testing.T) {
	raw := []byte(`{"hook_event_name":"UserPromptSubmit","session_id":"s","cwd":"/tmp"}`)

	_, err := ParseMessage(raw)
	if err == nil {
		t.Fatal("ParseMessage() expected error for unsupported event")
	}
}
