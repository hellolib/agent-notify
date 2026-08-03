package toast

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
)

const (
	// DefaultFocusProtocol is the protocol used by the bundled focus helper.
	DefaultFocusProtocol = "anfocus"

	defaultFocusHelperName = "toast-focus-helper.exe"
)

// FocusActivation describes the protocol activation data used to focus the
// source window when a Windows toast is clicked.
type FocusActivation struct {
	Protocol  string
	Helper    string
	Arguments string
	Window    uintptr
}

// FocusActivationArguments formats the URI passed to WithActivationArguments.
func FocusActivationArguments(pid int, protocols ...string) string {
	protocol := DefaultFocusProtocol
	if len(protocols) > 0 && protocols[0] != "" {
		protocol = protocols[0]
	}
	return fmt.Sprintf("%s:%d", protocol, pid)
}

// buildFocusArguments 拼装 "protocol:pid[:hwndhex[:b64logpath]]"。
// logPath 非空时即便 hwnd==0 也保留 hwnd 槽位（填 0），确保字段位置对齐。
func buildFocusArguments(pid int, hwnd uintptr, logPath, protocol string) string {
	if protocol == "" {
		protocol = DefaultFocusProtocol
	}
	switch {
	case hwnd == 0 && logPath == "":
		return fmt.Sprintf("%s:%d", protocol, pid)
	case logPath == "":
		return fmt.Sprintf("%s:%d:%x", protocol, pid, hwnd)
	default:
		return fmt.Sprintf("%s:%d:%x:%s", protocol, pid, hwnd,
			base64.RawURLEncoding.EncodeToString([]byte(logPath)))
	}
}

// ParseFocusActivation 解析 helper 收到的激活 URI。
// 未识别的 hwnd/base64 一律降级为零值/空串，绝不报错阻断聚焦。
func ParseFocusActivation(uri string) (pid int, hwnd uintptr, logPath string, err error) {
	s := uri
	if i := strings.Index(s, ":"); i >= 0 {
		s = s[i+1:]
	}
	s = strings.TrimPrefix(s, "//")
	parts := strings.SplitN(s, ":", 3)

	pid, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, "", err
	}
	if len(parts) >= 2 && parts[1] != "" {
		if h, e := strconv.ParseUint(parts[1], 16, 0); e == nil {
			hwnd = uintptr(h)
		}
	}
	if len(parts) >= 3 && parts[2] != "" {
		if b, e := base64.RawURLEncoding.DecodeString(parts[2]); e == nil {
			logPath = string(b)
		}
	}
	return pid, hwnd, logPath, nil
}
