package setup

import (
	"context"
	"testing"

	"github.com/hellolib/agent-notify/internal/config"
)

func TestService_CodexBarkChannel(t *testing.T) {
	loader := &mockConfigLoader{
		defaultPath: "/tmp/injected-config.yaml",
		loadedCfg:   config.Default(),
	}
	svc := NewService(
		WithClaudeIntegration(&mockIntegration{name: "Claude Code", detectInstalled: false}),
		WithCodexIntegration(&mockIntegration{name: "Codex", detectInstalled: true, settingsPath: "/tmp/.codex/config.toml"}),
		WithZcodeIntegration(&mockIntegration{name: "ZCode", detectInstalled: false}),
		WithGrokIntegration(&mockIntegration{name: "Grok", detectInstalled: false}),
		WithConfigLoader(loader),
	)
	wantURL := "https://api.day.app/testkey/replace-me"
	prompter := &mockPrompter{
		selectResult: "codex",
		multiResults: [][]string{
			{"bark"},
			{"run_completed"},
		},
		inputResult: wantURL,
	}

	_, err := svc.Run(context.Background(), prompter, &mockOutputWriter{}, "", "/tmp/agent-notify")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if !promptOptionsContain(prompter.multiOptions[0], "bark") {
		t.Fatal("channel options should include bark")
	}
	got := loader.savedCfg.Notify.Codex.Channels.Bark
	if !got.Enabled {
		t.Fatal("Codex bark should be enabled")
	}
	if got.WebhookURL != wantURL {
		t.Fatalf("Codex bark URL = %q, want %q", got.WebhookURL, wantURL)
	}
}

func TestService_DisablesExistingBarkWhenDeselected(t *testing.T) {
	cfg := config.Default()
	cfg.Notify.ClaudeCode.Channels.Bark.Enabled = true
	cfg.Notify.ClaudeCode.Channels.Bark.WebhookURL = "https://api.day.app/testkey"
	loader := &mockConfigLoader{
		defaultPath: "/tmp/injected-config.yaml",
		loadedCfg:   cfg,
	}
	svc := NewService(
		WithClaudeIntegration(&mockIntegration{name: "Claude Code", detectInstalled: true, settingsPath: "/tmp/.claude/settings.json"}),
		WithCodexIntegration(&mockIntegration{name: "Codex", detectInstalled: false}),
		WithZcodeIntegration(&mockIntegration{name: "ZCode", detectInstalled: false}),
		WithGrokIntegration(&mockIntegration{name: "Grok", detectInstalled: false}),
		WithConfigLoader(loader),
	)
	prompter := &mockPrompter{
		selectResult: "claude",
		multiResults: [][]string{
			{"system"},
			{"permission_required"},
		},
	}

	_, err := svc.Run(context.Background(), prompter, &mockOutputWriter{}, "", "/tmp/agent-notify")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if loader.savedCfg.Notify.ClaudeCode.Channels.Bark.Enabled {
		t.Fatal("ClaudeCode bark should be disabled after deselection")
	}
}

func promptOptionsContain(options []PromptOption, want string) bool {
	for _, option := range options {
		if option.Value == want {
			return true
		}
	}
	return false
}
