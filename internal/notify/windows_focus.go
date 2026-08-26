//go:build windows

package notify

import (
	"fmt"
	"strings"

	"github.com/hellolib/toast"
)

// selectWorkspaceWindow 在祖先进程拥有的窗口中，按工作区目录名唯一匹配标题。
// VS Code 的多个窗口通常共用一个 Code.exe，进程树只能定位到应用，工作区标题才能
// 区分具体窗口。匹配不唯一时返回 0，让调用方退回原有的进程树选择。
func selectWorkspaceWindow(chain []toast.AncestorWindows, workspace string) (hwnd uintptr, pid uint32, reason string) {
	name := workspaceBaseName(workspace)
	if name == "" {
		return 0, 0, "workspace has no usable base name"
	}

	type match struct {
		hwnd uintptr
		pid  uint32
	}
	var matches []match
	needle := strings.ToLower(name)
	for _, ancestor := range chain {
		for _, window := range ancestor.Windows {
			if !window.Visible || window.HasOwner || window.Title == "" {
				continue
			}
			if strings.Contains(strings.ToLower(window.Title), needle) {
				matches = append(matches, match{hwnd: window.HWND, pid: ancestor.PID})
			}
		}
	}

	if len(matches) != 1 {
		return 0, 0, fmt.Sprintf("workspace title %q matched %d usable windows", name, len(matches))
	}
	return matches[0].hwnd, matches[0].pid, fmt.Sprintf("unique window title match for workspace %q", name)
}

// workspaceBaseName 同时接受 Windows 与 POSIX 路径，保证交叉编译测试行为一致。
func workspaceBaseName(workspace string) string {
	path := strings.TrimSpace(strings.ReplaceAll(workspace, `\`, "/"))
	path = strings.TrimRight(path, "/")
	if path == "" {
		return ""
	}
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		path = path[i+1:]
	}
	if path == "." || strings.HasSuffix(path, ":") {
		return ""
	}
	return strings.TrimSpace(path)
}
