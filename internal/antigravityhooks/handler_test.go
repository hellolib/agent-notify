package antigravityhooks

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/config"
)

func TestHandleWritesRequiredJSONResponse(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	logPath := filepath.Join(dir, "agent-notify.log")
	cfg := config.Default()

	cases := []struct {
		name         string
		input        string
		wantDecision string
	}{
		{
			name:         "SessionStart returns empty JSON object",
			input:        `{"hookEventName":"SessionStart","conversationId":"c1","cwd":"/tmp"}`,
			wantDecision: "",
		},
		{
			name:         "PreToolUse ask_question returns allow decision",
			input:        `{"hookEventName":"PreToolUse","conversationId":"c1","toolCall":{"name":"ask_question","args":{"questions":[{"question":"Proceed?"}]}}}`,
			wantDecision: "allow",
		},
		{
			name:         "PreToolUse skipped tool still returns allow decision",
			input:        `{"hookEventName":"PreToolUse","conversationId":"c1","toolCall":{"name":"run_command","args":{"CommandLine":"ls"}}}`,
			wantDecision: "allow",
		},
		{
			name:         "Stop returns allow decision",
			input:        `{"hookEventName":"Stop","conversationId":"c1","terminationReason":"NO_TOOL_CALL","fullyIdle":true}`,
			wantDecision: "allow",
		},
		{
			name:         "Stop USER_CANCELED skipped still returns allow decision",
			input:        `{"hookEventName":"Stop","conversationId":"c1","terminationReason":"USER_CANCELED","fullyIdle":true}`,
			wantDecision: "allow",
		},
		{
			name:         "Malformed JSON still returns valid JSON",
			input:        `not-json`,
			wantDecision: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			if err := Handle(context.Background(), cfg, statePath, logPath, strings.NewReader(tc.input), &stdout); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			var resp map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
				t.Fatalf("stdout is not valid JSON: %q (err=%v)", stdout.String(), err)
			}
			gotDecision, _ := resp["decision"].(string)
			if gotDecision != tc.wantDecision {
				t.Errorf("decision = %q, want %q (stdout=%q)", gotDecision, tc.wantDecision, stdout.String())
			}
		})
	}
}
