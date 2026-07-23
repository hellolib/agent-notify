package codexhooks

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/config"
)

type codexFailingReader struct{}

func (codexFailingReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func TestHandleFailuresNeverBlockCodex(t *testing.T) {
	tests := []struct {
		name  string
		input io.Reader
	}{
		{name: "stdin read", input: codexFailingReader{}},
		{name: "malformed payload", input: strings.NewReader(`{"hook_event_name":"PreToolUse","tool_name":"request_user_input","tool_input":{"questions":"bad"}}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Handle(context.Background(), config.Default(), filepath.Join(t.TempDir(), "state.json"), filepath.Join(t.TempDir(), "hook.log"), tt.input); err != nil {
				t.Fatalf("Handle() error = %v, want nil", err)
			}
		})
	}
}

func TestHandleDispatchFailureDoesNotBlockCodex(t *testing.T) {
	cfg := config.Default()
	cfg.Notify.Codex.Events = []string{"input_required"}
	cfg.Notify.Codex.Channels.System.Enabled = true

	// Passing a directory as the state file makes the dispatcher fail before
	// invoking a platform notifier. The hook must still return success.
	statePath := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "hook.log")
	payload := strings.NewReader(`{"hook_event_name":"PreToolUse","tool_name":"request_user_input","tool_use_id":"call-1","tool_input":{"questions":[{"question":"继续？"}]}}`)
	if err := Handle(context.Background(), cfg, statePath, logPath, payload); err != nil {
		t.Fatalf("Handle() error = %v, want nil", err)
	}
}

func TestHandleIgnoresOtherPreToolUseTools(t *testing.T) {
	payload := strings.NewReader(`{"hook_event_name":"PreToolUse","tool_name":"shell","tool_input":{"command":"echo hi"}}`)
	if err := Handle(context.Background(), config.Default(), filepath.Join(t.TempDir(), "state.json"), filepath.Join(t.TempDir(), "hook.log"), payload); err != nil {
		t.Fatalf("Handle() error = %v, want nil", err)
	}
}
