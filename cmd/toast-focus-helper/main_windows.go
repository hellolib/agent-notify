//go:build windows

// toast-focus-helper is launched by the anfocus: protocol. Build it with
// -ldflags="-H windowsgui" so clicking the toast does not flash a console.
package main

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/hellolib/toast"
)

var (
	user32                = syscall.NewLazyDLL("user32.dll")
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	procGetWindowThreadPr = user32.NewProc("GetWindowThreadProcessId")
	procShowWindowAsync   = user32.NewProc("ShowWindowAsync")
	procSetForegroundWnd  = user32.NewProc("SetForegroundWindow")
	procAllowSetFG        = user32.NewProc("AllowSetForegroundWindow")
	procAttachThreadInp   = user32.NewProc("AttachThreadInput")
	procGetForegWnd       = user32.NewProc("GetForegroundWindow")
	procIsIconic          = user32.NewProc("IsIconic")
	procGetCurrentThID    = kernel32.NewProc("GetCurrentThreadId")
)

const (
	swRestore = 9
	asfwAny   = ^uint32(0)
)

func main() {
	if len(os.Args) < 2 {
		os.Exit(1)
	}
	pid, hwnd, logPath, err := toast.ParseFocusActivation(os.Args[1])
	if err != nil {
		os.Exit(1)
	}

	logf := openLog(logPath)
	defer logf.close()
	logf.printf("uri=%q parsed pid=%d hwnd=0x%x", os.Args[1], pid, hwnd)

	if hwnd != 0 && toast.IsUsableWindow(hwnd) {
		focusWindow(hwnd, logf, "direct")
		return
	}

	rehwnd, selPID, reason := toast.SelectHostWindow(toast.EnumerateAncestorWindows(uint32(pid)))
	logf.printf("path=rewalk selected_hwnd=0x%x selected_pid=%d reason=%q", rehwnd, selPID, reason)
	if rehwnd == 0 {
		logf.printf("result=giveup (no window)")
		os.Exit(1)
	}
	focusWindow(rehwnd, logf, "rewalk")
}

func focusWindow(hwnd uintptr, logf *logger, path string) {
	before, _, _ := procGetForegWnd.Call()
	iconic, _, _ := procIsIconic.Call(hwnd)
	if iconic != 0 {
		procShowWindowAsync.Call(hwnd, uintptr(swRestore))
	}
	setForeground(hwnd)
	after, _, _ := procGetForegWnd.Call()
	logf.printf("path=%s minimized=%t SetForeground done fg_before=0x%x fg_after=0x%x", path, iconic != 0, before, after)
}

func setForeground(hwnd uintptr) {
	procAllowSetFG.Call(uintptr(asfwAny))
	foreground, _, _ := procGetForegWnd.Call()
	if foreground != 0 {
		foregroundThread, _, _ := procGetWindowThreadPr.Call(foreground, 0)
		currentThread, _, _ := procGetCurrentThID.Call()
		if foregroundThread != currentThread {
			procAttachThreadInp.Call(foregroundThread, currentThread, 1)
			procSetForegroundWnd.Call(hwnd)
			procAttachThreadInp.Call(foregroundThread, currentThread, 0)
			return
		}
	}
	procSetForegroundWnd.Call(hwnd)
}

type logger struct{ f *os.File }

func openLog(path string) *logger {
	if path == "" {
		return &logger{}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return &logger{}
	}
	return &logger{f: f}
}

func (l *logger) printf(format string, args ...any) {
	if l.f == nil {
		return
	}
	fmt.Fprintf(l.f, "[click %s] %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
}

func (l *logger) close() {
	if l.f != nil {
		_ = l.f.Close()
	}
}
