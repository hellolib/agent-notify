//go:build linux

package codexhooks

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// TIOCSTI ioctl 常量，用于向终端注入按键。
const tiocsti = 0x5412

// FindControllingTTY 返回 Codex 终端的真实 pts 设备路径（如 /dev/pts/3）。
// hook 进程的 stdin 可能被 codex 重定向为管道，因此需要遍历父进程链
// 查找指向 /dev/pts/* 的 fd/0。
func FindControllingTTY() (string, error) {
	// 遍历父进程链，查找 fd/0 指向 /dev/pts/* 的进程
	pid := os.Getpid()
	for i := 0; i < 10; i++ {
		fd0 := fmt.Sprintf("/proc/%d/fd/0", pid)
		if link, err := os.Readlink(fd0); err == nil && link != "" {
			if isPtsDevice(link) {
				return link, nil
			}
		}
		// 也检查 fd/1 和 fd/2
		for _, fd := range []string{"1", "2"} {
			fdPath := fmt.Sprintf("/proc/%d/fd/%s", pid, fd)
			if link, err := os.Readlink(fdPath); err == nil && link != "" {
				if isPtsDevice(link) {
					return link, nil
				}
			}
		}
		// 获取父进程 PID
		ppid, err := readPpid(pid)
		if err != nil || ppid <= 1 {
			break
		}
		pid = ppid
	}

	// 回退：搜索所有进程的 fd/0，找 codex 相关的 pts
	if pts := findCodexPts(); pts != "" {
		return pts, nil
	}

	// 最后回退到 /dev/tty
	return "/dev/tty", nil
}

// isPtsDevice 检查路径是否是 /dev/pts/* 设备。
func isPtsDevice(path string) bool {
	return strings.HasPrefix(path, "/dev/pts/")
}

// readPpid 从 /proc/<pid>/stat 读取父进程 PID。
func readPpid(pid int) (int, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}
	// /proc/<pid>/stat 格式: pid (comm) state ppid ...
	// comm 可能包含空格和括号，需要找到最后一个 ')' 之后的内容
	s := string(data)
	idx := strings.LastIndex(s, ")")
	if idx < 0 || idx+2 >= len(s) {
		return 0, fmt.Errorf("解析 /proc/%d/stat 失败", pid)
	}
	fields := strings.Fields(s[idx+2:])
	if len(fields) < 2 {
		return 0, fmt.Errorf("解析 /proc/%d/stat 字段不足", pid)
	}
	var ppid int
	fmt.Sscanf(fields[1], "%d", &ppid)
	return ppid, nil
}

// findCodexPts 搜索所有进程的 fd/0，找到进程名包含 codex 的 pts 设备。
func findCodexPts() string {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		// 读取进程名
		commData, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
		if err != nil {
			continue
		}
		comm := strings.TrimSpace(string(commData))
		// 查找 codex 相关进程
		if !strings.Contains(comm, "codex") {
			continue
		}
		// 读取 fd/0
		if link, err := os.Readlink(fmt.Sprintf("/proc/%d/fd/0", pid)); err == nil && isPtsDevice(link) {
			return link
		}
	}
	return ""
}

// InjectKeystroke 通过 TIOCSTI 向指定终端设备注入一个字节（模拟键盘按键）。
func InjectKeystroke(ttyPath string, key byte) error {
	fd, err := syscall.Open(ttyPath, syscall.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("打开 TTY %s 失败: %w", ttyPath, err)
	}
	defer syscall.Close(fd)

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), tiocsti, uintptr(unsafe.Pointer(&key)))
	if errno != 0 {
		return fmt.Errorf("TIOCSTI 注入失败: %w", errno)
	}
	return nil
}

// InjectKeystrokeToTTY 直接打开控制终端 /dev/tty 并通过 TIOCSTI 注入按键。
// 调用者必须与目标终端在同一会话中（不 setsid），否则打开 /dev/tty 会失败。
func InjectKeystrokeToTTY(key byte) error {
	fd, err := syscall.Open("/dev/tty", syscall.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("打开 /dev/tty 失败: %w", err)
	}
	defer syscall.Close(fd)

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), tiocsti, uintptr(unsafe.Pointer(&key)))
	if errno != 0 {
		return fmt.Errorf("TIOCSTI 注入失败: %w", errno)
	}
	return nil
}

// AppendLogSimple 追加日志到指定文件。
func AppendLogSimple(logPath, msg string) error {
	if logPath == "" {
		return nil
	}
	line := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}
