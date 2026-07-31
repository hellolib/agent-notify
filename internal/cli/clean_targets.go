package cli

import (
	"slices"

	"github.com/hellolib/agent-notify/internal/agentintegrations"
	"github.com/hellolib/agent-notify/internal/config"
)

// hookCleanupTarget 是 clean 要清理的一个 agent 及其全部已安装位置。
type hookCleanupTarget struct {
	integration agentintegrations.Integration
	paths       []string
}

// hookCleanupTargets 决定 clean 该去哪些文件里摘 hook。
//
// 优先用 config.yaml 里记录的绝对路径:clean 硬编码 "user" scope 时,
// project scope 装到 <项目>/.claude/settings.json 的 hook 永远清不掉,
// 而且 SettingsPath("project") 返回的是相对路径,在别的目录执行 clean
// 根本指不到原来那个项目。
//
// recorded=false(配置读不出来)或某个 agent 没有记录(升级前装的)时回退到
// 扫 user + 当前目录的 project。Uninstall 对不存在的文件是 no-op,
// 所以多扫一个位置没有代价。
func hookCleanupTargets(cfg config.Config, recorded bool) []hookCleanupTarget {
	specs := []struct {
		integration agentintegrations.Integration
		agent       config.AgentTargetConfig
	}{
		{agentintegrations.NewClaudeIntegration(), cfg.Agent.ClaudeCode},
		{agentintegrations.NewCodexIntegration(), cfg.Agent.Codex},
		{agentintegrations.NewZcodeIntegration(), cfg.Agent.ZCode},
		{agentintegrations.NewGrokIntegration(), cfg.Agent.Grok},
		{agentintegrations.NewDroidIntegration(), cfg.Agent.Droid},
	}

	targets := make([]hookCleanupTarget, 0, len(specs))
	for _, spec := range specs {
		var paths []string
		if recorded {
			paths = append(paths, spec.agent.InstalledPaths...)
		}
		for _, scope := range []string{"user", "project"} {
			p, err := spec.integration.SettingsPath(scope)
			if err != nil {
				continue
			}
			if !slices.Contains(paths, p) {
				paths = append(paths, p)
			}
		}
		targets = append(targets, hookCleanupTarget{integration: spec.integration, paths: paths})
	}
	return targets
}
