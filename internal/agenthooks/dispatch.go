package agenthooks

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/notify"
	"github.com/hellolib/agent-notify/internal/state"
)

func Dispatch(ctx context.Context, cfg config.Config, statePath, logPath string, msg notify.Message) error {
	// hook 进程由终端 / IDE 启动，此处能从继承的环境变量识别宿主应用
	msg.SourceApp = notify.DetectSourceApp()

	store := state.NewStore(statePath)
	senders := buildSenders(cfg, msg, statePath)
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

func buildSenders(cfg config.Config, msg notify.Message, statePaths ...string) []notify.Sender {
	var senders []notify.Sender

	notifyCfg := cfg.Notify.ClaudeCode
	if msg.Agent == "codex" {
		notifyCfg = cfg.Notify.Codex
	} else if msg.Agent == "zcode" {
		notifyCfg = cfg.Notify.ZCode
	}

	if !contains(notifyCfg.Events, msg.Event) {
		return senders
	}

	if notifyCfg.Channels.System.Enabled {
		senders = append(senders, notify.NewSystemSender(notify.DefaultRunner, notifyCfg.Channels.System.ClickToFocus))
	}
	if notifyCfg.Channels.Feishu.Enabled {
		senders = append(senders, notify.NewDefaultFeishuSender())
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
	if notifyCfg.Channels.Xiaodu.Enabled && notifyCfg.Channels.Xiaodu.AccessToken != "" && notifyCfg.Channels.Xiaodu.APIBaseURL != "" {
		statePath := ""
		if len(statePaths) > 0 {
			statePath = statePaths[0]
		}
		senders = append(senders, notify.NewXiaoduSenderWithOAuth(
			notifyCfg.Channels.Xiaodu.APIBaseURL,
			notifyCfg.Channels.Xiaodu.AccessToken,
			notifyCfg.Channels.Xiaodu.RefreshToken,
			notifyCfg.Channels.Xiaodu.ClientID,
			notifyCfg.Channels.Xiaodu.ClientSecret,
			notifyCfg.Channels.Xiaodu.ExpiresAt,
			notifyCfg.Channels.Xiaodu.DeviceID,
			notifyCfg.Channels.Xiaodu.CUID,
			xiaoduRefreshSaver(msg.Agent, statePath, notifyCfg.Channels.Xiaodu.RefreshToken),
		))
	}

	return senders
}

func xiaoduRefreshSaver(agent, statePath, previousRefreshToken string) func(notify.XiaoduTokenState) error {
	return func(token notify.XiaoduTokenState) error {
		configPath, err := xiaoduConfigPath(statePath)
		if err != nil {
			return err
		}
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}

		updateSharedXiaoduToken(&cfg, agent, previousRefreshToken, token)

		return config.Save(configPath, cfg)
	}
}

func updateSharedXiaoduToken(cfg *config.Config, agent, previousRefreshToken string, token notify.XiaoduTokenState) {
	update := func(channel *config.XiaoduChannelConfig) {
		channel.AccessToken = token.AccessToken
		channel.RefreshToken = token.RefreshToken
		channel.ExpiresAt = token.ExpiresAt
	}

	switch agent {
	case "codex":
		update(&cfg.Notify.Codex.Channels.Xiaodu)
	case "zcode":
		update(&cfg.Notify.ZCode.Channels.Xiaodu)
	default:
		update(&cfg.Notify.ClaudeCode.Channels.Xiaodu)
	}

	if previousRefreshToken == "" {
		return
	}
	for _, channel := range []*config.XiaoduChannelConfig{
		&cfg.Notify.ClaudeCode.Channels.Xiaodu,
		&cfg.Notify.Codex.Channels.Xiaodu,
		&cfg.Notify.ZCode.Channels.Xiaodu,
	} {
		if channel.Enabled && channel.RefreshToken == previousRefreshToken {
			update(channel)
		}
	}
}

func xiaoduConfigPath(statePath string) (string, error) {
	if statePath != "" {
		return filepath.Join(filepath.Dir(statePath), "config.yaml"), nil
	}
	return config.DefaultPath()
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
