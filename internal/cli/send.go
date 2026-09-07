package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/notify"
	"github.com/spf13/cobra"
)

// newSendCmd sends a custom message through one configured notification
// channel. It intentionally bypasses agent event filtering and deduplication:
// the command is an explicit user request to send exactly one message.
func newSendCmd(ctx context.Context, streams Streams) *cobra.Command {
	var (
		channel   string
		agent     string
		title     string
		event     string
		workspace string
		message   string
	)

	cmd := &cobra.Command{
		Use:   "send [message]",
		Short: "Send a custom message through a configured channel",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			body := strings.TrimSpace(message)
			if len(args) > 0 {
				if body != "" {
					return fmt.Errorf("message must be provided either as an argument or with --message, not both")
				}
				body = strings.TrimSpace(strings.Join(args, " "))
			}
			if body == "" {
				return fmt.Errorf("message is required")
			}
			if strings.TrimSpace(channel) == "" {
				return fmt.Errorf("--channel is required")
			}

			cfg, _, err := loadDefaultConfig()
			if err != nil {
				return err
			}
			sender, err := newConfiguredSender(cfg, agent, channel)
			if err != nil {
				return err
			}

			eventName := strings.TrimSpace(event)
			if eventName == "" {
				eventName = "manual"
			}
			message := notify.Message{
				Agent:     normalizeSendAgent(agent),
				Event:     eventName,
				Title:     strings.TrimSpace(title),
				Body:      body,
				Workspace: strings.TrimSpace(workspace),
			}
			if message.Title == "" {
				message.Title = "Agent Notify"
			}

			timeout := time.Duration(cfg.Behavior.SendTimeoutSeconds) * time.Second
			if timeout <= 0 {
				timeout = 5 * time.Second
			}
			sendCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			if err := sender.Send(sendCtx, message); err != nil {
				return err
			}
			_, err = fmt.Fprintf(streams.Stdout, "✅ sent via %s\n", sender.Name())
			return err
		},
	}

	cmd.Flags().StringVarP(&channel, "channel", "c", "", "channel: system, feishu, wechat, wechat-work, dingtalk, bark, ntfy, slack")
	cmd.Flags().StringVarP(&agent, "agent", "a", "omp", "agent configuration: claude, codex, zcode, grok, droid, opencode, omp")
	cmd.Flags().StringVar(&title, "title", "Agent Notify", "message title")
	cmd.Flags().StringVar(&event, "event", "manual", "event style: manual, permission_required, input_required, run_completed, run_failed")
	cmd.Flags().StringVar(&workspace, "workspace", "", "workspace shown in the message")
	cmd.Flags().StringVarP(&message, "message", "m", "", "message body")
	return cmd
}

func newConfiguredSender(cfg config.Config, agent, channel string) (notify.Sender, error) {
	notifyCfg, err := notifyConfigForAgent(cfg, agent)
	if err != nil {
		return nil, err
	}

	switch normalizeSendChannel(channel) {
	case "system":
		return notify.NewSystemSender(
			notify.DefaultRunner,
			notifyCfg.Channels.System.ClickToFocus,
			config.FocusPrecisionFromEnv(),
			notifyCfg.Channels.System.EffectiveFocusDebug(),
		), nil
	case "feishu":
		return notify.NewDefaultFeishuSender(), nil
	case "wechat":
		if strings.TrimSpace(notifyCfg.Channels.Wechat.WebhookURL) == "" {
			return nil, fmt.Errorf("wechat webhook is not configured for agent %s", normalizeSendAgent(agent))
		}
		return notify.NewWechatSender(notifyCfg.Channels.Wechat.WebhookURL), nil
	case "wechat-work":
		if strings.TrimSpace(notifyCfg.Channels.WechatWork.WebhookURL) == "" {
			return nil, fmt.Errorf("wechat-work webhook is not configured for agent %s", normalizeSendAgent(agent))
		}
		return notify.NewWechatWorkSender(notifyCfg.Channels.WechatWork.WebhookURL), nil
	case "dingtalk":
		if strings.TrimSpace(notifyCfg.Channels.DingTalk.WebhookURL) == "" {
			return nil, fmt.Errorf("dingtalk webhook is not configured for agent %s", normalizeSendAgent(agent))
		}
		return notify.NewDingTalkSender(notifyCfg.Channels.DingTalk.WebhookURL), nil
	case "bark":
		if strings.TrimSpace(notifyCfg.Channels.Bark.WebhookURL) == "" {
			return nil, fmt.Errorf("bark webhook is not configured for agent %s", normalizeSendAgent(agent))
		}
		return notify.NewBarkSender(notifyCfg.Channels.Bark.WebhookURL), nil
	case "ntfy":
		if strings.TrimSpace(notifyCfg.Channels.Ntfy.TopicURL) == "" {
			return nil, fmt.Errorf("ntfy topic is not configured for agent %s", normalizeSendAgent(agent))
		}
		return notify.NewNtfySender(notifyCfg.Channels.Ntfy.TopicURL), nil
	case "slack":
		if strings.TrimSpace(notifyCfg.Channels.Slack.WebhookURL) == "" {
			return nil, fmt.Errorf("slack webhook is not configured for agent %s", normalizeSendAgent(agent))
		}
		return notify.NewSlackSender(notifyCfg.Channels.Slack.WebhookURL), nil
	default:
		return nil, fmt.Errorf("unsupported channel %q; choose system, feishu, wechat, wechat-work, dingtalk, bark, ntfy, or slack", channel)
	}
}

func notifyConfigForAgent(cfg config.Config, agent string) (config.AgentNotifyConfig, error) {
	switch normalizeSendAgent(agent) {
	case "claude":
		return cfg.Notify.ClaudeCode, nil
	case "codex":
		return cfg.Notify.Codex, nil
	case "zcode":
		return cfg.Notify.ZCode, nil
	case "grok":
		return cfg.Notify.Grok, nil
	case "droid":
		return cfg.Notify.Droid, nil
	case "opencode":
		return cfg.Notify.OpenCode, nil
	case "omp":
		return cfg.Notify.OMP, nil
	default:
		return config.AgentNotifyConfig{}, fmt.Errorf("unsupported agent %q; choose claude, codex, zcode, grok, droid, opencode, or omp", agent)
	}
}

func normalizeSendAgent(agent string) string {
	value := strings.ToLower(strings.TrimSpace(agent))
	value = strings.ReplaceAll(value, "_", "-")
	value = strings.ReplaceAll(value, " ", "-")
	switch value {
	case "claude-code":
		return "claude"
	case "open-code":
		return "opencode"
	case "oh-my-pi", "pi":
		return "omp"
	default:
		return value
	}
}

func normalizeSendChannel(channel string) string {
	value := strings.ToLower(strings.TrimSpace(channel))
	value = strings.ReplaceAll(value, "_", "-")
	switch value {
	case "wechatwork", "wecom", "wxwork":
		return "wechat-work"
	case "dingtalk":
		return "dingtalk"
	default:
		return value
	}
}
