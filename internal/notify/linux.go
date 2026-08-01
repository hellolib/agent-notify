package notify

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hellolib/agent-notify/internal/linuxfocus"
)

type LinuxSender struct {
	run          Runner
	clickToFocus bool
	startFocus   linuxFocusStarter
	// sendNotify 发送 D-Bus 桌面通知；抽成字段以便测试注入（默认 linuxfocus.SendNotification）。
	sendNotify func(ctx context.Context, icon, title, body string) error
}

type linuxFocusStarter func(ctx context.Context, icon, title, body, windowID string) error

func NewLinuxSender(run Runner, clickToFocus bool) *LinuxSender {
	return &LinuxSender{run: run, clickToFocus: clickToFocus, startFocus: defaultLinuxFocusStarter, sendNotify: linuxfocus.SendNotification}
}

func NewLinuxSenderWithFocusStarter(run Runner, clickToFocus bool, starter linuxFocusStarter) *LinuxSender {
	return &LinuxSender{run: run, clickToFocus: clickToFocus, startFocus: starter, sendNotify: linuxfocus.SendNotification}
}

func (s *LinuxSender) Name() string { return "system" }

func (s *LinuxSender) Send(ctx context.Context, msg Message) error {
	// Use notify-send for Linux notifications
	// Format: notify-send "Title" "Body" [options]

	formattedBody := s.formatBody(msg)
	icon := AgentLogoPath(msg.Agent)

	if s.clickToFocus && s.startFocus != nil {
		if err := s.startFocus(ctx, icon, msg.Title, formattedBody, msg.FocusWindowID); err == nil {
			return nil
		}
	}

	// D-Bus 桌面通知（非交互）。
	if s.sendNotify != nil {
		if err := s.sendNotify(ctx, icon, msg.Title, formattedBody); err == nil {
			return nil
		}
	}

	// notify-send 回退：-a 用 agent 显示名，-i 仅在找到图标时注入（空串走桌面默认）。
	args := []string{"-a", appDisplayName(msg.Agent), "-u", "normal", "-t", "5000"}
	if icon != "" {
		args = append(args, "-i", icon)
	}
	args = append(args, msg.Title, formattedBody)
	return s.run(ctx, linuxfocus.CommandPath("notify-send"), args...)
}

func defaultLinuxFocusStarter(ctx context.Context, icon, title, body, windowID string) error {
	windowID = strings.TrimSpace(windowID)
	if windowID == "" {
		// 未命中 SessionStart 缓存时，回退到按进程树定位（旧行为）。
		var err error
		windowID, err = linuxfocus.ResolveWindowID(ctx, 0)
		if err != nil {
			return err
		}
	}
	return linuxfocus.StartDetached(ctx, linuxfocus.Request{
		Title:    title,
		Body:     body,
		WindowID: windowID,
		Icon:     icon,
	})
}

func (s *LinuxSender) formatBody(msg Message) string {
	timestamp := time.Now().Format("15:04:05")
	if msg.Workspace != "" {
		return fmt.Sprintf("%s\n%s\n%s", msg.Workspace, msg.Body, timestamp)
	}
	return fmt.Sprintf("%s\n%s", msg.Body, timestamp)
}
