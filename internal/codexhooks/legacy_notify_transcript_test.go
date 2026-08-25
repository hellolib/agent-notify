package codexhooks

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInspectLegacyNotifyTranscriptsTracksExactTurn(t *testing.T) {
	now := time.Date(2026, 8, 25, 11, 31, 30, 0, time.UTC)
	home := writeLegacyTranscript(t, now, `C:\work\demo`, []map[string]any{
		legacyTranscriptEvent("2026-08-25T11:31:22.495Z", "task_started", "turn-1", ""),
	})
	p := legacyNotifyPayload{CWD: `C:\work\demo`, TurnID: "turn-1"}

	completion, err := inspectLegacyNotifyTranscripts(home, p, now)
	if err != nil {
		t.Fatal(err)
	}
	if completion.State != legacyTurnRunning {
		t.Fatalf("State = %v, want running", completion.State)
	}

	home = writeLegacyTranscript(t, now, `C:\work\demo`, []map[string]any{
		legacyTranscriptEvent("2026-08-25T11:31:22.495Z", "task_started", "turn-1", ""),
		legacyTranscriptEvent("2026-08-25T11:31:30.100Z", "task_complete", "turn-1", "verified final"),
	})
	completion, err = inspectLegacyNotifyTranscripts(home, p, now)
	if err != nil {
		t.Fatal(err)
	}
	if completion.State != legacyTurnComplete || completion.LastAgentMessage != "verified final" {
		t.Fatalf("completion = %+v, want completed transcript message", completion)
	}
}

func TestInspectLegacyNotifyTranscriptsCorrelatesVSCodeStartupTurn(t *testing.T) {
	now := time.Date(2026, 8, 25, 11, 31, 30, 0, time.UTC)
	home := writeLegacyTranscript(t, now, `C:\work\demo`, []map[string]any{
		legacyTranscriptEvent("2026-08-25T11:31:22.495Z", "task_started", "actual-turn", ""),
	})
	p := legacyNotifyPayload{
		ThreadID: "01a038b0-6fa4-7413-a77f-4bff2a187d53",
		TurnID:   "startup-turn-not-in-transcript",
		CWD:      `C:\work\demo`,
	}

	completion, err := inspectLegacyNotifyTranscripts(home, p, now)
	if err != nil {
		t.Fatal(err)
	}
	if completion.State != legacyTurnRunning {
		t.Fatalf("State = %v, want correlated running turn", completion.State)
	}
}

func TestInspectLegacyNotifyTranscriptsDoesNotCorrelateUnrelatedTask(t *testing.T) {
	now := time.Date(2026, 8, 25, 11, 31, 30, 0, time.UTC)
	home := writeLegacyTranscript(t, now, `C:\work\demo`, []map[string]any{
		legacyTranscriptEvent("2026-08-25T11:20:00Z", "task_started", "other-turn", ""),
	})
	p := legacyNotifyPayload{
		ThreadID: "01a038b0-6fa4-7413-a77f-4bff2a187d53",
		TurnID:   "unknown-turn",
		CWD:      `C:\work\demo`,
	}

	completion, err := inspectLegacyNotifyTranscripts(home, p, now)
	if err != nil {
		t.Fatal(err)
	}
	if completion.State != legacyTurnUnknown {
		t.Fatalf("State = %v, want unknown", completion.State)
	}
}

func TestWaitForLegacyNotifyCompletionFallsBackWithoutHistory(t *testing.T) {
	p := legacyNotifyPayload{CWD: `C:\work\demo`, TurnID: "turn-1"}
	completion, err := waitForLegacyNotifyCompletion(context.Background(), t.TempDir(), p, 0)
	if err != nil {
		t.Fatal(err)
	}
	if completion.State != legacyTurnUnknown {
		t.Fatalf("State = %v, want unknown", completion.State)
	}
}

func TestWaitForLegacyNotifyCompletionObservesDelayedTaskComplete(t *testing.T) {
	now := time.Now()
	home := writeLegacyTranscript(t, now, `C:\work\demo`, []map[string]any{
		legacyTranscriptEvent(now.Add(-time.Second).Format(time.RFC3339Nano), "task_started", "turn-1", ""),
	})
	path := filepath.Join(home, "sessions", "2026", "08", "25", "rollout-test.jsonl")
	writeDone := make(chan error, 1)
	go func() {
		time.Sleep(150 * time.Millisecond)
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err == nil {
			err = json.NewEncoder(f).Encode(legacyTranscriptEvent(
				time.Now().Format(time.RFC3339Nano),
				"task_complete",
				"turn-1",
				"fresh final body",
			))
			closeErr := f.Close()
			if err == nil {
				err = closeErr
			}
		}
		writeDone <- err
	}()

	p := legacyNotifyPayload{CWD: `C:\work\demo`, TurnID: "turn-1"}
	completion, err := waitForLegacyNotifyCompletion(context.Background(), home, p, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
	if completion.State != legacyTurnComplete || completion.LastAgentMessage != "fresh final body" {
		t.Fatalf("completion = %+v, want delayed task completion", completion)
	}
}

func TestVSCodeNotifyClient(t *testing.T) {
	for _, client := range []string{"codex_vscode", "codex-vscode", "VSCode"} {
		if !isVSCodeNotifyClient(client) {
			t.Fatalf("isVSCodeNotifyClient(%q) = false", client)
		}
	}
	if isVSCodeNotifyClient("codex-tui") {
		t.Fatal("codex-tui should not use transcript verification")
	}
}

func writeLegacyTranscript(
	t *testing.T,
	now time.Time,
	workspace string,
	events []map[string]any,
) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, "sessions", "2026", "08", "25")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-test.jsonl")
	lines := []map[string]any{
		{
			"timestamp": "2026-08-25T11:31:21Z",
			"type":      "session_meta",
			"payload": map[string]any{
				"cwd": workspace,
			},
		},
	}
	lines = append(lines, events...)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(f)
	for _, line := range lines {
		if err := encoder.Encode(line); err != nil {
			_ = f.Close()
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatal(err)
	}
	return home
}

func legacyTranscriptEvent(timestamp, eventType, turnID, lastAgentMessage string) map[string]any {
	payload := map[string]any{
		"type":    eventType,
		"turn_id": turnID,
	}
	if eventType == "task_complete" {
		payload["last_agent_message"] = lastAgentMessage
	}
	return map[string]any{
		"timestamp": timestamp,
		"type":      "event_msg",
		"payload":   payload,
	}
}
