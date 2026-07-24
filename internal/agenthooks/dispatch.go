package agenthooks

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/linuxfocus"
	"github.com/hellolib/agent-notify/internal/notify"
	"github.com/hellolib/agent-notify/internal/state"
	"github.com/hellolib/agent-notify/internal/winfocus"
)

func Dispatch(ctx context.Context, cfg config.Config, statePath, logPath string, msg notify.Message) error {
	// hook 进程由终端 / IDE 启动，此处能从继承的环境变量识别宿主应用
	msg.SourceApp = notify.DetectSourceApp()
	// Windows 上 hook stdin 偶发把路径中的中文变成 '?'；用 env / Getwd 纠正
	msg.Workspace = notify.ResolveWorkspace(msg.Workspace)

	// session_start 是纯副作用事件：仅在 Linux / macOS / Windows 捕获点击聚焦的目标窗口并缓存，
	// 永不产生通知，也不受任何 agent 的事件配置控制。其它平台直接返回。
	if msg.Event == "session_start" {
		captureFocusWindow(ctx, statePath, logPath, msg)
		return nil
	}

	// 非 session_start：若命中 SessionStart 缓存，填充精确窗口供点击聚焦复用。
	// Linux 存的是 X11 window id（→ FocusWindowID）；macOS / Windows 存的是窗口快照
	// JSON（→ FocusCapture）：mac 是 mac-focus-helper --capture 的输出，Windows 是
	// winfocus 的 {"hwnd","title"}。
	if data, ok := state.NewFocusStore(state.FocusWindowsPath(statePath)).Load(msg.SessionID); ok {
		applyFocusCache(&msg, runtime.GOOS, data)
	}

	store := state.NewStore(statePath)
	senders := buildSenders(cfg, msg)
	if len(senders) == 0 {
		return state.AppendLog(logPath, fmt.Sprintf("no sender enabled for event=%s", msg.Event))
	}

	dispatcher := notify.NewDispatcher(store, time.Duration(cfg.Behavior.DedupeSeconds)*time.Second, senders...)
	timeout := time.Duration(cfg.Behavior.SendTimeoutSeconds) * time.Second
	sendCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := dispatcher.SendAll(sendCtx, msg); err != nil {
		return state.AppendLog(logPath, fmt.Sprintf("dispatch error event=%s session=%s err=%v", msg.Event, msg.SessionID, err))
	}

	return nil
}

func applyFocusCache(msg *notify.Message, goos, data string) {
	switch goos {
	case "darwin", "windows":
		msg.FocusCapture = data
	case "linux":
		msg.FocusWindowID = data
	}
}

// captureFocusWindow 在 SessionStart 时刻抓取"启动本 agent 的那个窗口"并按 session 缓存，
// 供后续事件（permission/input/completed…）点击聚焦复用，避免发通知时用户已切走导致抓错窗口。
// Linux 用 EWMH 抓 active window（经 PID 祖先校验）；macOS 用 mac-focus-helper --capture 走进程树定位；
// Windows 用 winfocus 抓前台窗（经 PID 祖先校验）。抓取失败（如焦点在别的应用、helper 缺失）时
// 不覆盖已有缓存。其它平台不做。
func captureFocusWindow(ctx context.Context, statePath, logPath string, msg notify.Message) {
	if msg.SessionID == "" {
		return
	}
	var windowData string
	switch runtime.GOOS {
	case "linux":
		windowID, err := linuxfocus.CaptureActiveWindow(ctx)
		if err != nil || windowID == "" {
			return
		}
		windowData = windowID
	case "darwin":
		capture, err := notify.CaptureMacWindow()
		if err != nil || capture == "" {
			return
		}
		windowData = capture
	case "windows":
		capture, err := winfocus.Capture()
		if err != nil || capture == "" {
			return
		}
		windowData = capture
	default:
		return
	}
	if err := state.NewFocusStore(state.FocusWindowsPath(statePath)).Save(msg.SessionID, windowData, time.Now()); err != nil {
		_ = state.AppendLog(logPath, fmt.Sprintf("focus capture save error session=%s err=%v", msg.SessionID, err))
	}
}

func buildSenders(cfg config.Config, msg notify.Message) []notify.Sender {
	var senders []notify.Sender

	notifyCfg := cfg.Notify.ClaudeCode
	switch msg.Agent {
	case "codex":
		notifyCfg = cfg.Notify.Codex
	case "zcode":
		notifyCfg = cfg.Notify.ZCode
	case "grok":
		notifyCfg = cfg.Notify.Grok
	}

	if !contains(notifyCfg.Events, msg.Event) {
		return senders
	}

	if notifyCfg.Channels.System.Enabled {
		senders = append(senders, notify.NewSystemSender(
			notify.DefaultRunner,
			notifyCfg.Channels.System.ClickToFocus,
			config.FocusPrecisionFromEnv(),
			notifyCfg.Channels.System.EffectiveFocusDebug(),
		))
	}
	if notifyCfg.Channels.Feishu.Enabled {
		senders = append(senders, notify.NewDefaultFeishuSender())
	}
	if notifyCfg.Channels.Wechat.Enabled && notifyCfg.Channels.Wechat.WebhookURL != "" {
		senders = append(senders, notify.NewWechatSender(notifyCfg.Channels.Wechat.WebhookURL))
	}
	if notifyCfg.Channels.WechatWork.Enabled && notifyCfg.Channels.WechatWork.WebhookURL != "" {
		senders = append(senders, notify.NewWechatWorkSender(notifyCfg.Channels.WechatWork.WebhookURL))
	}
	if notifyCfg.Channels.DingTalk.Enabled && notifyCfg.Channels.DingTalk.WebhookURL != "" {
		senders = append(senders, notify.NewDingTalkSender(notifyCfg.Channels.DingTalk.WebhookURL))
	}
	if notifyCfg.Channels.Bark.Enabled && notifyCfg.Channels.Bark.WebhookURL != "" {
		senders = append(senders, notify.NewBarkSender(notifyCfg.Channels.Bark.WebhookURL))
	}
	if notifyCfg.Channels.Ntfy.Enabled && notifyCfg.Channels.Ntfy.TopicURL != "" {
		senders = append(senders, notify.NewNtfySender(notifyCfg.Channels.Ntfy.TopicURL))
	}
	if notifyCfg.Channels.Slack.Enabled && notifyCfg.Channels.Slack.WebhookURL != "" {
		senders = append(senders, notify.NewSlackSender(notifyCfg.Channels.Slack.WebhookURL))
	}

	return senders
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
