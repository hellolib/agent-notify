//go:build windows

package doctor

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hellolib/toast"
)

// RunFocusProbe 在当前终端跑一遍 send 解析并打印诊断，再拼上 helper 日志尾部。
func RunFocusProbe(out io.Writer) error {
	_, diag, err := toast.PrepareFocusActivationVerbose(os.Getppid(), "")
	if err != nil {
		fmt.Fprintf(out, "focus probe: %v\n", err)
	} else {
		fmt.Fprint(out, diag.String())
	}
	if home, e := os.UserHomeDir(); e == nil {
		logPath := filepath.Join(home, ".agent-notify", "focus-helper.log")
		if data, e := os.ReadFile(logPath); e == nil {
			fmt.Fprintf(out, "[click] 最近 helper 日志 (%s):\n%s\n", logPath, tailLines(string(data), 20))
		} else {
			fmt.Fprintf(out, "[click] 暂无 helper 日志（%s 不存在——先开 focus_debug 点一次通知）\n", logPath)
		}
	}
	return nil
}

func tailLines(s string, n int) string {
	lines := splitLines(s)
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	return out
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
