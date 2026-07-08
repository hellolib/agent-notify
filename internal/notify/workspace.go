package notify

import (
	"path/filepath"
	"strings"
)

// shortenWorkspace 将长路径缩短为末尾项目名，避免通知内容因路径过长而难以识别。
// 例如 /Users/foo/workspace/github/hellolib/agent-notify -> hellolib/agent-notify
func shortenWorkspace(ws string) string {
	ws = strings.TrimSpace(ws)
	normalized := strings.ReplaceAll(filepath.ToSlash(ws), "\\", "/")
	parts := strings.Split(normalized, "/")
	var segs []string
	for _, p := range parts {
		if p != "" {
			segs = append(segs, p)
		}
	}
	if len(segs) <= 2 {
		return ws
	}
	return strings.Join(segs[len(segs)-2:], "/")
}
