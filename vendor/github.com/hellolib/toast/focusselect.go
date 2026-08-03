package toast

import (
	"fmt"
	"strings"
)

// WindowInfo 是一个顶层窗口的朴素快照（除数值 HWND 外不含任何 Win32 句柄）。
// 中立类型，供 send 侧、helper、探针共用与单测。
type WindowInfo struct {
	HWND       uintptr
	OwnerPID   uint32
	Class      string
	Title      string
	Visible    bool
	HasOwner   bool
	Minimized  bool
	ExStyle    uint32
	X, Y, W, H int32
}

// AncestorWindows 归组一个祖先进程及其拥有的顶层窗口。
type AncestorWindows struct {
	PID     uint32
	PPID    uint32
	Exe     string
	Windows []WindowInfo
}

// FocusDiag 记录一次 send（或 click 重走）的完整解析，用于可观测。
type FocusDiag struct {
	StartPID     uint32
	Chain        []AncestorWindows
	SelectedHWND uintptr
	SelectedPID  uint32
	Reason       string
}

// SelectHostWindow 从祖先链中挑选宿主终端窗口。
// P0 语义（行为保持）：按 chain 顺序取第一个"可见 + 无 owner + 有标题"的窗口。
// 返回选中的 HWND（无则 0）、其所属 PID、以及人类可读的原因。
func SelectHostWindow(chain []AncestorWindows) (hwnd uintptr, pid uint32, reason string) {
	for _, anc := range chain {
		for _, w := range anc.Windows {
			if w.Visible && !w.HasOwner && w.Title != "" {
				return w.HWND, anc.PID,
					fmt.Sprintf("first usable window of ancestor pid=%d (%s)", anc.PID, anc.Exe)
			}
		}
	}
	return 0, 0, "no ancestor owned a usable (visible, unowned, titled) top-level window"
}

func (d FocusDiag) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "[send] start pid=%d\n", d.StartPID)
	for _, anc := range d.Chain {
		fmt.Fprintf(&b, "  pid %d %s (ppid=%d)", anc.PID, anc.Exe, anc.PPID)
		if len(anc.Windows) == 0 {
			b.WriteString("  (无可用顶层窗口)\n")
			continue
		}
		b.WriteString("\n")
		for _, w := range anc.Windows {
			mark := " "
			if w.HWND == d.SelectedHWND && d.SelectedHWND != 0 {
				mark = "✓"
			}
			fmt.Fprintf(&b, "    %s HWND 0x%08x class=%s title=%q visible=%t owner=%t min=%t rect=(%d,%d,%d,%d)\n",
				mark, w.HWND, w.Class, w.Title, w.Visible, w.HasOwner, w.Minimized, w.X, w.Y, w.W, w.H)
		}
	}
	fmt.Fprintf(&b, "  selected: hwnd=0x%x pid=%d reason=%s\n", d.SelectedHWND, d.SelectedPID, d.Reason)
	return b.String()
}
