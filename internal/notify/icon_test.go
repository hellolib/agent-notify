package notify

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAgentLogoPathReturnsAgentSpecificLogo(t *testing.T) {
	tmpDir := t.TempDir()
	agentlogoDir := filepath.Join(tmpDir, ".agent-notify", "agentlogo")
	if err := os.MkdirAll(agentlogoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentlogoDir, "claude.png"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	got := AgentLogoPath("claude_code")
	want := filepath.Join(tmpDir, ".agent-notify", "agentlogo", "claude.png")
	if got != want {
		t.Fatalf("AgentLogoPath(\"claude_code\") = %q, want %q", got, want)
	}
}

func TestAgentLogoPathFallsBackToDefault(t *testing.T) {
	tmpDir := t.TempDir()
	agentlogoDir := filepath.Join(tmpDir, ".agent-notify", "agentlogo")
	if err := os.MkdirAll(agentlogoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// 只放 fallback，不放 agent 专属
	if err := os.WriteFile(filepath.Join(tmpDir, ".agent-notify", "agent-notify.png"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	got := AgentLogoPath("codex")
	want := filepath.Join(tmpDir, ".agent-notify", "agent-notify.png")
	if got != want {
		t.Fatalf("AgentLogoPath(\"codex\") fallback = %q, want %q", got, want)
	}
}

func TestAgentLogoPathReturnsEmptyWhenNothingFound(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	got := AgentLogoPath("claude_code")
	if got != "" {
		t.Fatalf("AgentLogoPath(\"claude_code\") = %q, want empty", got)
	}
}

func TestAgentLogoNameMapping(t *testing.T) {
	cases := []struct {
		agent        string
		wantBasename string
	}{
		{"claude_code", "claude.png"},
		{"codex", "openai.png"},
		{"zcode", "zcode.png"},
		{"grok", "grok.png"},
		{"droid", "droid.png"},
		{"unknown", "unknown.png"},
	}
	for _, c := range cases {
		got := agentLogoFilename(c.agent)
		if got != c.wantBasename {
			t.Errorf("agentLogoFilename(%q) = %q, want %q", c.agent, got, c.wantBasename)
		}
	}
}
