package cli

import (
	"bytes"
	"testing"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/testutil"
)

func TestRunInitWechatEnablesOnlyConfiguredAgents(t *testing.T) {
	testutil.IsolateHome(t)

	path, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	// Only Grok has been set up (mirrors: Agent 通知配置 → Grok only).
	seed := config.Default()
	seed.Agent.Grok.Enabled = true
	if err := config.Save(path, seed); err != nil {
		t.Fatalf("Save() seed error = %v", err)
	}

	wantURL := "https://push.example.com/api/notify/abc"
	if err := runInitWechat(Streams{Stdout: &bytes.Buffer{}}, &fakePrompter{inputs: []string{wantURL}}); err != nil {
		t.Fatalf("runInitWechat() error = %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Enabled only for Grok — other agents must not appear "configured".
	if cfg.Notify.ClaudeCode.Channels.Wechat.Enabled {
		t.Fatal("ClaudeCode wechat should stay disabled when only Grok is configured")
	}
	if cfg.Notify.Codex.Channels.Wechat.Enabled {
		t.Fatal("Codex wechat should stay disabled when only Grok is configured")
	}
	if cfg.Notify.ZCode.Channels.Wechat.Enabled {
		t.Fatal("ZCode wechat should stay disabled when only Grok is configured")
	}
	if !cfg.Notify.Grok.Channels.Wechat.Enabled {
		t.Fatal("Grok wechat should be enabled")
	}
	if cfg.Notify.Grok.Channels.Wechat.WebhookURL != wantURL {
		t.Fatalf("Grok wechat URL = %q, want %q", cfg.Notify.Grok.Channels.Wechat.WebhookURL, wantURL)
	}
	// Credentials still stored on other agents for later setup defaults.
	if cfg.Notify.ClaudeCode.Channels.Wechat.WebhookURL != wantURL {
		t.Fatalf("ClaudeCode wechat URL should be stored as default, got %q", cfg.Notify.ClaudeCode.Channels.Wechat.WebhookURL)
	}
}

func TestRunInitWechatEnablesAllWhenAllAgentsEnabled(t *testing.T) {
	testutil.IsolateHome(t)

	path, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	seed := config.Default()
	seed.Agent.ClaudeCode.Enabled = true
	seed.Agent.Codex.Enabled = true
	seed.Agent.ZCode.Enabled = true
	seed.Agent.Grok.Enabled = true
	if err := config.Save(path, seed); err != nil {
		t.Fatalf("Save() seed error = %v", err)
	}

	wantURL := "https://push.example.com/api/notify/all"
	if err := runInitWechat(Streams{Stdout: &bytes.Buffer{}}, &fakePrompter{inputs: []string{wantURL}}); err != nil {
		t.Fatalf("runInitWechat() error = %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	for _, agent := range []struct {
		name string
		ch   config.WechatChannelConfig
	}{
		{"ClaudeCode", cfg.Notify.ClaudeCode.Channels.Wechat},
		{"Codex", cfg.Notify.Codex.Channels.Wechat},
		{"ZCode", cfg.Notify.ZCode.Channels.Wechat},
		{"Grok", cfg.Notify.Grok.Channels.Wechat},
	} {
		if !agent.ch.Enabled {
			t.Fatalf("%s wechat should be enabled when agent is enabled", agent.name)
		}
		if agent.ch.WebhookURL != wantURL {
			t.Fatalf("%s wechat URL = %q, want %q", agent.name, agent.ch.WebhookURL, wantURL)
		}
	}
}

func TestRunInitWechatPreservesExistingBarkAndDoesNotDisableIt(t *testing.T) {
	testutil.IsolateHome(t)

	path, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	seed := config.Default()
	seed.Agent.ClaudeCode.Enabled = true
	seed.Agent.Grok.Enabled = true
	barkURL := "https://api.day.app/existing-key"
	seed.Notify.ClaudeCode.Channels.Bark.Enabled = true
	seed.Notify.ClaudeCode.Channels.Bark.WebhookURL = barkURL
	seed.Notify.Grok.Channels.Bark.Enabled = true
	seed.Notify.Grok.Channels.Bark.WebhookURL = barkURL
	if err := config.Save(path, seed); err != nil {
		t.Fatalf("Save() seed error = %v", err)
	}

	wechatURL := "https://push.example.com/api/notify/wechat"
	if err := runInitWechat(Streams{Stdout: &bytes.Buffer{}}, &fakePrompter{inputs: []string{wechatURL}}); err != nil {
		t.Fatalf("runInitWechat() error = %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Notify.ClaudeCode.Channels.Wechat.Enabled || cfg.Notify.ClaudeCode.Channels.Wechat.WebhookURL != wechatURL {
		t.Fatal("ClaudeCode wechat should be enabled with new URL")
	}
	// Channel-menu init is additive: must not wipe a previously configured Bark.
	if !cfg.Notify.ClaudeCode.Channels.Bark.Enabled {
		t.Fatal("ClaudeCode bark should remain enabled after wechat init")
	}
	if cfg.Notify.ClaudeCode.Channels.Bark.WebhookURL != barkURL {
		t.Fatalf("ClaudeCode bark URL = %q, want %q", cfg.Notify.ClaudeCode.Channels.Bark.WebhookURL, barkURL)
	}
}

func TestRunInitWechatDoesNotClobberPriorWechatWithEmptyReload(t *testing.T) {
	testutil.IsolateHome(t)

	path, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	seed := config.Default()
	seed.Agent.Grok.Enabled = true
	oldURL := "https://push.example.com/api/notify/old"
	seed.Notify.Grok.Channels.Wechat.Enabled = true
	seed.Notify.Grok.Channels.Wechat.WebhookURL = oldURL
	// Simulate channel-only setup that never wrote events for Grok.
	seed.Notify.Grok.Events = nil
	if err := config.Save(path, seed); err != nil {
		t.Fatalf("Save() seed error = %v", err)
	}

	// Re-init with same default (empty input uses current URL from config).
	if err := runInitWechat(Streams{Stdout: &bytes.Buffer{}}, &fakePrompter{inputs: []string{oldURL}}); err != nil {
		t.Fatalf("runInitWechat() error = %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Notify.Grok.Channels.Wechat.WebhookURL != oldURL {
		t.Fatalf("Grok wechat URL lost: got %q want %q", cfg.Notify.Grok.Channels.Wechat.WebhookURL, oldURL)
	}
	if len(cfg.Notify.Grok.Events) == 0 {
		t.Fatal("Grok events should be backfilled so wechat notifications can dispatch")
	}
}

func TestRunInitWechatClearsEnabledOnUnconfiguredAgents(t *testing.T) {
	testutil.IsolateHome(t)

	path, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	// Polluted state: channel menu used to enable wechat on every agent.
	seed := config.Default()
	seed.Agent.Grok.Enabled = true
	seed.Agent.ClaudeCode.Enabled = false
	url := "https://push.example.com/api/notify/x"
	for _, ch := range []*config.WechatChannelConfig{
		&seed.Notify.ClaudeCode.Channels.Wechat,
		&seed.Notify.Codex.Channels.Wechat,
		&seed.Notify.ZCode.Channels.Wechat,
		&seed.Notify.Grok.Channels.Wechat,
	} {
		ch.Enabled = true
		ch.WebhookURL = url
	}
	if err := config.Save(path, seed); err != nil {
		t.Fatalf("Save() seed error = %v", err)
	}

	if err := runInitWechat(Streams{Stdout: &bytes.Buffer{}}, &fakePrompter{inputs: []string{url}}); err != nil {
		t.Fatalf("runInitWechat() error = %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Notify.ClaudeCode.Channels.Wechat.Enabled {
		t.Fatal("re-init should disable wechat on agents that are not enabled")
	}
	if !cfg.Notify.Grok.Channels.Wechat.Enabled {
		t.Fatal("Grok wechat should remain enabled")
	}
}
