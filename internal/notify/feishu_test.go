package notify

import (
	"context"
	"errors"
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

	// 验证 card 结构
	elements, ok := card["elements"].([]any)
	if !ok {
		t.Fatal("card elements should be a slice")
	}

	// 查找包含 Body 的元素
	found := false
	for _, el := range elements {
		if elMap, ok := el.(map[string]any); ok {
			if text, ok := elMap["text"].(map[string]any); ok {
				if content, ok := text["content"].(string); ok {
					if contains(content, "这是测试消息内容") {
						found = true
						break
					}
				}
			}
		}
	}

	if !found {
		t.Error("card should contain message body content")
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

	elements, ok := card["elements"].([]any)
	if !ok {
		t.Fatal("card elements should be a slice")
	}

	foundClaudeCode := false
	for _, el := range elements {
		elMap, ok := el.(map[string]any)
		if !ok || elMap["tag"] != "note" {
			continue
		}
		noteElements, ok := elMap["elements"].([]any)
		if !ok {
			continue
		}
		for _, noteEl := range noteElements {
			noteMap, ok := noteEl.(map[string]any)
			if !ok {
				continue
			}
			content, _ := noteMap["content"].(string)
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

	elements, ok := card["elements"].([]any)
	if !ok {
		t.Fatal("card elements should be a slice")
	}

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

func TestBuildCardRendersQuestionsAndInertOptions(t *testing.T) {
	sender := &FeishuSender{}
	card := sender.buildCard(Message{
		Agent: "codex",
		Event: "input_required",
		Title: "Codex 等待输入",
		Body:  "请选择配置",
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

	elements, ok := card["elements"].([]any)
	if !ok {
		t.Fatal("card elements should be a slice")
	}

	var foundQuestion, foundHeader, foundSecret, foundOther bool
	var labels, descriptions []string
	for _, raw := range elements {
		el, ok := raw.(map[string]any)
		if !ok {
			continue
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
		if el["tag"] == "button" {
			labels = append(labels, content)
			if _, exists := el["action"]; exists {
				t.Fatalf("visual option button must not contain action: %#v", el)
			}
			if _, exists := el["value"]; exists {
				t.Fatalf("visual option button must not contain value: %#v", el)
			}
			if _, exists := el["behaviors"]; exists {
				t.Fatalf("visual option button must not contain behaviors: %#v", el)
			}
		}
		if el["tag"] == "note" {
			noteElements, _ := el["elements"].([]any)
			for _, noteRaw := range noteElements {
				note, _ := noteRaw.(map[string]any)
				noteContent, _ := note["content"].(string)
				if contains(noteContent, "请回到 Codex 终端提交") {
					foundOther = true // instruction is present; checked below with option labels
				}
			}
		}
		if el["tag"] == "div" && contains(content, "面向真实用户") {
			descriptions = append(descriptions, content)
		}
	}

	if !foundQuestion || !foundHeader || !foundSecret {
		t.Fatalf("question card fields missing: question=%v header=%v secret=%v", foundQuestion, foundHeader, foundSecret)
	}
	if !foundOther {
		t.Fatal("card should include the Codex terminal submission instruction")
	}
	for _, want := range []string{"生产", "测试", "其他（自由输入）"} {
		matched := false
		for _, label := range labels {
			if contains(label, want) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("card button labels = %#v, missing %q", labels, want)
		}
	}
	if len(descriptions) != 1 || !contains(descriptions[0], "面向真实用户") {
		t.Fatalf("card option descriptions = %#v, want production description", descriptions)
	}
}
