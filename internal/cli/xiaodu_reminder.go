package cli

import (
	"context"
	"fmt"

	"github.com/hellolib/agent-notify/internal/agenthooks"
	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/notify"
	"github.com/hellolib/agent-notify/internal/state"
	"github.com/spf13/cobra"
)

func newXiaoduReminderCmd(ctx context.Context, streams Streams) *cobra.Command {
	var key string
	cmd := &cobra.Command{
		Use:    "xiaodu-reminder",
		Short:  "Internal Xiaodu reminder worker",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := config.DefaultPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			reminderPath, err := config.XiaoduReminderPath()
			if err != nil {
				return err
			}
			logPath, err := config.LogPath()
			if err != nil {
				return err
			}

			store := notify.NewXiaoduReminderStore(reminderPath)
			reminder, ok, err := store.Get(key)
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}

			xiaoduCfg := xiaoduConfigForAgent(cfg, reminder.Agent)
			sender := notify.NewXiaoduSenderWithOptions(notify.XiaoduSenderOptions{
				APIBaseURL:     xiaoduCfg.APIBaseURL,
				AccessToken:    xiaoduCfg.AccessToken,
				RefreshToken:   xiaoduCfg.RefreshToken,
				ClientID:       xiaoduCfg.ClientID,
				ClientSecret:   xiaoduCfg.ClientSecret,
				ExpiresAt:      xiaoduCfg.ExpiresAt,
				DeviceID:       xiaoduCfg.DeviceID,
				CUID:           xiaoduCfg.CUID,
				SpeakCompleted: xiaoduCfg.SpeakCompleted,
				RepeatCount:    1,
				OnRefresh:      agenthooks.XiaoduRefreshSaver(reminder.Agent, "", xiaoduCfg.RefreshToken),
			})
			err = notify.RunXiaoduReminderWorker(ctx, notify.XiaoduReminderWorker{
				Store: store,
				Key:   key,
				SendText: func(ctx context.Context, reminder notify.XiaoduReminder) error {
					return sender.SendReminderText(ctx, reminder)
				},
			})
			if err != nil {
				_ = state.AppendLog(logPath, fmt.Sprintf("xiaodu reminder worker error key=%s err=%v", key, err))
			}
			return err
		},
	}
	cmd.Flags().StringVar(&key, "key", "", "reminder key")
	_ = cmd.MarkFlagRequired("key")
	return cmd
}

func xiaoduConfigForAgent(cfg config.Config, agent string) config.XiaoduChannelConfig {
	switch agent {
	case "codex":
		return cfg.Notify.Codex.Channels.Xiaodu
	case "zcode":
		return cfg.Notify.ZCode.Channels.Xiaodu
	default:
		return cfg.Notify.ClaudeCode.Channels.Xiaodu
	}
}
