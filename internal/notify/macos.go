package notify

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Runner func(ctx context.Context, name string, args ...string) error

// PathResolver 返回 terminal-notifier 可执行文件路径，找不到返回空串。
type PathResolver func() string

type MacOSSender struct {
	run                Runner
	clickToFocus       bool
	focusPrecision     string
	notifierPath       PathResolver
	macFocusHelperPath func() string // resolves mac-focus-helper path; "" if absent
	ppid               func() int
}

// NewMacOSSender 构造 macOS 系统通知发送器。clickToFocus 为 true 时，点击通知会激活宿主应用。
// focusPrecision 控制聚焦精度（"app" | "window"），窗口级行为在 Task 3 实现。
func NewMacOSSender(run Runner, clickToFocus bool, focusPrecision string) *MacOSSender {
	return &MacOSSender{
		run:                run,
		clickToFocus:       clickToFocus,
		focusPrecision:     focusPrecision,
		notifierPath:       defaultTerminalNotifierPath,
		macFocusHelperPath: defaultMacFocusHelperPath,
		ppid:               os.Getppid,
	}
}

// NewMacOSSenderWithResolver 供测试注入 notifierPath 解析。
func NewMacOSSenderWithResolver(run Runner, clickToFocus bool, focusPrecision string, resolver PathResolver) *MacOSSender {
	return &MacOSSender{
		run:                run,
		clickToFocus:       clickToFocus,
		focusPrecision:     focusPrecision,
		notifierPath:       resolver,
		macFocusHelperPath: defaultMacFocusHelperPath,
		ppid:               os.Getppid,
	}
}

func DefaultRunner(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}

func (s *MacOSSender) Name() string { return "system" }

func (s *MacOSSender) Send(ctx context.Context, msg Message) error {
	// Use terminal-notifier if available for better notifications with icon support
	if s.tryTerminalNotifier(ctx, msg) {
		return nil
	}

	// Fallback to osascript with improved content
	formattedBody := s.formatBody(msg)
	script := fmt.Sprintf(`display notification %q with title %q sound name "Submarine"`, formattedBody, msg.Title)
	return s.run(ctx, "osascript", "-e", script)
}

// defaultTerminalNotifierPath 返回 terminal-notifier 可执行文件路径：
// 优先用随 npx 解压到 ~/.agent-notify/terminal-notifier.app 的本地预置 bundle，
// 其次回退到系统 PATH 上的 terminal-notifier。找不到返回空串。
func defaultTerminalNotifierPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		localExe := filepath.Join(home, ".agent-notify", "terminal-notifier.app", "Contents", "MacOS", "terminal-notifier")
		if info, err := os.Stat(localExe); err == nil && !info.IsDir() {
			return localExe
		}
	}
	if p, err := exec.LookPath("terminal-notifier"); err == nil {
		return p
	}
	return ""
}

// defaultMacFocusHelperPath returns the mac-focus-helper executable path:
// prefer ~/.agent-notify/mac-focus-helper, else "" (window-level degrades to app-level).
func defaultMacFocusHelperPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".agent-notify", "mac-focus-helper")
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

// agentIconPath 按 agent 选择应用图标；图标文件存在才返回，否则返回空串。
func agentIconPath(agent string) string {
	var app string
	switch agent {
	case "codex":
		app = "Codex.app"
	case "zcode":
		app = "ZCode.app"
	default:
		app = "Claude.app"
	}
	icon := filepath.Join("/Applications", app, "Contents", "Resources", "AppIcon.icns")
	if info, err := os.Stat(icon); err == nil && !info.IsDir() {
		return icon
	}
	return ""
}

// tryTerminalNotifier attempts to use terminal-notifier for richer notifications
func (s *MacOSSender) tryTerminalNotifier(ctx context.Context, msg Message) bool {
	exe := s.notifierPath()
	if exe == "" {
		return false
	}

	args := []string{
		"-title", msg.Title,
		"-subtitle", time.Now().Format("15:04:05"),
		"-message", s.formatBody(msg),
		"-sound", "Submarine",
		"-group", fmt.Sprintf("com.agent-notify.%s", msg.Agent),
	}

	// 点击通知时激活触发事件的宿主应用（终端 / IDE）。
	// 窗口级聚焦在 helper 可用时调用 mac-focus-helper，否则降级到 app 级 open -b。
	if s.clickToFocus {
		if exec := s.focusExecuteCommand(msg); exec != "" {
			args = append(args, "-execute", exec)
		}
	}

	// 图标按 agent 选择，存在才追加
	if icon := agentIconPath(msg.Agent); icon != "" {
		args = append(args, "-appIcon", icon)
	}

	// terminal-notifier returns 0 on success, non-zero on failure
	return s.run(ctx, exe, args...) == nil
}

func openBundleCommand(bundleID string) string {
	if !isSafeBundleID(bundleID) {
		return ""
	}
	return "open -b " + bundleID
}

// captureWindowInfo holds the JSON output from mac-focus-helper --capture.
type captureWindowInfo struct {
	WindowID uint32 `json:"window_id"`
	OwnerPID int    `json:"owner_pid"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	W        int    `json:"w"`
	H        int    `json:"h"`
	Title    string `json:"title"`
}

// focusExecuteCommand builds the -execute payload. Returns "" when nothing
// should be executed on click (no bundle / unsafe bundle).
func (s *MacOSSender) focusExecuteCommand(msg Message) string {
	bundle := msg.SourceApp.BundleID
	if !isSafeBundleID(bundle) {
		return ""
	}
	if s.focusPrecision == "window" {
		if helper := s.macFocusHelperPath(); helper != "" {
			// Call helper --capture to get current window info.
			info, err := captureWindowInfoFromHelper(helper)
			if err == nil && info.WindowID > 0 {
				return fmt.Sprintf("%s --owner-pid %d --bundle %s --window-id %d --x %d --y %d --w %d --h %d --title %s",
					shellQuote(helper), info.OwnerPID, bundle,
					info.WindowID, info.X, info.Y, info.W, info.H,
					shellQuote(info.Title))
			}
			// capture failed, fall back to process-tree walk at click time.
			return fmt.Sprintf("%s --owner-pid %d --bundle %s", shellQuote(helper), s.ppid(), bundle)
		}
	}
	return "open -b " + bundle
}

// captureWindowInfoFromHelper runs mac-focus-helper --capture and parses the JSON output.
func captureWindowInfoFromHelper(helperPath string) (captureWindowInfo, error) {
	out, err := exec.Command(helperPath, "--capture").Output()
	if err != nil {
		return captureWindowInfo{}, err
	}
	var info captureWindowInfo
	// Minimal JSON parse — avoid importing encoding/json for a small fixed struct.
	// Format: {"window_id":N,"owner_pid":N,"bundle":"...","title":"...","x":N,"y":N,"w":N,"h":N,"reason":"..."}
	s := string(out)
	info.WindowID = jsonUint32(s, "window_id")
	info.OwnerPID = jsonInt(s, "owner_pid")
	info.X = jsonInt(s, "x")
	info.Y = jsonInt(s, "y")
	info.W = jsonInt(s, "w")
	info.H = jsonInt(s, "h")
	info.Title = jsonString(s, "title")
	return info, nil
}

func jsonUint32(s, key string) uint32 {
	return uint32(jsonInt(s, key))
}

func jsonInt(s, key string) int {
	needle := `"` + key + `":`
	idx := strings.Index(s, needle)
	if idx < 0 {
		return 0
	}
	s = s[idx+len(needle):]
	// skip whitespace
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	neg := false
	if len(s) > 0 && s[0] == '-' {
		neg = true
		s = s[1:]
	}
	n := 0
	for len(s) >= 1 && s[0] >= '0' && s[0] <= '9' {
		n = n*10 + int(s[0]-'0')
		s = s[1:]
	}
	if neg {
		return -n
	}
	return n
}

func jsonString(s, key string) string {
	needle := `"` + key + `":"`
	idx := strings.Index(s, needle)
	if idx < 0 {
		return ""
	}
	s = s[idx+len(needle):]
	end := strings.Index(s, `"`)
	if end < 0 {
		return ""
	}
	return s[:end]
}

// shellQuote single-quote escapes a string for safe inclusion in a shell command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func isSafeBundleID(bundleID string) bool {
	if bundleID == "" {
		return false
	}
	for _, r := range bundleID {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '.' || r == '-' {
			continue
		}
		return false
	}
	return true
}

// formatBody formats the notification body. 时间戳已移至 subtitle，
// 正文只留工作区（缩短为末尾项目名，避免长路径换行）与消息。
func (s *MacOSSender) formatBody(msg Message) string {
	if msg.Workspace != "" {
		return fmt.Sprintf("📁 %s\n%s", shortenWorkspace(msg.Workspace), msg.Body)
	}
	return msg.Body
}
