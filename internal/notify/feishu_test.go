package notify

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type stubFeishuConfigProvider struct {
	cfg FeishuCLIConfig
	err error
}

func (p stubFeishuConfigProvider) Parse() (FeishuCLIConfig, error) {
	if p.err != nil {
		return FeishuCLIConfig{}, p.err
	}
	return p.cfg, nil
}

type stubFeishuMessenger struct {
	creatorAppID  string
	creatorOpenID string
	sentReceiveID string
	sentCard      map[string]any
	creatorErr    error
	sendErr       error
}

func (m *stubFeishuMessenger) CreatorOpenID(ctx context.Context, appID string) (string, error) {
	m.creatorAppID = appID
	if m.creatorErr != nil {
		return "", m.creatorErr
	}
	return m.creatorOpenID, nil
}

func (m *stubFeishuMessenger) SendCard(ctx context.Context, receiveOpenID string, card map[string]any) error {
	m.sentReceiveID = receiveOpenID
	m.sentCard = card
	return m.sendErr
}

func TestFeishuSenderSendUsesCLIConfigAndCreator(t *testing.T) {
	provider := stubFeishuConfigProvider{
		cfg: FeishuCLIConfig{
			AppID:     "cli_app",
			AppSecret: "secret",
		},
	}
	messenger := &stubFeishuMessenger{creatorOpenID: "ou_creator"}
	sender := NewFeishuSender(provider)
	sender.newMessenger = func(appID, appSecret string) (feishuMessenger, error) {
		if appID != "cli_app" {
			t.Fatalf("appID = %q, want cli_app", appID)
		}
		if appSecret != "secret" {
			t.Fatalf("appSecret = %q, want secret", appSecret)
		}
		return messenger, nil
	}

	msg := Message{Event: "permission_required", SessionID: "session-123", Workspace: "/path/to/project", Title: "Claude Code 等待授权", Body: "项目: demo"}
	if err := sender.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if messenger.creatorAppID != "cli_app" {
		t.Fatalf("creator appID = %q, want cli_app", messenger.creatorAppID)
	}
	if messenger.sentReceiveID != "ou_creator" {
		t.Fatalf("receiveOpenID = %q, want ou_creator", messenger.sentReceiveID)
	}
	if messenger.sentCard == nil {
		t.Fatal("sentCard is nil, want card")
	}
	if got := messenger.sentCard["schema"]; got != "2.0" {
		t.Fatalf("card schema = %#v, want 2.0", got)
	}
	// Verify card has header with title
	header, ok := messenger.sentCard["header"].(map[string]any)
	if !ok {
		t.Fatal("card header is missing")
	}
	title, ok := header["title"].(map[string]any)
	if !ok {
		t.Fatal("card header title is missing")
	}
	if title["content"] == nil {
		t.Fatal("card header title content is missing")
	}
}

func TestFeishuSenderSendReturnsConfigError(t *testing.T) {
	sender := NewFeishuSender(stubFeishuConfigProvider{err: errors.New("missing config")})

	err := sender.Send(context.Background(), Message{Title: "t", Body: "b"})
	if err == nil {
		t.Fatal("Send() error = nil, want config error")
	}
	if err.Error() != "missing config" {
		t.Fatalf("Send() error = %v, want missing config", err)
	}
}

func TestBuildCardContainsBody(t *testing.T) {
	sender := &FeishuSender{}
	msg := Message{
		Event:     "permission_required",
		Title:     "测试标题",
		Body:      "这是测试消息内容",
		Workspace: "/test/path",
	}

	card := sender.buildCard(msg)

	elements := cardBodyElements(t, card)
	visibleText := cardElementText(elements)
	for _, want := range []string{"**消息内容**", "这是测试消息内容"} {
		if !contains(visibleText, want) {
			t.Errorf("ordinary card text = %q, missing %q", visibleText, want)
		}
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestBuildCardFooterDoesNotHardcodeClaudeCode(t *testing.T) {
	sender := &FeishuSender{}
	card := sender.buildCard(Message{Event: "run_completed", Title: "Codex 运行完成", Body: "done"})

	elements := cardBodyElements(t, card)

	foundClaudeCode := false
	for _, el := range elements {
		elMap, ok := el.(map[string]any)
		if !ok {
			continue
		}
		content, _ := elMap["content"].(string)
		if contains(content, "Claude Code") {
			foundClaudeCode = true
		}
		if text, ok := elMap["text"].(map[string]any); ok {
			content, _ := text["content"].(string)
			if contains(content, "Claude Code") {
				foundClaudeCode = true
			}
		}
	}

	if foundClaudeCode {
		t.Fatal("card footer should not hardcode Claude Code")
	}
}

func TestBuildCardOmitsWorkspaceForCodexNotification(t *testing.T) {
	sender := &FeishuSender{}
	card := sender.buildCard(Message{Event: "run_completed", Title: "运行完成", Body: "done", Workspace: "/tmp/project", Agent: "codex"})

	elements := cardBodyElements(t, card)

	for _, el := range elements {
		elMap, ok := el.(map[string]any)
		if !ok {
			continue
		}
		text, ok := elMap["text"].(map[string]any)
		if !ok {
			continue
		}
		content, _ := text["content"].(string)
		if contains(content, "**工作目录**") {
			t.Fatalf("card should omit workspace for Codex notification, got %q", content)
		}
	}
}

func TestBuildCardRendersQuestionsAsReadOnlyText(t *testing.T) {
	sender := &FeishuSender{}
	card := sender.buildCard(Message{
		Agent: "codex",
		Event: "input_required",
		Title: "Codex 等待输入",
		Body: "Codex 正在等待您的输入\n" +
			"1. 环境: 使用哪个环境？🌏\n" +
			"   - 生产: 面向真实用户\n" +
			"   - 测试: 仅用于验证\n" +
			"   - 其他（自由输入）\n" +
			"2. 密钥: 请输入令牌 [敏感输入]\n" +
			"请回到 Codex 终端提交答案",
		Questions: []Question{
			{
				ID:       "environment",
				Header:   "环境",
				Question: "使用哪个环境？🌏",
				Options: []QuestionOption{
					{Label: "生产", Description: "面向真实用户"},
					{Label: "测试", Description: "仅用于验证"},
				},
				IsOther:  true,
				IsSecret: false,
			},
			{
				Header:   "密钥",
				Question: "请输入令牌",
				IsSecret: true,
			},
		},
	})

	elements := cardBodyElements(t, card)
	visibleText := cardElementText(elements)

	for _, unwanted := range []string{"消息内容", "Codex 正在等待您的输入"} {
		if contains(visibleText, unwanted) {
			t.Errorf("structured question card text = %q, should not contain %q", visibleText, unwanted)
		}
	}
	for _, wantOnce := range []string{
		"**问题 1** · 环境",
		"使用哪个环境？🌏",
		"生产",
		"面向真实用户",
		"测试",
		"仅用于验证",
		"其他（自由输入）",
		"• 生产： 面向真实用户",
		"• 测试： 仅用于验证",
		"• 其他（自由输入）：请回到 Codex 终端输入",
		"**问题 2** · 密钥 🔒 敏感输入",
		"请输入令牌",
		"敏感输入",
		"请回到 Codex 终端提交答案",
	} {
		if got := strings.Count(visibleText, wantOnce); got != 1 {
			t.Errorf("structured question card contains %q %d times, want exactly once; text = %q", wantOnce, got, visibleText)
		}
	}

	var foundQuestion, foundHeader, foundSecret, foundFooter bool
	var optionContents []string
	for _, raw := range elements {
		el, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if el["tag"] == "button" {
			t.Fatalf("read-only question card must not contain buttons: %#v", el)
		}
		if _, exists := el["behaviors"]; exists {
			t.Fatalf("read-only question card must not contain callback behaviors: %#v", el)
		}
		text, _ := el["text"].(map[string]any)
		content, _ := text["content"].(string)
		if contains(content, "使用哪个环境？🌏") {
			foundQuestion = true
		}
		if contains(content, "环境") {
			foundHeader = true
		}
		if contains(content, "敏感输入") {
			foundSecret = true
		}
		if el["tag"] == "div" && strings.HasPrefix(content, "• ") {
			optionContents = append(optionContents, content)
		}
		if el["tag"] == "markdown" {
			markdownContent, _ := el["content"].(string)
			if contains(markdownContent, "请回到 Codex 终端提交") {
				foundFooter = true
			}
		}
	}

	if !foundQuestion || !foundHeader || !foundSecret {
		t.Fatalf("question card fields missing: question=%v header=%v secret=%v", foundQuestion, foundHeader, foundSecret)
	}
	if !foundFooter {
		t.Fatal("card should include the Codex terminal submission instruction")
	}
	for _, want := range []string{"生产", "测试", "其他（自由输入）"} {
		matched := false
		for _, optionContent := range optionContents {
			if contains(optionContent, want) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("card option text = %#v, missing %q", optionContents, want)
		}
	}
	if len(optionContents) != 3 || !contains(optionContents[0], "面向真实用户") || !contains(optionContents[1], "仅用于验证") {
		t.Fatalf("card option text = %#v, want labels and descriptions", optionContents)
	}
}

func TestBuildCardInputRequiredWithoutQuestionsFallsBackToBody(t *testing.T) {
	sender := &FeishuSender{}
	card := sender.buildCard(Message{
		Agent: "codex",
		Event: "input_required",
		Title: "Codex 等待输入",
		Body:  "无法解析结构化问题，请在终端查看原始提示",
	})

	visibleText := cardElementText(cardBodyElements(t, card))
	for _, want := range []string{"**消息内容**", "无法解析结构化问题，请在终端查看原始提示"} {
		if !contains(visibleText, want) {
			t.Errorf("input-required fallback card text = %q, missing %q", visibleText, want)
		}
	}
}

func cardElementText(elements []any) string {
	var contents []string
	for _, raw := range elements {
		el, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if content, ok := el["content"].(string); ok {
			contents = append(contents, content)
		}
		if text, ok := el["text"].(map[string]any); ok {
			if content, ok := text["content"].(string); ok {
				contents = append(contents, content)
			}
		}
	}
	return strings.Join(contents, "\n")
}

func cardBodyElements(t *testing.T, card map[string]any) []any {
	t.Helper()
	if got := card["schema"]; got != "2.0" {
		t.Fatalf("card schema = %#v, want 2.0", got)
	}
	if _, exists := card["elements"]; exists {
		t.Fatal("Card 2.0 must not use a top-level elements field")
	}
	body, ok := card["body"].(map[string]any)
	if !ok {
		t.Fatalf("card body = %#v, want object", card["body"])
	}
	elements, ok := body["elements"].([]any)
	if !ok {
		t.Fatalf("card body elements = %#v, want slice", body["elements"])
	}
	return elements
}
