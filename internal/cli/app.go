package cli

import (
	"context"
	"io"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/i18n"
)

func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	initLocale()

	// 二进制升级不会重写 opencode 插件 JS，在交互调用时顺带自愈一次。
	// 放在这里而非 cobra 的 PersistentPreRun：无参数启动 TUI 的分支
	// （下方 runMenu）根本不经过 cobra，而那正是 npx 升级后最常见的路径。
	if !isHeadlessInvocation(args) {
		refreshOpenCodePlugin()
	}
	if shouldRefreshCodexNotify(args) {
		refreshCodexNotifyCommand()
	}

	streams := Streams{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}
	if len(args) == 0 {
		return runMenu(ctx, streams)
	}

	cmd := NewRootCmd(ctx, Streams{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	})
	cmd.SetArgs(args)
	return cmd.Execute()
}

// initLocale loads the persisted locale from config and applies it.
// If the config cannot be loaded, the default (zh-CN) is used.
func initLocale() {
	path, err := config.DefaultPath()
	if err != nil {
		return
	}
	cfg, err := config.Load(path)
	if err != nil {
		return
	}
	i18n.Set(cfg.Behavior.Locale)
}
