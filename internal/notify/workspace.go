package notify

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveWorkspace returns a displayable workspace path.
//
// Hook stdin payloads on Windows sometimes deliver paths whose non-ASCII
// segments were replaced with '?' (classic ANSI round-trip loss). When the
// payload path is empty or corrupted, fall back to agent-provided env vars
// and the process working directory (Go uses wide Win32 APIs, so Getwd is
// reliable for CJK paths).
func ResolveWorkspace(fromPayload string) string {
	if isUsableWorkspace(fromPayload) {
		return filepath.Clean(fromPayload)
	}
	for _, key := range []string{
		"GROK_WORKSPACE_ROOT",
		"CLAUDE_PROJECT_DIR",
		"PWD",
	} {
		if v := os.Getenv(key); isUsableWorkspace(v) {
			return filepath.Clean(v)
		}
	}
	if wd, err := os.Getwd(); err == nil && isUsableWorkspace(wd) {
		return filepath.Clean(wd)
	}
	// Last resort: keep the original payload even if imperfect.
	return strings.TrimSpace(fromPayload)
}

func isUsableWorkspace(p string) bool {
	p = strings.TrimSpace(p)
	if p == "" {
		return false
	}
	// '?' is not a legal path character on Windows; its presence almost always
	// means the original CJK characters were lost during an ANSI conversion.
	if strings.Contains(p, "?") {
		return false
	}
	return true
}

// shortenWorkspace shortens a long path to the last two segments for toast display.
// Example: E:\学习ai编程项目\agent-notify → 学习ai编程项目/agent-notify
func shortenWorkspace(ws string) string {
	// 同时归一化 '\' 与 '/'：filepath.ToSlash 只处理本机分隔符，跨平台时
	// （如在 Linux 上收到 Windows 的 E:\a\b 路径）不足，故再显式替换反斜杠。
	normalized := strings.ReplaceAll(filepath.ToSlash(ws), `\`, "/")
	parts := strings.Split(normalized, "/")
	var segs []string
	for _, p := range parts {
		if p != "" {
			segs = append(segs, p)
		}
	}
	if len(segs) <= 2 {
		// Prefer forward slashes for consistent multi-platform toast text.
		return normalized
	}
	return strings.Join(segs[len(segs)-2:], "/")
}
