//go:build windows

package winfocus

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unicode"
	"unsafe"

	"github.com/hellolib/toast"
)

var (
	winUser32                  = syscall.NewLazyDLL("user32.dll")
	procGetForegroundWindow    = winUser32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcess = winUser32.NewProc("GetWindowThreadProcessId")
	procGetWindowTextW         = winUser32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW   = winUser32.NewProc("GetWindowTextLengthW")
)

// Capture 抓取当前前台窗口，作为「启动本 agent 的那个终端窗口」的快照。
//
// 在 SessionStart 时刻调用（你刚在某个 WT 窗口里回车起 agent，那扇窗必然是前台）。
// 校验前台窗的属主进程在本 hook 进程的祖先链内：焦点此刻若已切到别的应用，则其属主
// 不在祖先里 → 返回 error，调用方据此放弃写缓存（保留上次正确值，防污染）。这样即便
// WT 是「一个进程管多个窗口」，也能锁定正确那扇——因为拿的是真实前台窗的 HWND，绕开了
// 进程树对同 PID 多窗的天然歧义。
func Capture() (string, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return "", errors.New("winfocus: no foreground window")
	}
	var pid uint32
	procGetWindowThreadProcess.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 || !pidInAncestry(pid) {
		return "", errors.New("winfocus: foreground window not in process ancestry")
	}
	title := windowTitle(hwnd)
	if title == "" {
		return "", errors.New("winfocus: foreground window has no title")
	}
	return Encode(hwnd, title), nil
}

// IsUsableAndMatches 供发送时复核缓存句柄：仍可用（存在+可见+无 owner+有标题）且当前
// 标题与缓存兼容。Windows Terminal / Claude Code 会把 spinner 写进标题（如
// "⠂ Claude Code"），完全相等会误拒缓存，导致退回不精确的进程树兜底；因此这里会
// 归一化标题（去前导符号、忽略大小写）并允许 "claude" 与 "Claude Code" 这类包含关系。
// 明显不相关的标题（如缓存 claude、当前 Windows PowerShell）仍会拒用，防 M2——缓存窗口
// 已关闭、HWND 值被 Windows 回收复用给另一扇窗。
func IsUsableAndMatches(hwnd uintptr, expectedTitle string) bool {
	if !toast.IsUsableWindow(hwnd) {
		return false
	}
	return titlesCompatible(expectedTitle, windowTitle(hwnd))
}

// pidInAncestry 判断 target 是否在本进程向上的祖先链内。复用 toast 已导出的进程树遍历
// （EnumerateAncestorWindows 顺带枚举窗口，这里只取其 PID），避免在 agent-notify 侧重复
// 实现 CreateToolhelp32Snapshot 进程快照。SessionStart 每会话一次，开销可忽略。
func pidInAncestry(target uint32) bool {
	for _, anc := range toast.EnumerateAncestorWindows(uint32(os.Getpid())) {
		if anc.PID == target {
			return true
		}
	}
	return false
}

// Probe 返回一段人类可读的捕获诊断，供 doctor / --focus-probe 免点击验证：打印当前前台窗
// 的 HWND、属主 PID、是否通过祖先校验、标题，以及 SessionStart 将写入缓存的 JSON。
// 在某个 WT 窗口里跑，就能核对「抓到的正是这扇窗」。
func Probe() string {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return "[capture] 无前台窗口\n"
	}
	var pid uint32
	procGetWindowThreadProcess.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	title := windowTitle(hwnd)
	inAnc := pidInAncestry(pid)

	var b strings.Builder
	fmt.Fprintf(&b, "[capture] 前台窗 hwnd=0x%x pid=%d 祖先校验=%t 标题=%q\n", hwnd, pid, inAnc, title)
	switch {
	case !inAnc:
		b.WriteString("[capture] → 前台窗不在本进程祖先链，SessionStart 会拒绝缓存（保留上次正确值，防污染）\n")
	case title == "":
		b.WriteString("[capture] → 前台窗无标题，SessionStart 会拒绝缓存\n")
	default:
		fmt.Fprintf(&b, "[capture] → 将缓存: %s\n", Encode(hwnd, title))
	}
	return b.String()
}

func titlesCompatible(expected, current string) bool {
	exp := normalizeTitle(expected)
	cur := normalizeTitle(current)
	if exp == "" || cur == "" {
		return false
	}
	return exp == cur || strings.Contains(cur, exp) || strings.Contains(exp, cur)
}

func normalizeTitle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimLeftFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	return strings.Join(strings.Fields(s), " ")
}

func windowTitle(hwnd uintptr) string {
	n, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}
