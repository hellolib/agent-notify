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

func TestParseFailedTool(t *testing.T) {
	msg, err := ParseMessage(strings.NewReader(`{"type":"tool_execution_end","session_id":"s3","tool_name":"write","is_error":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if msg.Event != "run_failed" || !strings.Contains(msg.Body, "write") {
		t.Fatalf("unexpected failed-tool message: %+v", msg)
	}
}

func TestParseSessionStopSkipsContinuationAndAbort(t *testing.T) {
	for _, raw := range []string{
		`{"type":"session_stop","stop_hook_active":true}`,
		`{"type":"session_stop","signal_aborted":true}`,
	} {
		if _, err := ParseMessage(strings.NewReader(raw)); err == nil {
			t.Fatalf("ParseMessage(%s) should skip", raw)
		}
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
