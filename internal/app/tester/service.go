// Package tester provides the test notification service for agent-notify.
// It handles sending test notifications through various channels.
package tester

import (
	"context"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/i18n"
	"github.com/hellolib/agent-notify/internal/notify"
)

// FeishuPreparer prepares the Feishu CLI for use.
type FeishuPreparer interface {
	EnsureReady(ctx context.Context) error
}

// ConfigLoader loads configuration.
type ConfigLoader interface {
	Load(path string) (config.Config, error)
	DefaultPath() (string, error)
}

// Service handles test notifications.
type Service struct {
	feishuPreparer   FeishuPreparer
	configLoader     ConfigLoader
	feishuSender     notify.Sender
	systemSender     notify.Sender
	wechatSender     notify.Sender
	wechatWorkSender notify.Sender
	dingTalkSender   notify.Sender
	barkSender       notify.Sender
	ntfySender       notify.Sender
	slackSender      notify.Sender
}

// NewService creates a new tester service.
func NewService(opts ...Option) *Service {
	s := &Service{}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Option configures the service.
type Option func(*Service)

// WithFeishuPreparer sets the Feishu preparer.
func WithFeishuPreparer(p FeishuPreparer) Option {
	return func(s *Service) { s.feishuPreparer = p }
}

// WithConfigLoader sets the config loader.
func WithConfigLoader(l ConfigLoader) Option {
	return func(s *Service) { s.configLoader = l }
}

// WithFeishuSender sets the Feishu sender.
func WithFeishuSender(sender notify.Sender) Option {
	return func(s *Service) { s.feishuSender = sender }
}

// WithSystemSender sets the system sender.
func WithSystemSender(sender notify.Sender) Option {
	return func(s *Service) { s.systemSender = sender }
}

// WithWechatSender sets the personal WeChat webhook sender.
func WithWechatSender(sender notify.Sender) Option {
	return func(s *Service) { s.wechatSender = sender }
}

// WithWechatWorkSender sets the WeChat Work sender.
func WithWechatWorkSender(sender notify.Sender) Option {
	return func(s *Service) { s.wechatWorkSender = sender }
}

// WithDingTalkSender sets the DingTalk sender.
func WithDingTalkSender(sender notify.Sender) Option {
	return func(s *Service) { s.dingTalkSender = sender }
}

// WithBarkSender sets the Bark sender.
func WithBarkSender(sender notify.Sender) Option {
	return func(s *Service) { s.barkSender = sender }
}

// WithNtfySender sets the Ntfy sender.
func WithNtfySender(sender notify.Sender) Option {
	return func(s *Service) { s.ntfySender = sender }
}

// WithSlackSender sets the Slack sender.
func WithSlackSender(sender notify.Sender) Option {
	return func(s *Service) { s.slackSender = sender }
}

// TestFeishuResult contains the result of a Feishu test.
type TestFeishuResult struct {
	Message string
}

// TestFeishu sends a test Feishu notification.
func (s *Service) TestFeishu(ctx context.Context) (*TestFeishuResult, error) {
	// Note: Test notification intentionally ignores the enabled flag in config.
	// This allows users to verify Feishu connectivity before enabling it permanently.
	if s.feishuPreparer != nil {
		if err := s.feishuPreparer.EnsureReady(ctx); err != nil {
			return nil, err
		}
	}
	msg := notify.Message{Event: "permission_required", Title: i18n.T("test.msg_title"), Body: i18n.T("test.msg_body")}
	if err := s.feishuNotificationSender().Send(ctx, msg); err != nil {
		return nil, err
	}
	return &TestFeishuResult{Message: i18n.T("test.feishu_sent")}, nil
}

// TestSystemResult contains the result of a system test.
type TestSystemResult struct {
	Message string
}

// TestSystem sends a test system notification.
func (s *Service) TestSystem(ctx context.Context) (*TestSystemResult, error) {
	msg := notify.Message{Event: "permission_required", Title: i18n.T("test.msg_title"), Body: i18n.T("test.msg_body"), SourceApp: notify.DetectSourceApp()}
	if err := s.systemNotificationSender().Send(ctx, msg); err != nil {
		return nil, err
	}
	return &TestSystemResult{Message: i18n.T("test.system_sent")}, nil
}

// TestWechatResult contains the result of a personal WeChat webhook test.
type TestWechatResult struct {
	Message string
}

// TestWechat sends a test WeChat notification using the provided webhook URL.
func (s *Service) TestWechat(ctx context.Context, webhookURL string) (*TestWechatResult, error) {
	msg := notify.Message{Event: "permission_required", Title: i18n.T("test.msg_title"), Body: i18n.T("test.msg_body_wechat_personal")}
	if err := s.wechatNotificationSender(webhookURL).Send(ctx, msg); err != nil {
		return nil, err
	}
	return &TestWechatResult{Message: i18n.T("test.wechat_personal_sent")}, nil
}

// TestWechatWorkResult contains the result of a WeChat Work test.
type TestWechatWorkResult struct {
	Message string
}

// TestWechatWork sends a test WeChat Work notification using the provided webhook URL.
func (s *Service) TestWechatWork(ctx context.Context, webhookURL string) (*TestWechatWorkResult, error) {
	msg := notify.Message{Event: "permission_required", Title: i18n.T("test.msg_title"), Body: i18n.T("test.msg_body_wechat")}
	if err := s.wechatWorkNotificationSender(webhookURL).Send(ctx, msg); err != nil {
		return nil, err
	}
	return &TestWechatWorkResult{Message: i18n.T("test.wechat_sent")}, nil
}

// TestDingTalkResult contains the result of a DingTalk test.
type TestDingTalkResult struct {
	Message string
}

// TestDingTalk sends a test DingTalk notification using the provided webhook URL.
func (s *Service) TestDingTalk(ctx context.Context, webhookURL string) (*TestDingTalkResult, error) {
	msg := notify.Message{Event: "permission_required", Title: i18n.T("test.msg_title"), Body: i18n.T("test.msg_body_dingtalk")}
	if err := s.dingTalkNotificationSender(webhookURL).Send(ctx, msg); err != nil {
		return nil, err
	}
	return &TestDingTalkResult{Message: i18n.T("test.dingtalk_sent")}, nil
}

// TestBarkResult contains the result of a Bark test.
type TestBarkResult struct {
	Message string
}

// TestBark sends a test Bark notification using the provided webhook URL.
func (s *Service) TestBark(ctx context.Context, webhookURL string) (*TestBarkResult, error) {
	msg := notify.Message{Event: "permission_required", Title: i18n.T("test.msg_title"), Body: i18n.T("test.msg_body_bark")}
	if err := s.barkNotificationSender(webhookURL).Send(ctx, msg); err != nil {
		return nil, err
	}
	return &TestBarkResult{Message: i18n.T("test.bark_sent")}, nil
}

// TestNtfyResult contains the result of a Ntfy test.
type TestNtfyResult struct {
	Message string
}

// TestNtfy sends a test Ntfy notification using the provided topic URL.
func (s *Service) TestNtfy(ctx context.Context, topicURL string) (*TestNtfyResult, error) {
	msg := notify.Message{Event: "permission_required", Title: i18n.T("test.msg_title"), Body: i18n.T("test.msg_body_ntfy")}
	if err := s.ntfyNotificationSender(topicURL).Send(ctx, msg); err != nil {
		return nil, err
	}
	return &TestNtfyResult{Message: i18n.T("test.ntfy_sent")}, nil
}

// TestSlackResult contains the result of a Slack test.
type TestSlackResult struct {
	Message string
}

// TestSlack sends a test Slack notification using the provided webhook URL.
func (s *Service) TestSlack(ctx context.Context, webhookURL string) (*TestSlackResult, error) {
	msg := notify.Message{Event: "permission_required", Title: i18n.T("test.msg_title"), Body: i18n.T("test.msg_body_slack")}
	if err := s.slackNotificationSender(webhookURL).Send(ctx, msg); err != nil {
		return nil, err
	}
	return &TestSlackResult{Message: i18n.T("test.slack_sent")}, nil
}

func (s *Service) defaultConfigPath() (string, error) {
	if s.configLoader != nil {
		return s.configLoader.DefaultPath()
	}
	return config.DefaultPath()
}

func (s *Service) loadConfig(path string) (config.Config, error) {
	if s.configLoader != nil {
		return s.configLoader.Load(path)
	}
	return config.Load(path)
}

func (s *Service) feishuNotificationSender() notify.Sender {
	if s.feishuSender != nil {
		return s.feishuSender
	}
	return notify.NewDefaultFeishuSender()
}

func (s *Service) systemNotificationSender() notify.Sender {
	if s.systemSender != nil {
		return s.systemSender
	}
	return notify.NewSystemSender(notify.DefaultRunner, true)
}

func (s *Service) wechatNotificationSender(webhookURL string) notify.Sender {
	if s.wechatSender != nil {
		return s.wechatSender
	}
	return notify.NewWechatSender(webhookURL)
}

func (s *Service) wechatWorkNotificationSender(webhookURL string) notify.Sender {
	if s.wechatWorkSender != nil {
		return s.wechatWorkSender
	}
	return notify.NewWechatWorkSender(webhookURL)
}

func (s *Service) dingTalkNotificationSender(webhookURL string) notify.Sender {
	if s.dingTalkSender != nil {
		return s.dingTalkSender
	}
	return notify.NewDingTalkSender(webhookURL)
}

func (s *Service) barkNotificationSender(webhookURL string) notify.Sender {
	if s.barkSender != nil {
		return s.barkSender
	}
	return notify.NewBarkSender(webhookURL)
}

func (s *Service) ntfyNotificationSender(topicURL string) notify.Sender {
	if s.ntfySender != nil {
		return s.ntfySender
	}
	return notify.NewNtfySender(topicURL)
}

func (s *Service) slackNotificationSender(webhookURL string) notify.Sender {
	if s.slackSender != nil {
		return s.slackSender
	}
	return notify.NewSlackSender(webhookURL)
}
