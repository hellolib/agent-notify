package notify

import (
	"context"
	"regexp"
	"strings"
	"testing"
)

const mockExe = "/mock/terminal-notifier"

// mockResolver 返回一个固定可执行路径，模拟 terminal-notifier 已安装。
func mockResolver() string { return mockExe }

func TestMacOSSenderSendFallbackToOsascript(t *testing.T) {
	var gotName string
	var gotArgs []string
	callCount := 0

	sender := NewMacOSSenderWithResolver(func(_ context.Context, name string, args ...string) error {
		callCount++
		if name == mockExe {
			// 模拟 terminal-notifier 调用失败 → 回退 osascript
			return context.DeadlineExceeded
		}
		gotName = name
		gotArgs = args
		return nil
	}, true, func() string { return "" }) // notifierPath 返回空 → 直接走 osascript

	if err := sender.Send(context.Background(), Message{Title: "Title", Body: "Body", Workspace: "/path"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if gotName != "osascript" {
		t.Fatalf("name = %q, want osascript", gotName)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "-e" {
		t.Fatalf("args = %#v, want osascript script args", gotArgs)
	}
	if callCount < 1 {
		t.Fatalf("callCount = %d, expected at least 1", callCount)
	}
}

func TestMacOSSenderSendUsesTerminalNotifier(t *testing.T) {
	var calls []struct {
		name string
		args []string
	}

	sender := NewMacOSSenderWithResolver(func(_ context.Context, name string, args ...string) error {
		calls = append(calls, struct {
			name string
			args []string
		}{name, args})
		return nil
	}, true, mockResolver)

	if err := sender.Send(context.Background(), Message{Title: "Title", Body: "Body", Workspace: "/path"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if len(calls) < 1 || calls[0].name != mockExe {
		t.Fatalf("expected first call to terminal-notifier at %s, got %#v", mockExe, calls)
	}
}

func TestMacOSSenderTerminalNotifierExecutesOpenBundle(t *testing.T) {
	newSenderArgs := func(msg Message, clickToFocus bool) []string {
		var gotArgs []string
		sender := NewMacOSSenderWithResolver(func(_ context.Context, name string, args ...string) error {
			if name == mockExe {
				gotArgs = args
			}
			return nil
		}, clickToFocus, mockResolver)
		if err := sender.Send(context.Background(), msg); err != nil {
			t.Fatalf("Send() error = %v", err)
		}
		return gotArgs
	}

	hasExecute := func(args []string, command string) bool {
		for i, a := range args {
			if a == "-execute" && i+1 < len(args) && args[i+1] == command {
				return true
			}
		}
		return false
	}

	// 有 BundleID + clickToFocus 开启 → 含 -execute open -b
	args := newSenderArgs(Message{Title: "Title", Body: "Body", SourceApp: SourceApp{BundleID: "com.googlecode.iterm2"}}, true)
	if !hasExecute(args, "open -b com.googlecode.iterm2") {
		t.Fatalf("args = %#v, want -execute open -b com.googlecode.iterm2", args)
	}

	// clickToFocus 关闭 → 不含 -execute
	args = newSenderArgs(Message{Title: "Title", Body: "Body", SourceApp: SourceApp{BundleID: "com.googlecode.iterm2"}}, false)
	for _, a := range args {
		if a == "-execute" {
			t.Fatalf("args = %#v, unexpected -execute when clickToFocus disabled", args)
		}
	}

	// 无 BundleID → 不含 -execute
	args = newSenderArgs(Message{Title: "Title", Body: "Body"}, true)
	for _, a := range args {
		if a == "-execute" {
			t.Fatalf("args = %#v, unexpected -execute without SourceApp", args)
		}
	}
}

func TestOpenBundleCommandRejectsUnsafeBundleID(t *testing.T) {
	if got := openBundleCommand("com.microsoft.VSCode"); got != "open -b com.microsoft.VSCode" {
		t.Fatalf("openBundleCommand() = %q", got)
	}
	if got := openBundleCommand("com.example.app; touch /tmp/bad"); got != "" {
		t.Fatalf("openBundleCommand() = %q, want empty for unsafe bundle id", got)
	}
}

func TestMacOSSenderGroupPerAgent(t *testing.T) {
	var gotArgs []string
	sender := NewMacOSSenderWithResolver(func(_ context.Context, name string, args ...string) error {
		if name == mockExe {
			gotArgs = args
		}
		return nil
	}, true, mockResolver)

	if err := sender.Send(context.Background(), Message{Agent: "codex", Title: "T", Body: "B"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	found := false
	for i, a := range gotArgs {
		if a == "-group" && i+1 < len(gotArgs) && gotArgs[i+1] == "com.agent-notify.codex" {
			found = true
		}
	}
	if !found {
		t.Fatalf("args = %#v, want -group com.agent-notify.codex", gotArgs)
	}
}

func TestMacOSSenderSubtitleHasTimestamp(t *testing.T) {
	var gotArgs []string
	sender := NewMacOSSenderWithResolver(func(_ context.Context, name string, args ...string) error {
		if name == mockExe {
			gotArgs = args
		}
		return nil
	}, true, mockResolver)

	if err := sender.Send(context.Background(), Message{Agent: "codex", Title: "T", Body: "B"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	// subtitle 应含时间戳(HH:MM:SS)
	got := ""
	for i, a := range gotArgs {
		if a == "-subtitle" && i+1 < len(gotArgs) {
			got = gotArgs[i+1]
		}
	}
	if !regexp.MustCompile(`^\d{2}:\d{2}:\d{2}$`).MatchString(got) {
		t.Fatalf("subtitle = %q, want HH:MM:SS timestamp", got)
	}
}

func TestMacOSSenderFormatBodyNoTimestamp(t *testing.T) {
	sender := NewMacOSSenderWithResolver(func(_ context.Context, name string, args ...string) error {
		return nil
	}, true, mockResolver)

	// 正文不含时间戳，时间已移至 subtitle
	body := sender.formatBody(Message{Body: "任务完成", Workspace: "/repo"})
	if containsAlarmEmoji(body) || strings.Contains(body, "15:") {
		t.Fatalf("body = %q, should not contain timestamp", body)
	}
}

// containsAlarmEmoji 检查是否含 ⏰ 标记。
func containsAlarmEmoji(s string) bool {
	for _, r := range s {
		if r == '⏰' {
			return true
		}
	}
	return false
}

func TestShortenWorkspace(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"/Users/foo/workspace/github/hellolib/agent-notify", "hellolib/agent-notify"},
		{"/repo/x", "/repo/x"},           // 两段，原样
		{"agent-notify", "agent-notify"}, // 一段，原样
		{"/a/b/c/d", "c/d"},              // 多段取末尾两段
		{"/Users/foo/./x", "./x"},        // 四段取末两段
		{`C:\Users\demo\project`, "demo/project"},
	}
	for _, c := range cases {
		got := shortenWorkspace(c.in)
		if got != c.want {
			t.Errorf("shortenWorkspace(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
