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

	// Windows 上 os.UserHomeDir() 读 %USERPROFILE%，Unix 读 $HOME；同时设置两者，
	// 让 AgentLogoPath 在三平台 CI 上都解析到 tmpDir。
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

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

	// Windows 上 os.UserHomeDir() 读 %USERPROFILE%，Unix 读 $HOME；同时设置两者，
	// 让 AgentLogoPath 在三平台 CI 上都解析到 tmpDir。
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	got := AgentLogoPath("codex")
	want := filepath.Join(tmpDir, ".agent-notify", "agent-notify.png")
	if got != want {
		t.Fatalf("AgentLogoPath(\"codex\") fallback = %q, want %q", got, want)
	}
}

func TestAgentLogoPathReturnsEmptyWhenNothingFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Windows 上 os.UserHomeDir() 读 %USERPROFILE%，Unix 读 $HOME；同时设置两者，
	// 让 AgentLogoPath 在三平台 CI 上都解析到 tmpDir。
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

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
		{"opencode", "opencode.png"},
		{"unknown", "unknown.png"},
	}
	for _, c := range cases {
		got := agentLogoFilename(c.agent)
		if got != c.wantBasename {
			t.Errorf("agentLogoFilename(%q) = %q, want %q", c.agent, got, c.wantBasename)
		}
	}
}

// supportedAgents 是本项目支持的全部 agent 名，与 appDisplayName 的分支一一对应。
// 这里硬编码而非从 agentintegrations 取：那个包 import 了各 agent 的 hooks 包，
// 而它们又 import 本包，引用过去会形成循环依赖。
var supportedAgents = []string{"claude_code", "codex", "zcode", "grok", "droid", "opencode"}

// 每个支持的 agent 都必须在仓库里备有 logo 文件，否则 release archive 打不进去
// （release.yml 用 assist/logo/agentlogo/*.png 通配打包），通知会静默回退成通用
// 图标——opencode.png 就曾因为漏了 git add 而一直没进过任何 release。
func TestEverySupportedAgentHasLogoAsset(t *testing.T) {
	for _, agent := range supportedAgents {
		path := filepath.Join("..", "..", "assist", "logo", "agentlogo", agentLogoFilename(agent))
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("agent %q has no logo asset at %s: %v", agent, path, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("agent %q logo asset %s is empty", agent, path)
		}
	}
}

// appDisplayName 与 supportedAgents 必须同步：漏了某个 agent 会让通知标题
// 直接显示内部名（如 "opencode 等待输入" 而非 "OpenCode 等待输入"）。
func TestSupportedAgentsAllHaveDisplayName(t *testing.T) {
	for _, agent := range supportedAgents {
		if got := appDisplayName(agent); got == agent {
			t.Errorf("appDisplayName(%q) returned the raw agent name; add a case for it", agent)
		}
	}
}
