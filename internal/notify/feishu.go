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
	// request_user_input carries structured questions in addition to the
	// textual body.  Render those questions explicitly so a remote recipient
	// can see the same choices as the Codex terminal.  The button elements are
	// deliberately inert (there is no action/value/behaviors field): Codex
	// hooks cannot accept a UserInputAnswer yet, so the recipient must answer
	// in the terminal.
	if len(msg.Questions) > 0 {
		elements = appendQuestionElements(elements, msg.Questions)
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
				"tag":     "plain_text",
				"content": fmt.Sprintf("%s %s", emoji, msg.Title),
			},
			"template": s.getHeaderColor(msg.Event),
		},
		"elements": elements,
	}
}

// appendQuestionElements appends a visual, non-interactive rendering of an
// input-required prompt.  Keeping options as ordinary button elements (and
// not putting them in an action group) is intentional: an Feishu card must
// not imply that clicking a choice submits an answer when the hook has no
// callback path.
func appendQuestionElements(elements []any, questions []Question) []any {
	elements = append(elements, map[string]any{"tag": "hr"})

	for index, question := range questions {
		heading := fmt.Sprintf("**问题 %d**", index+1)
		if question.Header != "" {
			heading += " · " + question.Header
		}
		if question.IsSecret {
			heading += " 🔒 敏感输入"
		}

		questionText := strings.TrimSpace(question.Question)
		if questionText == "" {
			questionText = "（未提供问题文本）"
		}
		elements = append(elements, map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": heading + "\n" + questionText,
			},
		})

		for _, option := range question.Options {
			label := option.Label
			if strings.TrimSpace(label) == "" {
				label = "未命名选项"
			}
			// A button without action/value/behaviors is purely visual.  Keep
			// the label in plain text so Feishu does not interpret user input
			// as markdown.
			elements = append(elements, map[string]any{
				"tag": "button",
				"text": map[string]any{
					"tag":     "plain_text",
					"content": label,
				},
				"type": "default",
			})
			if description := strings.TrimSpace(option.Description); description != "" {
				elements = append(elements, map[string]any{
					"tag": "div",
					"text": map[string]any{
						"tag":     "lark_md",
						"content": "　" + description,
					},
				})
			}
		}

		if question.IsOther {
			elements = append(elements, map[string]any{
				"tag": "button",
				"text": map[string]any{
					"tag":     "plain_text",
					"content": "其他（自由输入）",
				},
				"type": "default",
			})
		}
	}

	elements = append(elements, map[string]any{
		"tag": "note",
		"elements": []any{
			map[string]any{
				"tag":     "plain_text",
				"content": "请回到 Codex 终端提交；当前版本不支持卡片回传",
			},
		},
	})

	return elements
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
