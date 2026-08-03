//go:build windows

package toast

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

var (
	focusKernel32              = syscall.NewLazyDLL("kernel32.dll")
	focusUser32                = syscall.NewLazyDLL("user32.dll")
	focusProcCreateToolhelp32  = focusKernel32.NewProc("CreateToolhelp32Snapshot")
	focusProcProcess32First    = focusKernel32.NewProc("Process32FirstW")
	focusProcProcess32Next     = focusKernel32.NewProc("Process32NextW")
	focusProcCloseHandle       = focusKernel32.NewProc("CloseHandle")
	focusProcEnumWindows       = focusUser32.NewProc("EnumWindows")
	focusProcGetWindow         = focusUser32.NewProc("GetWindow")
	focusProcGetWindowTextLen  = focusUser32.NewProc("GetWindowTextLengthW")
	focusProcGetWindowThreadPr = focusUser32.NewProc("GetWindowThreadProcessId")
	focusProcIsWindowVisible   = focusUser32.NewProc("IsWindowVisible")
	focusProcGetClassNameW     = focusUser32.NewProc("GetClassNameW")
	focusProcGetWindowTextW    = focusUser32.NewProc("GetWindowTextW")
	focusProcGetWindowRect     = focusUser32.NewProc("GetWindowRect")
	focusProcGetWindowLongW    = focusUser32.NewProc("GetWindowLongW")
	focusProcIsIconic          = focusUser32.NewProc("IsIconic")
	focusProcIsWindow          = focusUser32.NewProc("IsWindow")
)

const (
	focusTH32CSSnapProcess = 0x00000002
	focusMaxPath           = 260
	focusGWOwner           = 4
	// focusGWLExStyle is GWL_EXSTYLE (-20) expressed as a uintptr (two's complement).
	focusGWLExStyle = ^uintptr(19)
)

type focusProcessEntry32 struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [focusMaxPath]uint16
}

// FindFocusHelper locates the helper executable used for toast click-to-focus.
// Explicit candidates are checked first, followed by conventional helper names
// next to the current executable.
func FindFocusHelper(candidates ...string) (string, error) {
	for _, candidate := range append(nonEmpty(candidates), defaultFocusHelperCandidates()...) {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", errors.New("toast focus helper not found")
}

// RegisterFocusProtocol registers protocol as a user-local URL protocol that
// launches helperPath with the clicked toast URI.
func RegisterFocusProtocol(helperPath string, protocols ...string) error {
	if helperPath == "" {
		return errors.New("focus helper path is empty")
	}

	protocol := DefaultFocusProtocol
	if len(protocols) > 0 && protocols[0] != "" {
		protocol = protocols[0]
	}

	command := fmt.Sprintf(`"%s" "%%1"`, helperPath)
	commands := [][]string{
		{"add", `HKCU\Software\Classes\` + protocol, "/ve", "/d", "URL:toast focus", "/f"},
		{"add", `HKCU\Software\Classes\` + protocol, "/v", "URL Protocol", "/d", "", "/f"},
		{"add", `HKCU\Software\Classes\` + protocol + `\shell\open\command`, "/ve", "/d", command, "/f"},
	}

	for _, args := range commands {
		cmd := exec.Command("reg", args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("reg %s: %w: %s", args[1], err, string(out))
		}
	}
	return nil
}

// PrepareFocusActivationVerbose 找 helper、注册协议、解析宿主窗口，并返回完整诊断。
// logPath 非空时嵌入激活 URI（helper 点击时据此写日志）。
func PrepareFocusActivationVerbose(pid int, logPath string, helperCandidates ...string) (FocusActivation, FocusDiag, error) {
	helper, err := FindFocusHelper(helperCandidates...)
	if err != nil {
		return FocusActivation{}, FocusDiag{StartPID: uint32(pid)}, err
	}
	if err := RegisterFocusProtocol(helper); err != nil {
		return FocusActivation{}, FocusDiag{StartPID: uint32(pid)}, err
	}
	chain := EnumerateAncestorWindows(uint32(pid))
	hwnd, selPID, reason := SelectHostWindow(chain)
	diag := FocusDiag{
		StartPID:     uint32(pid),
		Chain:        chain,
		SelectedHWND: hwnd,
		SelectedPID:  selPID,
		Reason:       reason,
	}
	return FocusActivation{
		Protocol:  DefaultFocusProtocol,
		Helper:    helper,
		Arguments: buildFocusArguments(pid, hwnd, logPath, DefaultFocusProtocol),
		Window:    hwnd,
	}, diag, nil
}

// PrepareFocusActivation 保留原签名，委托 Verbose（无 logPath、丢弃诊断）。
func PrepareFocusActivation(pid int, helperCandidates ...string) (FocusActivation, error) {
	act, _, err := PrepareFocusActivationVerbose(pid, "", helperCandidates...)
	return act, err
}

type focusRect struct{ Left, Top, Right, Bottom int32 }

// focusProcessTable 单次快照，返回 pid -> (ppid, exe)。
func focusProcessTable() map[uint32]struct {
	PPID uint32
	Exe  string
} {
	table := map[uint32]struct {
		PPID uint32
		Exe  string
	}{}
	snap, _, _ := focusProcCreateToolhelp32.Call(focusTH32CSSnapProcess, 0)
	if snap == 0 {
		return table
	}
	defer focusProcCloseHandle.Call(snap)

	entry := focusProcessEntry32{Size: uint32(unsafe.Sizeof(focusProcessEntry32{}))}
	ok, _, _ := focusProcProcess32First.Call(snap, uintptr(unsafe.Pointer(&entry)))
	for ok != 0 {
		exe := syscall.UTF16ToString(entry.ExeFile[:])
		table[entry.ProcessID] = struct {
			PPID uint32
			Exe  string
		}{PPID: entry.ParentProcessID, Exe: exe}
		ok, _, _ = focusProcProcess32Next.Call(snap, uintptr(unsafe.Pointer(&entry)))
	}
	return table
}

func focusWindowText(hwnd uintptr) string {
	n, _, _ := focusProcGetWindowTextLen.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	focusProcGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}

func focusWindowClass(hwnd uintptr) string {
	buf := make([]uint16, 256)
	focusProcGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}

func focusWindowInfo(hwnd uintptr, ownerPID uint32) WindowInfo {
	visible, _, _ := focusProcIsWindowVisible.Call(hwnd)
	owner, _, _ := focusProcGetWindow.Call(hwnd, focusGWOwner)
	iconic, _, _ := focusProcIsIconic.Call(hwnd)
	exStyle, _, _ := focusProcGetWindowLongW.Call(hwnd, focusGWLExStyle)
	var r focusRect
	focusProcGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return WindowInfo{
		HWND:      hwnd,
		OwnerPID:  ownerPID,
		Class:     focusWindowClass(hwnd),
		Title:     focusWindowText(hwnd),
		Visible:   visible != 0,
		HasOwner:  owner != 0,
		Minimized: iconic != 0,
		ExStyle:   uint32(exStyle),
		X:         r.Left,
		Y:         r.Top,
		W:         r.Right - r.Left,
		H:         r.Bottom - r.Top,
	}
}

func windowsForPID(pid uint32) []WindowInfo {
	var out []WindowInfo
	focusProcEnumWindows.Call(
		syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
			var windowPID uint32
			focusProcGetWindowThreadPr.Call(hwnd, uintptr(unsafe.Pointer(&windowPID)))
			if windowPID == pid {
				out = append(out, focusWindowInfo(hwnd, pid))
			}
			return 1
		}),
		0,
	)
	return out
}

// EnumerateAncestorWindows 从 start 沿进程树向上，逐祖先归组其全部顶层窗口。
func EnumerateAncestorWindows(start uint32) []AncestorWindows {
	table := focusProcessTable()
	var chain []AncestorWindows
	seen := map[uint32]bool{}
	for current := start; current != 0 && !seen[current]; {
		seen[current] = true
		info := table[current]
		chain = append(chain, AncestorWindows{
			PID:     current,
			PPID:    info.PPID,
			Exe:     info.Exe,
			Windows: windowsForPID(current),
		})
		parent := info.PPID
		if parent == 0 || parent == current {
			break
		}
		current = parent
	}
	return chain
}

// IsUsableWindow 校验单个 HWND（供 helper 直连路径）：存在 + 可见 + 无 owner + 有标题。
func IsUsableWindow(hwnd uintptr) bool {
	exists, _, _ := focusProcIsWindow.Call(hwnd)
	if exists == 0 {
		return false
	}
	visible, _, _ := focusProcIsWindowVisible.Call(hwnd)
	if visible == 0 {
		return false
	}
	owner, _, _ := focusProcGetWindow.Call(hwnd, focusGWOwner)
	if owner != 0 {
		return false
	}
	titleLen, _, _ := focusProcGetWindowTextLen.Call(hwnd)
	return titleLen > 0
}

func defaultFocusHelperCandidates() []string {
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	dir := filepath.Dir(exe)
	base := filepath.Base(exe)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)

	var candidates []string
	if suffix := strings.TrimPrefix(stem, "toast-focus"); suffix != stem && suffix != "" {
		candidates = append(candidates, filepath.Join(dir, "toast-focus-helper"+suffix+ext))
	}
	candidates = append(candidates,
		filepath.Join(dir, stem+"-focus-helper"+ext),
		filepath.Join(dir, stem+"-helper"+ext),
		filepath.Join(dir, defaultFocusHelperName),
	)
	if runtime.GOARCH == "arm64" {
		candidates = append([]string{filepath.Join(dir, "toast-focus-helper-arm64.exe")}, candidates...)
	}
	return candidates
}

func nonEmpty(items []string) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}
