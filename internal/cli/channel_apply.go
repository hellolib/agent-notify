package cli

import "github.com/hellolib/agent-notify/internal/config"

// applyChannelToAgents updates a notification channel across agents.
//
// Webhook/topic credentials are written for every agent so a later agent setup
// can reuse them as input defaults. Enabled is set only for agents the user has
// already turned on (Agent.X.Enabled) — this prevents channel-menu init from
// marking Claude/Codex/ZCode as configured when only Grok (or any single agent)
// was set up.
//
// If no agent is enabled yet, credentials are still stored everywhere but the
// channel stays disabled until agent setup selects it.
func applyChannelToAgents(cfg *config.Config, apply func(agentEnabled bool, notify *config.AgentNotifyConfig)) {
	apply(cfg.Agent.ClaudeCode.Enabled, &cfg.Notify.ClaudeCode)
	apply(cfg.Agent.Codex.Enabled, &cfg.Notify.Codex)
	apply(cfg.Agent.ZCode.Enabled, &cfg.Notify.ZCode)
	apply(cfg.Agent.Grok.Enabled, &cfg.Notify.Grok)
}
