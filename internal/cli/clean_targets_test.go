package cli

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/hellolib/agent-notify/internal/config"
)

// TestHookCleanupTargetsUsesRecordedPaths 是 issue #39 第 4 项的回归测试。
// 旧实现硬编码 SettingsPath("user"),project scope 装出去的 hook 永远清不掉。
func TestHookCleanupTargetsUsesRecordedPaths(t *testing.T) {
	recorded := filepath.Join(t.TempDir(), "proj-a", ".claude", "settings.json")
	cfg := config.Default()
	cfg.Agent.ClaudeCode.InstalledPaths = []string{recorded}

	targets := hookCleanupTargets(cfg, true)

	claude := targets[0]
	if claude.integration.Name() != "Claude Code" {
		t.Fatalf("targets[0] = %s", claude.integration.Name())
	}
	if !slices.Contains(claude.paths, recorded) {
		t.Fatalf("记录的安装路径未被纳入清理: %v", claude.paths)
	}
	if claude.paths[0] != recorded {
		t.Fatalf("记录的路径应排在兜底扫描之前: %v", claude.paths)
	}
}

func TestHookCleanupTargetsAlwaysIncludesBothScopes(t *testing.T) {
	// 升级前安装的用户没有 installed_paths 记录,必须回退到扫两个 scope
	targets := hookCleanupTargets(config.Default(), true)

	for _, target := range targets {
		userPath, err := target.integration.SettingsPath("user")
		if err != nil {
			t.Fatal(err)
		}
		projectPath, err := target.integration.SettingsPath("project")
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(target.paths, userPath) {
			t.Fatalf("%s: user scope 未纳入: %v", target.integration.Name(), target.paths)
		}
		if !slices.Contains(target.paths, projectPath) {
			t.Fatalf("%s: project scope 未纳入: %v", target.integration.Name(), target.paths)
		}
	}
}

func TestHookCleanupTargetsDoesNotDuplicatePaths(t *testing.T) {
	cfg := config.Default()
	// 记录的路径恰好就是 user scope 的位置(最常见的情况)
	claude := hookCleanupTargets(cfg, true)[0]
	userPath, err := claude.integration.SettingsPath("user")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Agent.ClaudeCode.InstalledPaths = []string{userPath}

	paths := hookCleanupTargets(cfg, true)[0].paths
	seen := map[string]int{}
	for _, p := range paths {
		seen[p]++
	}
	if seen[userPath] != 1 {
		t.Fatalf("user scope 路径出现 %d 次: %v", seen[userPath], paths)
	}
}

func TestHookCleanupTargetsIgnoresRecordsWhenConfigUnreadable(t *testing.T) {
	cfg := config.Default()
	cfg.Agent.ClaudeCode.InstalledPaths = []string{"/should/not/appear/settings.json"}

	paths := hookCleanupTargets(cfg, false)[0].paths

	if slices.Contains(paths, "/should/not/appear/settings.json") {
		t.Fatalf("配置读不出来时不应信任其中的记录: %v", paths)
	}
	if len(paths) != 2 {
		t.Fatalf("应只剩 user + project 两个兜底位置: %v", paths)
	}
}

func TestHookCleanupTargetsCoversAllAgents(t *testing.T) {
	targets := hookCleanupTargets(config.Default(), true)
	var names []string
	for _, target := range targets {
		names = append(names, target.integration.Name())
	}
	want := []string{"Claude Code", "Codex", "ZCode", "Grok", "Droid", "OpenCode"}
	if !slices.Equal(names, want) {
		t.Fatalf("agents = %v, want %v", names, want)
	}
}
