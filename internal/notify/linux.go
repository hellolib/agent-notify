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
	sendNotify func(ctx context.Context, title, body string) error
}

type linuxFocusStarter func(ctx context.Context, title, body, windowID string) error

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
	if s.clickToFocus && s.startFocus != nil {
		if err := s.startFocus(ctx, msg.Title, formattedBody, msg.FocusWindowID); err == nil {
			return nil
		}
	}

	// notify-send arguments:
	// -a "Claude Code" sets app name
	// -u normal sets urgency
	// -t 5000 sets timeout in milliseconds (5 seconds)
	if s.sendNotify != nil {
		if err := s.sendNotify(ctx, msg.Title, formattedBody); err == nil {
			return nil
		}
	}
	return s.run(ctx, linuxfocus.CommandPath("notify-send"),
		"-a", "Claude Code",
		"-u", "normal",
		"-t", "5000",
		msg.Title,
		formattedBody,
	)
}

func defaultLinuxFocusStarter(ctx context.Context, title, body, windowID string) error {
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
	})
}

func (s *LinuxSender) formatBody(msg Message) string {
	timestamp := time.Now().Format("15:04:05")
	if msg.Workspace != "" {
		return fmt.Sprintf("%s\n%s\n%s", msg.Workspace, msg.Body, timestamp)
	}
	return fmt.Sprintf("%s\n%s", msg.Body, timestamp)
}
