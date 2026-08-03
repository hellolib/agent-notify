package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hellolib/agent-notify/internal/feishucli"
	"github.com/hellolib/agent-notify/internal/state"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkapplication "github.com/larksuite/oapi-sdk-go/v3/service/application/v6"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type FeishuCLIConfig struct {
	AppID     string
	AppSecret string
}

type feishuConfigProvider interface {
	Parse() (FeishuCLIConfig, error)
}

type clientToolsConfigProvider struct{}

func (clientToolsConfigProvider) Parse() (FeishuCLIConfig, error) {
	cfg, err := feishucli.ParseConfig()
	if err != nil {
		return FeishuCLIConfig{}, err
	}
	return FeishuCLIConfig{
		AppID:     cfg.AppID,
		AppSecret: cfg.AppSecret,
	}, nil
}

type feishuMessenger interface {
	CreatorOpenID(ctx context.Context, appID string) (string, error)
	SendCard(ctx context.Context, receiveOpenID string, card map[string]any) error
}

type sdkFeishuMessenger struct {
	client *lark.Client
}

type FeishuSender struct {
	provider     feishuConfigProvider
	newMessenger func(appID, appSecret string) (feishuMessenger, error)
}

func NewFeishuSender(provider feishuConfigProvider) *FeishuSender {
	return &FeishuSender{
		provider:     provider,
		newMessenger: newSDKFeishuMessenger,
	}
}

func NewDefaultFeishuSender() *FeishuSender {
	return NewFeishuSender(clientToolsConfigProvider{})
}

func (s *FeishuSender) Name() string { return "feishu" }

func (s *FeishuSender) Send(ctx context.Context, msg Message) error {
	cfg, err := s.provider.Parse()
	if err != nil {
		return err
	}

	messenger, err := s.newMessenger(cfg.AppID, cfg.AppSecret)
	if err != nil {
		return err
	}

	creatorOpenID, err := messenger.CreatorOpenID(ctx, cfg.AppID)
	if err != nil {
		return err
	}

	card := s.buildCard(msg)
	return messenger.SendCard(ctx, creatorOpenID, card)
}

// buildCard creates a rich interactive card for Feishu notification
func (s *FeishuSender) buildCard(msg Message) map[string]any {
	// Event emoji mapping
	eventEmoji := map[string]string{
		"permission_required": "🔐",
		"input_required":      "⌨️",
		"run_completed":       "✅",
		"run_failed":          "❌",
	}
	emoji := eventEmoji[msg.Event]
	if emoji == "" {
		emoji = "🔔"
	}

	// Event type mapping for display
	eventType := map[string]string{
		"permission_required": "等待授权",
		"input_required":      "等待输入",
		"run_completed":       "运行完成",
		"run_failed":          "运行失败",
	}
	eventName := eventType[msg.Event]
	if eventName == "" {
		eventName = msg.Event
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	isCodex := msg.Agent == "codex"
	footerText := "🤖 Agent Notify"
	if isCodex {
		footerText = "🤖 Codex Agent Notify"
	}

	elements := []any{
		map[string]any{
			"tag": "div",
			"fields": []any{
				map[string]any{
					"is_short": true,
					"text": map[string]any{
						"tag":     "lark_md",
						"content": fmt.Sprintf("**事件类型**\n%s", eventName),
					},
				},
				map[string]any{
					"is_short": true,
					"text": map[string]any{
						"tag":     "lark_md",
						"content": fmt.Sprintf("**时间**\n%s", timestamp),
					},
				},
			},
		},
		map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": fmt.Sprintf("**消息内容**\n%s", msg.Body),
			},
		},
	}
	if msg.Workspace != "" && !isCodex {
		elements = append(elements, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": fmt.Sprintf("**工作目录**\n`%s`", msg.Workspace),
			},
		})
	}
	// 有 RequestID 时才添加详情按钮
	if msg.RequestID != "" {
		elements = append(elements, map[string]any{
			"tag": "action",
			"actions": []any{
				map[string]any{
					"tag":  "button",
					"text": map[string]any{"tag": "plain_text", "content": "📋 查看详情"},
					"type": "default",
					"value": map[string]any{
						"action":     "view_detail",
						"request_id": msg.RequestID,
					},
				},
			},
		})
	}
	if len(msg.Actions) > 0 {
		buttons := make([]any, 0, len(msg.Actions))
		for _, a := range msg.Actions {
			buttons = append(buttons, map[string]any{
				"tag": "button",
				"text": map[string]any{
					"tag":     "plain_text",
					"content": a.Label,
				},
				"type": a.Style,
				"value": map[string]any{
					"action":     a.Value,
					"request_id": msg.RequestID,
				},
			})
		}
		elements = append(elements, map[string]any{
			"tag":     "action",
			"actions": buttons,
		})
	}

	elements = append(elements,
		map[string]any{
			"tag": "hr",
		},
		map[string]any{
			"tag": "note",
			"elements": []any{
				map[string]any{
					"tag":     "plain_text",
					"content": footerText,
				},
			},
		},
	)

	return map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"header": map[string]any{
			"title": map[string]any{
				"tag": "plain_text",
				"content": func() string {
					projectName := ""
					if msg.Workspace != "" {
						parts := strings.Split(msg.Workspace, "/")
						projectName = parts[len(parts)-1]
					}
					if msg.Event == "permission_required" && msg.RequestID != "" {
						return fmt.Sprintf("%s %s | 📦 %s\n%s req: %s | session: %s", emoji, msg.Title, projectName, emoji, state.ShortID(msg.RequestID), state.ShortID(msg.SessionID))
					}
					if projectName != "" {
						return fmt.Sprintf("%s %s | 📦 %s", emoji, msg.Title, projectName)
					}
					return fmt.Sprintf("%s %s", emoji, msg.Title)
				}(),
			},
			"template": s.getHeaderColor(msg.Event),
		},
		"elements": elements,
	}
}

// getHeaderColor returns the header color based on event type
func (s *FeishuSender) getHeaderColor(event string) string {
	switch event {
	case "permission_required":
		return "orange"
	case "input_required":
		return "blue"
	case "run_completed":
		return "green"
	case "run_failed":
		return "red"
	default:
		return "turquoise"
	}
}

func newSDKFeishuMessenger(appID, appSecret string) (feishuMessenger, error) {
	if appID == "" || appSecret == "" {
		return nil, errors.New("feishu app_id or app_secret is empty")
	}
	return &sdkFeishuMessenger{client: lark.NewClient(appID, appSecret)}, nil
}

func (m *sdkFeishuMessenger) CreatorOpenID(ctx context.Context, appID string) (string, error) {
	req := larkapplication.NewGetApplicationReqBuilder().
		AppId(appID).
		Lang("zh_cn").
		UserIdType("open_id").
		Build()

	resp, err := m.client.Application.V6.Application.Get(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("feishu get application failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.App == nil || resp.Data.App.CreatorId == nil || *resp.Data.App.CreatorId == "" {
		return "", errors.New("feishu application creator open_id is empty")
	}

	return *resp.Data.App.CreatorId, nil
}

func (m *sdkFeishuMessenger) SendCard(ctx context.Context, receiveOpenID string, card map[string]any) error {
	content, err := json.Marshal(card)
	if err != nil {
		return err
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("open_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(receiveOpenID).
			MsgType("interactive").
			Content(string(content)).
			Uuid(uuid.NewString()).
			Build()).
		Build()

	resp, err := m.client.Im.V1.Message.Create(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("feishu send message failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	return nil
}
