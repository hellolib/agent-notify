package cli

import (
	"bytes"
	"testing"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/testutil"
)

func TestRunInitBarkEnablesOnlyConfiguredAgents(t *testing.T) {
	testutil.IsolateHome(t)

	path, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	seed := config.Default()
	seed.Agent.Codex.Enabled = true
	seed.Agent.Grok.Enabled = true
	if err := config.Save(path, seed); err != nil {
		t.Fatalf("Save() seed error = %v", err)
	}

	wantURL := "https://api.day.app/testkey/replace-me"
	if err := runInitBark(Streams{Stdout: &bytes.Buffer{}}, &fakePrompter{inputs: []string{wantURL}}); err != nil {
		t.Fatalf("runInitBark() error = %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Notify.ClaudeCode.Channels.Bark.Enabled {
		t.Fatal("ClaudeCode bark should stay disabled when Claude is not enabled")
	}
	if !cfg.Notify.Codex.Channels.Bark.Enabled {
		t.Fatal("Codex bark should be enabled")
	}
	if !cfg.Notify.Grok.Channels.Bark.Enabled {
		t.Fatal("Grok bark should be enabled")
	}
	if cfg.Notify.Codex.Channels.Bark.WebhookURL != wantURL {
		t.Fatalf("Codex bark URL = %q, want %q", cfg.Notify.Codex.Channels.Bark.WebhookURL, wantURL)
	}
	if cfg.Notify.Grok.Channels.Bark.WebhookURL != wantURL {
		t.Fatalf("Grok bark URL = %q, want %q", cfg.Notify.Grok.Channels.Bark.WebhookURL, wantURL)
	}
}

func TestRunInitBarkPreservesExistingWechat(t *testing.T) {
	testutil.IsolateHome(t)

	path, err := config.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath() error = %v", err)
	}
	seed := config.Default()
	seed.Agent.ClaudeCode.Enabled = true
	seed.Agent.Grok.Enabled = true
	wechatURL := "https://push.example.com/api/notify/wechat-token"
	seed.Notify.ClaudeCode.Channels.Wechat.Enabled = true
	seed.Notify.ClaudeCode.Channels.Wechat.WebhookURL = wechatURL
	seed.Notify.Grok.Channels.Wechat.Enabled = true
	seed.Notify.Grok.Channels.Wechat.WebhookURL = wechatURL
	if err := config.Save(path, seed); err != nil {
		t.Fatalf("Save() seed error = %v", err)
	}

	barkURL := "https://api.day.app/device-key"
	if err := runInitBark(Streams{Stdout: &bytes.Buffer{}}, &fakePrompter{inputs: []string{barkURL}}); err != nil {
		t.Fatalf("runInitBark() error = %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.Notify.ClaudeCode.Channels.Wechat.Enabled {
		t.Fatal("ClaudeCode wechat should remain enabled after bark init")
	}
	if cfg.Notify.ClaudeCode.Channels.Wechat.WebhookURL != wechatURL {
		t.Fatalf("ClaudeCode wechat URL = %q, want %q", cfg.Notify.ClaudeCode.Channels.Wechat.WebhookURL, wechatURL)
	}
	if !cfg.Notify.Grok.Channels.Wechat.Enabled || cfg.Notify.Grok.Channels.Wechat.WebhookURL != wechatURL {
		t.Fatal("Grok wechat should remain enabled with original URL after bark init")
	}
	if !cfg.Notify.ClaudeCode.Channels.Bark.Enabled || cfg.Notify.ClaudeCode.Channels.Bark.WebhookURL != barkURL {
		t.Fatal("ClaudeCode bark should be enabled with new URL")
	}
}
