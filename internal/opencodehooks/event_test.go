package opencodehooks

import (
	"bytes"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestParseSessionCreated(t *testing.T) {
	input := `{"type":"session.created","properties":{"sessionID":"sess-oc-0","directory":"/Users/demo/project"}}`

	msg, err := ParseMessage(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Agent != "opencode" {
		t.Fatalf("Agent = %q, want opencode", msg.Agent)
	}
	if msg.Event != "session_start" {
		t.Fatalf("Event = %q, want session_start", msg.Event)
	}
	if msg.SessionID != "sess-oc-0" {
		t.Fatalf("SessionID = %q, want sess-oc-0", msg.SessionID)
	}
	if msg.Workspace != "/Users/demo/project" {
		t.Fatalf("Workspace = %q, want /Users/demo/project", msg.Workspace)
	}
}

func TestParsePermissionAsked(t *testing.T) {
	input := `{"type":"permission.asked","properties":{"sessionID":"s1","directory":"/p","permission":"bash"}}`

	msg, err := ParseMessage(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "permission_required" {
		t.Fatalf("Event = %q, want permission_required", msg.Event)
	}
	if !strings.Contains(msg.Body, "bash") {
		t.Fatalf("Body = %q, want permission text containing bash", msg.Body)
	}
}

func TestParseSessionStatusIdle(t *testing.T) {
	input := `{"type":"session.status","properties":{"sessionID":"s2","directory":"/p","status":{"type":"idle"}}}`

	msg, err := ParseMessage(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "input_required" {
		t.Fatalf("Event = %q, want input_required", msg.Event)
	}
}

func TestParseSessionStatusBusySkipped(t *testing.T) {
	input := `{"type":"session.status","properties":{"sessionID":"s2","directory":"/p","status":{"type":"busy"}}}`

	if _, err := ParseMessage(bytes.NewReader([]byte(input))); err == nil {
		t.Fatal("ParseMessage() expected error for busy status, got nil")
	}
}

func TestParseSessionIdle(t *testing.T) {
	input := `{"type":"session.idle","properties":{"sessionID":"s3","directory":"/p"}}`

	msg, err := ParseMessage(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "run_completed" {
		t.Fatalf("Event = %q, want run_completed", msg.Event)
	}
	if !strings.Contains(msg.Body, "完成") {
		t.Fatalf("Body = %q, want completion hint", msg.Body)
	}
}

func TestParseSessionError(t *testing.T) {
	input := `{"type":"session.error","properties":{"sessionID":"s4","directory":"/p","error":{"message":"boom"}}}`

	msg, err := ParseMessage(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "run_failed" {
		t.Fatalf("Event = %q, want run_failed", msg.Event)
	}
	if !strings.Contains(msg.Body, "boom") {
		t.Fatalf("Body = %q, want error text containing boom", msg.Body)
	}
}

func TestParseUnsupportedEvent(t *testing.T) {
	input := `{"type":"unknown.event","properties":{"sessionID":"s"}}`
	if _, err := ParseMessage(bytes.NewReader([]byte(input))); err == nil {
		t.Fatal("ParseMessage() expected error for unknown event, got nil")
	}
}

func TestParseQuestionAsked(t *testing.T) {
	input := `{"type":"question.asked","properties":{"sessionID":"s6","directory":"/p","id":"q1",` +
		`"questions":[{"header":"任务类型","question":"你想让我帮你处理哪类任务？"},` +
		`{"header":"项目范围","question":"范围多大？"},` +
		`{"header":"回复风格","question":"要多详细？"}]}}`

	msg, err := ParseMessage(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Agent != "opencode" {
		t.Fatalf("Agent = %q, want opencode", msg.Agent)
	}
	// 选择题挂起时会话仍是 busy，session.status(idle)/session.idle 都不会触发，
	// 因此必须由 question.asked 自己映射为 input_required。
	if msg.Event != "input_required" {
		t.Fatalf("Event = %q, want input_required", msg.Event)
	}
	if msg.SessionID != "s6" {
		t.Fatalf("SessionID = %q, want s6", msg.SessionID)
	}
	if msg.Workspace != "/p" {
		t.Fatalf("Workspace = %q, want /p", msg.Workspace)
	}
	if !strings.Contains(msg.Body, "3 个问题") {
		t.Fatalf("Body = %q, want question count", msg.Body)
	}
	if !strings.Contains(msg.Body, "你想让我帮你处理哪类任务？") {
		t.Fatalf("Body = %q, want first question text", msg.Body)
	}
}

func TestQuestionBodySingle(t *testing.T) {
	got := questionBody([]questionInfo{{Header: "任务类型", Question: "你想让我帮你处理哪类任务？"}})
	if got != "你想让我帮你处理哪类任务？" {
		t.Fatalf("questionBody() = %q, want bare question without count prefix", got)
	}
}

func TestQuestionBodyEmpty(t *testing.T) {
	got := questionBody(nil)
	if !strings.Contains(got, "回答") {
		t.Fatalf("questionBody(nil) = %q, want fallback text", got)
	}
}

// question 缺失时应退回 header 短标签，而不是留空。
func TestQuestionBodyHeaderFallback(t *testing.T) {
	got := questionBody([]questionInfo{{Header: "任务类型"}})
	if !strings.Contains(got, "任务类型") {
		t.Fatalf("questionBody() = %q, want header fallback", got)
	}
}

// questions 非空但内容全空时仍应保留数量信息。
func TestQuestionBodyBlankKeepsCount(t *testing.T) {
	got := questionBody([]questionInfo{{}, {}})
	if !strings.Contains(got, "2 个问题") {
		t.Fatalf("questionBody() = %q, want count for blank questions", got)
	}
}

func TestQuestionBodyBounded(t *testing.T) {
	long := strings.Repeat("x", 500)
	got := questionBody([]questionInfo{{Question: long}, {Question: long}})
	if n := len([]rune(got)); n > 300 {
		t.Fatalf("questionBody too long: %d runes", n)
	}
}

// 回归测试：插件 JS 里 subscribed 白名单转发的每个事件，
// ParseMessage 都必须能处理。issue #51 的根因就是两边不同步——
// 事件被 JS 丢掉，Go 侧再正确也收不到。
func TestPluginSubscriptionMatchesParser(t *testing.T) {
	// 每个订阅事件的最小可解析样例。新增订阅事件时必须在此登记，
	// 否则测试会失败并提示补充。
	samples := map[string]string{
		"session.created":  `{"type":"session.created","properties":{"sessionID":"s","directory":"/p"}}`,
		"permission.asked": `{"type":"permission.asked","properties":{"sessionID":"s","permission":"bash"}}`,
		"question.asked":   `{"type":"question.asked","properties":{"sessionID":"s","questions":[{"header":"h","question":"q"}]}}`,
		"session.status":   `{"type":"session.status","properties":{"sessionID":"s","status":{"type":"idle"}}}`,
		"session.idle":     `{"type":"session.idle","properties":{"sessionID":"s"}}`,
		"session.error":    `{"type":"session.error","properties":{"sessionID":"s","error":{"message":"boom"}}}`,
	}

	subscribed := parseSubscribedEvents(t, PluginJS)
	if len(subscribed) == 0 {
		t.Fatal("no subscribed events found in plugin JS")
	}

	for _, event := range subscribed {
		sample, ok := samples[event]
		if !ok {
			t.Fatalf("plugin subscribes to %q but the test has no sample payload; add one", event)
		}
		if _, err := ParseMessage(bytes.NewReader([]byte(sample))); err != nil {
			t.Fatalf("plugin forwards %q but ParseMessage rejected it: %v", event, err)
		}
	}

	// 反向：登记的样例事件也应全部在插件白名单里，防止 Go 支持了但 JS 没转发。
	for event := range samples {
		if !slices.Contains(subscribed, event) {
			t.Fatalf("ParseMessage handles %q but plugin JS does not subscribe to it", event)
		}
	}
}

// parseSubscribedEvents 从插件 JS 的 subscribed Set 字面量中提取事件名。
func parseSubscribedEvents(t *testing.T, js string) []string {
	t.Helper()
	block := regexp.MustCompile(`(?s)const subscribed = new Set\(\[(.*?)\]\)`).FindStringSubmatch(js)
	if block == nil {
		t.Fatal("could not locate subscribed Set in plugin JS")
	}
	var events []string
	for _, m := range regexp.MustCompile(`"([^"]+)"`).FindAllStringSubmatch(block[1], -1) {
		events = append(events, m[1])
	}
	return events
}

func TestPermissionBodyBounded(t *testing.T) {
	long := strings.Repeat("x", 500)
	got := permissionBody(long)
	if n := len([]rune(got)); n > 300 {
		t.Fatalf("permissionBody too long: %d runes", n)
	}
}

func TestPermissionBodyEmpty(t *testing.T) {
	got := permissionBody("")
	if !strings.Contains(got, "授权") {
		t.Fatalf("permissionBody(\"\") = %q, want default permission text", got)
	}
}

func TestFailedErrorBodyFallback(t *testing.T) {
	input := `{"type":"session.error","properties":{"sessionID":"s5","directory":"/p"}}`

	msg, err := ParseMessage(bytes.NewReader([]byte(input)))
	if err != nil {
		t.Fatalf("ParseMessage() error = %v", err)
	}
	if msg.Event != "run_failed" {
		t.Fatalf("Event = %q, want run_failed", msg.Event)
	}
	if !strings.Contains(msg.Body, "错误") {
		t.Fatalf("Body = %q, want fallback error text", msg.Body)
	}
}
