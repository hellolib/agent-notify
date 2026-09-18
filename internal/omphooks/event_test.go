package omphooks

import (
	"strings"
	"testing"
)

func TestParseSessionStart(t *testing.T) {
	msg, err := ParseMessage(strings.NewReader(`{"type":"session_start","session_id":"s1","cwd":"/tmp/demo"}`))
	if err != nil {
		t.Fatal(err)
	}
	if msg.Agent != "omp" || msg.Event != "session_start" || msg.SessionID != "s1" || msg.Workspace != "/tmp/demo" {
		t.Fatalf("unexpected message: %+v", msg)
	}
}

func TestParseApprovalRequest(t *testing.T) {
	msg, err := ParseMessage(strings.NewReader(`{"type":"tool_approval_requested","session_id":"s2","cwd":"/repo","tool_name":"bash","reason":"needs permission"}`))
	if err != nil {
		t.Fatal(err)
	}
	if msg.Event != "permission_required" || !strings.Contains(msg.Body, "bash") || !strings.Contains(msg.Body, "needs permission") {
		t.Fatalf("unexpected approval message: %+v", msg)
	}
}

// 单个工具报错不构成「运行失败」通知，只静默跳过。
func TestParseToolExecutionEndIsSkipped(t *testing.T) {
	if _, err := ParseMessage(strings.NewReader(`{"type":"tool_execution_end","session_id":"s3","tool_name":"write","is_error":true}`)); err == nil {
		t.Fatal("tool_execution_end should be skipped")
	}
}

func TestParseQuestionAskedMapsToInputRequired(t *testing.T) {
	msg, err := ParseMessage(strings.NewReader(`{"type":"question_asked","session_id":"s3","cwd":"/repo","tool_name":"ask","question":"Which color do you prefer?"}`))
	if err != nil {
		t.Fatal(err)
	}
	if msg.Event != "input_required" || !strings.Contains(msg.Body, "Which color") || msg.SessionID != "s3" {
		t.Fatalf("unexpected input_required message: %+v", msg)
	}
}

// stop_reason 判定运行成败：error 必失败；aborted 需带错误信息才算失败。
func TestParseSessionStopFailureClassification(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"error stop reason reports failure", `{"type":"session_stop","stop_reason":"error","error_message":"401 invalid key"}`, "run_failed"},
		{"aborted with message reports failure", `{"type":"session_stop","stop_reason":"aborted","error_message":"interrupted"}`, "run_failed"},
		{"aborted without message is completed", `{"type":"session_stop","stop_reason":"aborted"}`, "run_completed"},
		{"end_turn is completed", `{"type":"session_stop","stop_reason":"end_turn"}`, "run_completed"},
		{"missing stop reason is completed", `{"type":"session_stop"}`, "run_completed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := ParseMessage(strings.NewReader(tc.raw))
			if err != nil {
				t.Fatal(err)
			}
			if msg.Event != tc.want {
				t.Fatalf("event = %q, want %q", msg.Event, tc.want)
			}
		})
	}
}

func TestParseSessionStop(t *testing.T) {
	msg, err := ParseMessage(strings.NewReader(`{"type":"session_stop","session_id":"s4","cwd":"/repo"}`))
	if err != nil {
		t.Fatal(err)
	}
	if msg.Event != "run_completed" || msg.SessionID != "s4" {
		t.Fatalf("unexpected stop message: %+v", msg)
	}
}
