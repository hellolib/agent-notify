package cli

import (
	"strings"

	"github.com/hellolib/agent-notify/internal/agentintegrations"
	"github.com/hellolib/agent-notify/internal/opencodehooks"
)

// isHeadlessInvocation 判断本次调用是否由 agent/系统自动拉起（hook 路径），
// 而非用户交互执行。
//
// hook handler 统一以 handle- 前缀命名（见 CLAUDE.md），新增 agent 沿用该前缀
// 即自动被覆盖；linux-notify-wait 由通知点击回调拉起，同样无人值守。
// 无参数表示启动交互式 TUI，属于交互调用。
func isHeadlessInvocation(args []string) bool {
	if len(args) == 0 {
		return false
	}
	name := args[0]
	return strings.HasPrefix(name, "handle-") || name == "linux-notify-wait"
}

// refreshOpenCodePlugin 把过期的 opencode 插件 JS 重写为当前二进制内嵌的版本。
//
// 二进制升级（npx 下载新版本）不会触碰 ~/.agent-notify/opencode-plugin.js，
// 只有用户重跑向导时的 Install 才会写它。于是新版本新增的订阅事件对存量用户
// 永远不生效。而 npx 升级后必然会 exec 二进制（npx/bin/agent-notify.js:147），
// 因此在交互命令入口做这件事，可以同时覆盖 npx 升级、make local 和手动替换
// 二进制三条路径。
//
// 全程 best-effort：任何失败都不能影响用户实际想跑的命令，因此吞掉错误。
// 插件文件不存在时 RefreshPluginIfStale 会跳过，不会让已卸载的集成复活；
// 文件里烘焙的二进制路径原样保留，不会被当前进程的可执行路径顶掉。
func refreshOpenCodePlugin() {
	pluginPath, err := agentintegrations.PluginFilePath()
	if err != nil {
		return
	}
	_, _ = opencodehooks.RefreshPluginIfStale(pluginPath)
}
