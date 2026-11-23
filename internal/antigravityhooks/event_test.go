package antigravityhooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readFixture(t *testing.T, name string) *os.File {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "testdata", "antigravity-hooks", name))
	if err != nil {
		t.Fatalf("open fixture %s: %v", name, err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func TestParseMessage_SessionStart(t *testing.T) {
	msg, err := ParseMessage(readFixture(t, "session_start.json"))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Agent != "antigravity" {
		t.Errorf("Agent = %q, want antigravity", msg.Agent)
	}
	if msg.Event != "session_start" {
		t.Errorf("Event = %q, want session_start", msg.Event)
	}
	if msg.SessionID != "ec33ebf9-0cba-4100-8142-c61503f6c587" {
		t.Errorf("SessionID = %q", msg.SessionID)
	}
	if msg.Workspace != "/workspace/project" {
		t.Errorf("Workspace = %q, want /workspace/project", msg.Workspace)
	}
}

func TestParseMessage_PreToolUseAskPermission(t *testing.T) {
	msg, err := ParseMessage(readFixture(t, "pre_tool_use_ask_permission.json"))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Agent != "antigravity" || msg.Event != "permission_required" {
		t.Fatalf("got Agent=%q Event=%q, want antigravity/permission_required", msg.Agent, msg.Event)
	}
	if msg.Title != "Antigravity 等待授权" {
		t.Errorf("Title = %q, want Antigravity 等待授权", msg.Title)
	}
	if !strings.Contains(msg.Body, "write /etc/hosts") || !strings.Contains(msg.Body, "Update local dev host mapping") {
		t.Errorf("Body = %q, want permission target and reason", msg.Body)
	}
}

func TestParseMessage_PreToolUseAskQuestion(t *testing.T) {
	msg, err := ParseMessage(readFixture(t, "pre_tool_use_ask_question.json"))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Agent != "antigravity" || msg.Event != "input_required" {
		t.Fatalf("got Agent=%q Event=%q, want antigravity/input_required", msg.Agent, msg.Event)
	}
	if msg.Title != "Antigravity 等待输入" {
		t.Errorf("Title = %q, want Antigravity 等待输入", msg.Title)
	}
	if !strings.Contains(msg.Body, "Which database migration strategy should I use?") {
		t.Errorf("Body = %q, want question text", msg.Body)
	}
}

func TestParseMessage_PreToolUseOtherToolSkipped(t *testing.T) {
	input := `{"hookEventName":"PreToolUse","conversationId":"s1","toolCall":{"name":"run_command","args":{"CommandLine":"ls"}}}`
	eventName, _, err := parseHook(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected skip error for non-interactive tool")
	}
	if eventName != "PreToolUse" {
		t.Errorf("eventName = %q, want PreToolUse", eventName)
	}
}

func TestParseMessage_StopCompleted(t *testing.T) {
	msg, err := ParseMessage(readFixture(t, "stop.json"))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "run_completed" {
		t.Errorf("Event = %q, want run_completed", msg.Event)
	}
	if msg.Title != "Antigravity 运行完成" {
		t.Errorf("Title = %q, want Antigravity 运行完成", msg.Title)
	}
}

func TestParseMessage_StopError(t *testing.T) {
	msg, err := ParseMessage(readFixture(t, "stop_error.json"))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "run_failed" {
		t.Errorf("Event = %q, want run_failed", msg.Event)
	}
	if !strings.Contains(msg.Body, "upstream model timeout") {
		t.Errorf("Body = %q, want error message", msg.Body)
	}
}

func TestParseMessage_StopFailureReasonsWithoutErrorString(t *testing.T) {
	for _, reason := range []string{"MAX_INVOCATIONS", "MAX_TOKEN_BUDGET_EXCEEDED", "max_steps_exceeded", "ERROR"} {
		input := `{"hookEventName":"Stop","conversationId":"s1","terminationReason":"` + reason + `","fullyIdle":true}`
		msg, err := ParseMessage(strings.NewReader(input))
		if err != nil {
			t.Fatalf("reason=%s: ParseMessage() error = %v", reason, err)
		}
		if msg.Event != "run_failed" {
			t.Errorf("reason=%s: Event = %q, want run_failed", reason, msg.Event)
		}
	}
}

func TestParseMessage_StopUserCanceledSkipped(t *testing.T) {
	eventName, _, err := parseHook(readFixture(t, "stop_canceled.json"))
	if err == nil {
		t.Fatal("expected USER_CANCELED without error to be skipped")
	}
	if eventName != "Stop" {
		t.Errorf("eventName = %q, want Stop", eventName)
	}
}

func TestParseMessage_InfersEventAndWorkspaceWhenFieldsOmitted(t *testing.T) {
	// 兼容未携带 hookEventName 和 cwd 的旧版 payload（只带 workspacePaths 和事件专属字段）。
	cases := []struct {
		name      string
		input     string
		wantEvent string
	}{
		{
			name:      "session_start",
			input:     `{"conversationId":"c1","workspacePaths":["/repo/a"]}`,
			wantEvent: "session_start",
		},
		{
			name:      "pre_tool_use",
			input:     `{"conversationId":"c1","workspacePaths":["/repo/a"],"toolCall":{"name":"ask_question","args":{"questions":[{"question":"Continue?"}]}}}`,
			wantEvent: "input_required",
		},
		{
			name:      "stop",
			input:     `{"conversationId":"c1","workspacePaths":["/repo/a"],"executionNum":1,"terminationReason":"model_stop","fullyIdle":true}`,
			wantEvent: "run_completed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := ParseMessage(strings.NewReader(tc.input))
			if err != nil {
				t.Fatalf("ParseMessage() error = %v", err)
			}
			if msg.Event != tc.wantEvent {
				t.Errorf("Event = %q, want %q", msg.Event, tc.wantEvent)
			}
			if msg.Workspace != "/repo/a" {
				t.Errorf("Workspace = %q, want /repo/a", msg.Workspace)
			}
		})
	}
}

func TestResponseJSON(t *testing.T) {
	if got := ResponseJSON("PreToolUse"); got != "{\"decision\":\"allow\"}\n" {
		t.Errorf("PreToolUse response = %q", got)
	}
	if got := ResponseJSON("Stop"); got != "{\"decision\":\"allow\"}\n" {
		t.Errorf("Stop response = %q", got)
	}
	if got := ResponseJSON("SessionStart"); got != "{}\n" {
		t.Errorf("SessionStart response = %q", got)
	}
	if got := ResponseJSON(""); got != "{}\n" {
		t.Errorf("empty event response = %q", got)
	}
}

func TestParseMessage_StopExtractsLastAssistantReplyFromTranscript(t *testing.T) {
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	lines := strings.Join([]string{
		`{"step_index":1,"source":"USER_EXPLICIT","type":"USER_INPUT","content":"old prompt"}`,
		`{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","content":"old reply"}`,
		`{"step_index":3,"source":"USER_EXPLICIT","type":"USER_INPUT","content":"fix unit tests"}`,
		`{"step_index":4,"source":"MODEL","type":"PLANNER_RESPONSE","thinking":"running tests"}`,
		`{"step_index":5,"source":"MODEL","type":"PLANNER_RESPONSE","content":"已修复全部 3 个失败用例，go test ./... 全部通过。"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(transcript, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}

	input := `{"hookEventName":"Stop","conversationId":"c1","cwd":"/repo","terminationReason":"NO_TOOL_CALL","fullyIdle":true,"lastUserInput":"fix unit tests","transcriptPath":` + jsonQuote(t, transcript) + `}`
	msg, err := ParseMessage(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	want := "已修复全部 3 个失败用例，go test ./... 全部通过。"
	if msg.Body != want {
		t.Errorf("Body = %q, want %q", msg.Body, want)
	}
}

func TestParseMessage_StopDoesNotLeakPreviousTurnReplyAndFallsBackToUserInput(t *testing.T) {
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	lines := strings.Join([]string{
		`{"step_index":1,"source":"USER_EXPLICIT","type":"USER_INPUT","content":"old prompt"}`,
		`{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","content":"stale previous reply"}`,
		`{"step_index":3,"source":"USER_EXPLICIT","type":"USER_INPUT","content":"run build"}`,
		`{"step_index":4,"source":"MODEL","type":"PLANNER_RESPONSE","thinking":"tool call only"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(transcript, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}

	input := `{"hookEventName":"Stop","conversationId":"c1","cwd":"/repo","terminationReason":"NO_TOOL_CALL","fullyIdle":true,"lastUserInput":"run build","transcriptPath":` + jsonQuote(t, transcript) + `}`
	msg, err := ParseMessage(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if strings.Contains(msg.Body, "stale previous reply") {
		t.Fatalf("Body leaked previous turn reply: %q", msg.Body)
	}
	if msg.Body != "任务已完成：run build" {
		t.Errorf("Body = %q, want %q", msg.Body, "任务已完成：run build")
	}
}

func TestParseMessage_StopHandlesLargeTranscriptTail(t *testing.T) {
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	var b strings.Builder
	// 写入超过 maxTranscriptTailBytes 的历史记录，验证尾部 seek 仍能准确提取最新回复。
	padding := strings.Repeat("x", 4096)
	for i := 0; i < 80; i++ {
		b.WriteString(`{"type":"GENERIC","content":"`)
		b.WriteString(padding)
		b.WriteString("\"}\n")
	}
	b.WriteString(`{"type":"USER_INPUT","content":"check status"}` + "\n")
	b.WriteString(`{"type":"PLANNER_RESPONSE","content":"部署已完成，所有健康检查通过。"}` + "\n")
	if err := os.WriteFile(transcript, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}

	input := `{"hookEventName":"Stop","conversationId":"c1","terminationReason":"NO_TOOL_CALL","transcriptPath":` + jsonQuote(t, transcript) + `}`
	msg, err := ParseMessage(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Body != "部署已完成，所有健康检查通过。" {
		t.Errorf("Body = %q, want latest reply", msg.Body)
	}
}

func jsonQuote(t *testing.T, s string) string {
	t.Helper()
	// 构造合法 JSON 字符串字面量（兼容 Windows 反斜杠路径）。
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + r.Replace(s) + `"`
}
