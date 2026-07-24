package agenthooks

import (
	"testing"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/notify"
)

func TestApplyFocusCacheRoutesByPlatform(t *testing.T) {
	cases := []struct {
		goos        string
		wantWindow  string
		wantCapture string
	}{
		{"linux", "123", ""},
		{"darwin", "", "123"},
		{"windows", "", "123"},
		{"freebsd", "", ""},
	}
	for _, c := range cases {
		msg := notify.Message{}
		applyFocusCache(&msg, c.goos, "123")
		if msg.FocusWindowID != c.wantWindow || msg.FocusCapture != c.wantCapture {
			t.Fatalf("%s: got FocusWindowID=%q FocusCapture=%q, want %q/%q",
				c.goos, msg.FocusWindowID, msg.FocusCapture, c.wantWindow, c.wantCapture)
		}
	}
}

func TestBuildSendersUsesClaudeCodeConfigByDefault(t *testing.T) {
	cfg := config.Default()
	cfg.Notify.ClaudeCode.Channels.System.Enabled = true
	cfg.Notify.ClaudeCode.Channels.Feishu.Enabled = true
	cfg.Notify.ClaudeCode.Events = []string{"run_completed"}
	cfg.Notify.Codex.Channels.System.Enabled = false
	cfg.Notify.Codex.Channels.Feishu.Enabled = false

	senders := buildSenders(cfg, notify.Message{Event: "run_completed"})

	if len(senders) != 2 {
		t.Fatalf("len(senders) = %d, want 2", len(senders))
	}
	if senders[0].Name() != "system" {
		t.Fatalf("senders[0] = %q, want system", senders[0].Name())
	}
	if senders[1].Name() != "feishu" {
		t.Fatalf("senders[1] = %q, want feishu", senders[1].Name())
	}
}

func TestBuildSendersUsesCodexConfigForCodexMessages(t *testing.T) {
	cfg := config.Default()
	cfg.Notify.ClaudeCode.Channels.System.Enabled = true
	cfg.Notify.ClaudeCode.Channels.Feishu.Enabled = true
	cfg.Notify.ClaudeCode.Events = []string{"run_completed"}
	cfg.Notify.Codex.Channels.System.Enabled = true
	cfg.Notify.Codex.Channels.Feishu.Enabled = false
	cfg.Notify.Codex.Events = []string{"run_completed"}

	senders := buildSenders(cfg, notify.Message{Agent: "codex", Event: "run_completed"})

	if len(senders) != 1 {
		t.Fatalf("len(senders) = %d, want 1", len(senders))
	}
	if senders[0].Name() != "system" {
		t.Fatalf("senders[0] = %q, want system", senders[0].Name())
	}
}

func TestBuildSendersFiltersCodexEventsNotSelected(t *testing.T) {
	cfg := config.Default()
	cfg.Notify.Codex.Channels.System.Enabled = true
	cfg.Notify.Codex.Channels.Feishu.Enabled = true
	// 用户只订阅了 permission_required，run_completed 不应触发任何 sender
	cfg.Notify.Codex.Events = []string{"permission_required"}

	senders := buildSenders(cfg, notify.Message{Agent: "codex", Event: "run_completed"})

	if len(senders) != 0 {
		t.Fatalf("len(senders) = %d, want 0 (event not subscribed)", len(senders))
	}
}

func TestBuildSendersSendsSubscribedCodexEvent(t *testing.T) {
	cfg := config.Default()
	cfg.Notify.Codex.Channels.System.Enabled = true
	cfg.Notify.Codex.Channels.Feishu.Enabled = true
	cfg.Notify.Codex.Events = []string{"permission_required", "run_completed"}

	senders := buildSenders(cfg, notify.Message{Agent: "codex", Event: "permission_required"})

	if len(senders) != 2 {
		t.Fatalf("len(senders) = %d, want 2", len(senders))
	}
}

func TestBuildSendersUsesGrokConfigForGrokMessages(t *testing.T) {
	cfg := config.Default()
	cfg.Notify.ClaudeCode.Channels.System.Enabled = true
	cfg.Notify.ClaudeCode.Events = []string{"run_completed"}
	cfg.Notify.Grok.Channels.System.Enabled = true
	cfg.Notify.Grok.Channels.Feishu.Enabled = false
	cfg.Notify.Grok.Events = []string{"run_completed"}

	senders := buildSenders(cfg, notify.Message{Agent: "grok", Event: "run_completed"})

	if len(senders) != 1 {
		t.Fatalf("len(senders) = %d, want 1", len(senders))
	}
	if senders[0].Name() != "system" {
		t.Fatalf("senders[0] = %q, want system", senders[0].Name())
	}
}

func TestBuildSendersAddsBarkForCodex(t *testing.T) {
	cfg := config.Default()
	cfg.Notify.Codex.Channels.Bark.Enabled = true
	cfg.Notify.Codex.Channels.Bark.WebhookURL = "https://api.day.app/key"
	cfg.Notify.Codex.Events = []string{"run_completed"}

	senders := buildSenders(cfg, notify.Message{Agent: "codex", Event: "run_completed"})

	if len(senders) != 1 {
		t.Fatalf("len(senders) = %d, want 1", len(senders))
	}
	if senders[0].Name() != "bark" {
		t.Fatalf("senders[0] = %q, want bark", senders[0].Name())
	}
}
