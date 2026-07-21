//go:build linux

package linuxfocus

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/ewmh"
	"github.com/godbus/dbus/v5"
)

const (
	defaultNotificationTimeout = 15 * time.Second
	defaultWaitTimeout         = 20 * time.Second
	maxParentDepth             = 32
)

var commonBinDirs = []string{"/usr/bin", "/usr/local/bin", "/bin"}

type Request struct {
	Title    string
	Body     string
	WindowID string
}

func CommandPath(name string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	for _, dir := range commonBinDirs {
		path := filepath.Join(dir, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return name
}

func ResolveWindowID(ctx context.Context, startPID int) (string, error) {
	if startPID <= 0 {
		startPID = os.Getppid()
	}
	for pid, depth := startPID, 0; pid > 1 && depth < maxParentDepth; depth++ {
		if windowID, err := firstWindowForPID(ctx, pid); err == nil && windowID != "" {
			return windowID, nil
		}
		parent, err := parentPID(pid)
		if err != nil || parent <= 1 || parent == pid {
			break
		}
		pid = parent
	}
	return "", errors.New("linux focus window not found")
}

// CaptureActiveWindow 返回当前 _NET_ACTIVE_WINDOW，并校验它属于本进程树祖先
// （其 _NET_WM_PID 必须出现在从本进程向上的父进程链里）。这样在 SessionStart
// 时刻抓到的就是"运行本 agent 的那个终端窗口"，能区分同一终端进程的多个兄弟窗口。
// 校验不通过（例如用户此刻焦点在别的应用）时返回 error，调用方应据此放弃写入缓存。
func CaptureActiveWindow(_ context.Context) (string, error) {
	x, err := xgbutil.NewConn()
	if err != nil {
		return "", err
	}
	defer x.Conn().Close()

	active, err := ewmh.ActiveWindowGet(x)
	if err != nil {
		return "", err
	}
	if active == 0 {
		return "", errors.New("no active window")
	}
	wpid, err := ewmh.WmPidGet(x, active)
	if err != nil {
		return "", err
	}
	if !pidInAncestry(int(wpid)) {
		return "", fmt.Errorf("active window pid %d not in process ancestry", wpid)
	}
	return strconv.FormatUint(uint64(active), 10), nil
}

// pidInAncestry 判断 target 是否出现在从当前进程向上的父进程链中。
func pidInAncestry(target int) bool {
	if target <= 1 {
		return false
	}
	for pid, depth := os.Getpid(), 0; pid > 1 && depth < maxParentDepth; depth++ {
		if pid == target {
			return true
		}
		parent, err := parentPID(pid)
		if err != nil || parent <= 1 || parent == pid {
			break
		}
		pid = parent
	}
	return false
}

func StartDetached(ctx context.Context, req Request) error {
	if strings.TrimSpace(req.WindowID) == "" {
		return errors.New("linux focus window id is empty")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	cmd := exec.CommandContext(ctx, exe,
		"linux-notify-wait",
		"--title", req.Title,
		"--body", req.Body,
		"--window", req.WindowID,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0); err == nil {
		defer devNull.Close()
		cmd.Stdin = devNull
		cmd.Stdout = devNull
		cmd.Stderr = devNull
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func WaitNotifyAndFocus(ctx context.Context, req Request) error {
	if strings.TrimSpace(req.WindowID) == "" {
		return errors.New("linux focus window id is empty")
	}
	waitCtx, cancel := context.WithTimeout(ctx, defaultWaitTimeout)
	defer cancel()

	clicked, err := waitNotificationAction(waitCtx, req)
	if waitCtx.Err() != nil || err != nil || !clicked {
		return nil
	}
	return ActivateWindow(ctx, req.WindowID)
}

func SendNotification(ctx context.Context, title, body string) error {
	_, obj, closeFn, err := notificationBus()
	if err != nil {
		return err
	}
	defer closeFn()

	var id uint32
	return obj.CallWithContext(ctx, "org.freedesktop.Notifications.Notify", 0,
		"AgentNotify",
		uint32(0),
		"",
		title,
		body,
		[]string{},
		map[string]dbus.Variant{},
		int32(5000),
	).Store(&id)
}

func ActivateWindow(ctx context.Context, windowID string) error {
	windowID = strings.TrimSpace(windowID)
	if windowID == "" {
		return errors.New("linux focus window id is empty")
	}
	win, err := parseWindowID(windowID)
	if err != nil {
		return err
	}
	x, err := xgbutil.NewConn()
	if err != nil {
		return err
	}
	defer x.Conn().Close()
	if _, err := xproto.GetWindowAttributes(x.Conn(), win).Reply(); err != nil {
		return err
	}
	return ewmh.ActiveWindowReq(x, win)
}

func firstWindowForPID(_ context.Context, pid int) (string, error) {
	x, err := xgbutil.NewConn()
	if err != nil {
		return "", err
	}
	defer x.Conn().Close()
	clients, err := ewmh.ClientListGet(x)
	if err != nil {
		return "", err
	}
	for _, win := range clients {
		wpid, err := ewmh.WmPidGet(x, win)
		if err == nil && int(wpid) == pid {
			return strconv.FormatUint(uint64(win), 10), nil
		}
	}
	return "", errors.New("window not found")
}

func parentPID(pid int) (int, error) {
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}
	text := string(stat)
	end := strings.LastIndex(text, ")")
	if end < 0 || end+2 >= len(text) {
		return 0, errors.New("invalid proc stat")
	}
	fields := strings.Fields(text[end+2:])
	if len(fields) < 2 {
		return 0, errors.New("invalid proc stat fields")
	}
	return strconv.Atoi(fields[1])
}

func notificationClicked(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		if strings.TrimSpace(line) == "default" {
			return true
		}
	}
	return false
}

func waitNotificationAction(ctx context.Context, req Request) (bool, error) {
	conn, obj, closeFn, err := notificationBus()
	if err != nil {
		return false, err
	}
	defer closeFn()

	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath("/org/freedesktop/Notifications"),
		dbus.WithMatchInterface("org.freedesktop.Notifications"),
	); err != nil {
		return false, err
	}

	var id uint32
	err = obj.CallWithContext(ctx, "org.freedesktop.Notifications.Notify", 0,
		"AgentNotify",
		uint32(0),
		"",
		req.Title,
		req.Body,
		[]string{"default", "打开"},
		map[string]dbus.Variant{},
		int32(defaultNotificationTimeout/time.Millisecond),
	).Store(&id)
	if err != nil {
		return false, err
	}

	signals := make(chan *dbus.Signal, 8)
	conn.Signal(signals)
	defer conn.RemoveSignal(signals)

	timer := time.NewTimer(defaultNotificationTimeout + time.Second)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, nil
		case <-timer.C:
			closeNotification(context.Background(), obj, id)
			return false, nil
		case sig := <-signals:
			if sig == nil {
				continue
			}
			switch sig.Name {
			case "org.freedesktop.Notifications.ActionInvoked":
				if len(sig.Body) >= 2 {
					if nid, ok := sig.Body[0].(uint32); ok && nid == id {
						if action, ok := sig.Body[1].(string); ok && action == "default" {
							return true, nil
						}
					}
				}
			case "org.freedesktop.Notifications.NotificationClosed":
				if len(sig.Body) >= 1 {
					if nid, ok := sig.Body[0].(uint32); ok && nid == id {
						return false, nil
					}
				}
			}
		}
	}
}

func closeNotification(ctx context.Context, obj dbus.BusObject, id uint32) {
	if id == 0 {
		return
	}
	_ = obj.CallWithContext(ctx, "org.freedesktop.Notifications.CloseNotification", 0, id).Err
}

func notificationBus() (*dbus.Conn, dbus.BusObject, func(), error) {
	conn, err := dbus.SessionBusPrivate()
	if err != nil {
		return nil, nil, nil, err
	}
	closeFn := func() { conn.Close() }
	if err := conn.Auth(nil); err != nil {
		closeFn()
		return nil, nil, nil, err
	}
	if err := conn.Hello(); err != nil {
		closeFn()
		return nil, nil, nil, err
	}
	obj := conn.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	return conn, obj, closeFn, nil
}

func parseWindowID(s string) (xproto.Window, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("window id is empty")
	}
	base := 10
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		base = 0
	}
	value, err := strconv.ParseUint(s, base, 32)
	if err != nil {
		return 0, err
	}
	return xproto.Window(value), nil
}
