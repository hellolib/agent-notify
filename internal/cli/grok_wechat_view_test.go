package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/testutil"
)

// Regression: configuring only Grok + WeChat must not mark other agents as configured
// in view config (previously Default() pre-enabled System for Claude/ZCode/Grok, and
// channel-menu init enabled webhooks on every agent).
func TestRunInitGrokWechatOnlyShowsGrokConfiguredInView(t *testing.T) {
	dir := testutil.IsolateHome(t)
	configPath := filepath.Join(dir, ".agent-notify", "config.yaml")

	useFakePrompter(t, &fakePrompter{
		selects: []string{"grok"},
		multi: [][]string{
			{"wechat"},
			{"session_start", "permission_required", "input_required", "run_completed", "run_failed"},
		},
		inputs: []string{"https://push.example.com/api/notify/only-grok"},
	})

	if err := Run(context.Background(), []string{"init", "--config", configPath, "--binary", "/tmp/agent-notify"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !got.Notify.Grok.Channels.Wechat.Enabled {
		t.Fatal("Grok wechat should be enabled")
	}
	if got.Notify.ClaudeCode.Channels.Wechat.Enabled || got.Notify.ClaudeCode.Channels.System.Enabled {
		t.Fatal("Claude must not show any channel enabled after Grok-only setup")
	}
	if got.Notify.Codex.Channels.Wechat.Enabled || got.Notify.Codex.Channels.System.Enabled {
		t.Fatal("Codex must not show any channel enabled after Grok-only setup")
	}
	if got.Notify.ZCode.Channels.Wechat.Enabled || got.Notify.ZCode.Channels.System.Enabled {
		t.Fatal("ZCode must not show any channel enabled after Grok-only setup")
	}
	if got.Agent.ClaudeCode.Enabled {
		t.Fatal("Claude agent should remain disabled")
	}
	if !got.Agent.Grok.Enabled {
		t.Fatal("Grok agent should be enabled")
	}

	// Channel-menu wechat after Grok-only setup must still only enable Grok.
	if err := config.Save(mustDefaultPath(t), got); err != nil {
		t.Fatalf("Save default path: %v", err)
	}
	if err := runInitWechat(Streams{Stdout: &bytes.Buffer{}}, &fakePrompter{inputs: []string{"https://push.example.com/api/notify/only-grok"}}); err != nil {
		t.Fatalf("runInitWechat: %v", err)
	}
	after, err := config.Load(mustDefaultPath(t))
	if err != nil {
		t.Fatalf("Load after channel menu: %v", err)
	}
	if after.Notify.ClaudeCode.Channels.Wechat.Enabled {
		t.Fatal("channel-menu wechat must not enable Claude when only Grok is configured")
	}
	if !after.Notify.Grok.Channels.Wechat.Enabled {
		t.Fatal("Grok wechat should stay enabled after channel-menu re-init")
	}

	var stdout bytes.Buffer
	if err := printCurrentNotifyConfig(Streams{Stdout: &stdout}); err != nil {
		t.Fatalf("printCurrentNotifyConfig: %v", err)
	}
	out := stdout.String()
	// Only Grok row should contain a WeChat ✅. Other agent rows should be all ❌.
	if !strings.Contains(out, "| Grok         |  ❌  |  ❌  |  ✅  |") {
		t.Fatalf("expected Grok wechat-only row, got:\n%s", out)
	}
	for _, agent := range []string{"Claude Code", "Codex", "ZCode"} {
		// Each of these should be all disabled in the channel table.
		// Claude Code is padded to 12 chars: "Claude Code "
		rowPrefix := "| " + agent
		for _, line := range strings.Split(out, "\n") {
			if !strings.HasPrefix(strings.TrimRight(line, "\r"), rowPrefix) && !strings.Contains(line, "| "+padAgent(agent)) {
				continue
			}
			if strings.Contains(line, agent) && strings.Contains(line, "✅") {
				t.Fatalf("%s row should have no ✅ after Grok-only wechat setup:\n%s\nfull:\n%s", agent, line, out)
			}
		}
	}
}

func padAgent(name string) string {
	if len(name) >= 12 {
		return name
	}
	return name + strings.Repeat(" ", 12-len(name))
}

func mustDefaultPath(t *testing.T) string {
	t.Helper()
	p, err := config.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	return p
}
