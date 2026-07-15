package notify

import "testing"

func TestFormatTitle(t *testing.T) {
	tests := []struct {
		name  string
		agent string
		event string
		want  string
	}{
		{name: "claude permission", agent: "claude_code", event: "permission_required", want: "Claude Code 等待授权"},
		{name: "claude failed", agent: "claude_code", event: "run_failed", want: "Claude Code 运行失败"},
		{name: "codex completed", agent: "codex", event: "run_completed", want: "Codex 运行完成"},
		{name: "grok completed", agent: "grok", event: "run_completed", want: "Grok 运行完成"},
		{name: "grok session", agent: "grok", event: "session_start", want: "Grok 会话开始"},
	}

	for _, tt := range tests {
		if got := FormatTitle(tt.agent, tt.event); got != tt.want {
			t.Fatalf("%s: FormatTitle() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestDefaultBody(t *testing.T) {
	if got := DefaultBody("run_completed"); got != "任务已完成，请查看结果" {
		t.Fatalf("DefaultBody() = %q, want %q", got, "任务已完成，请查看结果")
	}
}
