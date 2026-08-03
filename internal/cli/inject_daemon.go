package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/hellolib/agent-notify/internal/codexhooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/state"
	"github.com/spf13/cobra"
)

// newInjectDaemonCmd 创建 inject-daemon 子命令。
// 由 hook 进程启动为后台子进程（不 setsid），继承 Codex 的控制终端。
// 常驻轮询 inject_queue 目录，发现新文件时通过 TIOCSTI 注入文本到终端。
func newInjectDaemonCmd(ctx context.Context) *cobra.Command {
	var timeoutSec int
	var sessionID string
	var ttyPath string

	cmd := &cobra.Command{
		Use:    "inject-daemon",
		Short:  "Internal: poll inject_queue and inject text via TIOCSTI",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			statePath, err := config.StatePath()
			if err != nil {
				return err
			}
			return runInjectDaemon(ctx, statePath, sessionID, ttyPath, timeoutSec)
		},
	}
	cmd.Flags().StringVar(&sessionID, "session-id", "", "codex session ID for queue isolation")
	cmd.Flags().StringVar(&ttyPath, "tty", "", "TTY device path (e.g. /dev/pts/4)")
	cmd.Flags().IntVar(&timeoutSec, "timeout", config.DefaultTimeoutSec, "max wait seconds")
	return cmd
}

// runInjectDaemon 常驻轮询 inject_queue/<session_id>/ 目录，发现新文件时注入文本。
// 每个 codex session 有独立的队列子目录，避免跨 session 消息串扰。
func runInjectDaemon(ctx context.Context, statePath, sessionID, ttyPath string, timeoutSec int) error {
	logPath, _ := config.LogPath()

	// 去重检查：通过 pid 文件防止同一 session 启动多个 daemon。
	// pid 文件名统一为 inject-daemon-<key>.pid，key 为 sessionID 或 "serve"。
	key := sessionID
	if key == "" {
		key = "serve"
	}
	pidFile := filepath.Join(state.BaseDir(statePath), fmt.Sprintf("inject-daemon-%s.pid", key))
	if data, err := os.ReadFile(pidFile); err == nil {
		var oldPid int
		fmt.Sscanf(string(data), "%d", &oldPid)
		if oldPid > 0 {
			if err := syscall.Kill(oldPid, 0); err == nil {
				_ = codexhooks.AppendLogSimple(logPath, fmt.Sprintf("inject-daemon: session=%s 已有实例运行 (pid=%d)，退出", sessionID, oldPid))
				return nil
			}
		}
	}
	_ = os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0o600)
	defer os.Remove(pidFile)

	// session 专属队列子目录（无 session 时用全局目录）
	queueDir := filepath.Join(state.BaseDir(statePath), "inject_queue")
	if sessionID != "" {
		queueDir = filepath.Join(queueDir, sessionID)
	} else {
		queueDir = filepath.Join(queueDir, "global")
	}
	_ = os.MkdirAll(queueDir, 0o700)

	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	ticker := time.NewTicker(config.DaemonPollInterval)
	defer ticker.Stop()

	_ = codexhooks.AppendLogSimple(logPath, "inject-daemon: 启动，监控 "+queueDir)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				_ = codexhooks.AppendLogSimple(logPath, "inject-daemon: 超时退出")
				return nil
			}
			// 读取队列目录
			entries, err := os.ReadDir(queueDir)
			if err != nil || len(entries) == 0 {
				continue
			}
			// 按文件名排序后处理（文件名是纳秒时间戳，确保有序）
			var files []string
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				files = append(files, entry.Name())
			}
			sort.Strings(files)

			// 处理每个队列文件
			for _, name := range files {
				filePath := filepath.Join(queueDir, name)
				data, err := os.ReadFile(filePath)
				if err != nil {
					continue
				}
				// 删除队列文件（先删再注入，避免重复处理）
				_ = os.Remove(filePath)

				text := string(data)
				if text == "" {
					continue
				}

				// 逐字节注入（UTF-8 多字节字符需要按字节注入）
				// 优先用 ttyPath（具体 pts 路径），回退到 /dev/tty
				injectByte := func(b byte) error {
					if ttyPath != "" {
						return codexhooks.InjectKeystroke(ttyPath, b)
					}
					return codexhooks.InjectKeystrokeToTTY(b)
				}
				for _, b := range []byte(text) {
					if err := injectByte(b); err != nil {
						_ = codexhooks.AppendLogSimple(logPath, fmt.Sprintf("inject-daemon: 注入字节 0x%02x 失败: %v", b, err))
						break
					}
					time.Sleep(config.InjectByteDelay)
				}
				// 注入回车
				time.Sleep(config.InjectEnterDelay)
				_ = injectByte('\r')

				_ = codexhooks.AppendLogSimple(logPath, fmt.Sprintf("inject-daemon: 注入文本 %q", text))
			}
		}
	}
}
