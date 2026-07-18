package notify

import (
	"os"
	"path/filepath"
)

// focusHelperLogPath 返回点击聚焦探针日志文件路径；focusDebug 关闭或取不到 home 时返回空串。
func focusHelperLogPath(focusDebug bool) string {
	if !focusDebug {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".agent-notify", "focus-helper.log")
}
