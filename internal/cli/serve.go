package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/hellolib/agent-notify/internal/config"
	"github.com/hellolib/agent-notify/internal/feishucli"
	"github.com/hellolib/agent-notify/internal/notify"
	"github.com/hellolib/agent-notify/internal/state"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkapplication "github.com/larksuite/oapi-sdk-go/v3/service/application/v6"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"github.com/spf13/cobra"
)

func newServeCmd(ctx context.Context, streams Streams) *cobra.Command {
	var listenAddr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "启动远程审批回调服务（飞书 WebSocket 长连接）",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgPath, err := config.DefaultPath()
			if err != nil {
				return err
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			return runServe(ctx, streams, listenAddr, cfg)
		},
	}
	cmd.Flags().StringVar(&listenAddr, "listen", config.DefaultListenAddr, "本地 HTTP 回调服务监听地址（备用）")
	return cmd
}

func runServe(ctx context.Context, streams Streams, listenAddr string, cfg config.Config) error {
	if !cfg.RemoteApproval.Enabled {
		return fmt.Errorf("远程审批未启用，请在配置中设置 remote_approval.enabled: true")
	}

	statePath, err := config.StatePath()
	if err != nil {
		return err
	}

	// 启动时清理过期 pending 请求
	state.CleanExpiredPending(statePath, 10*time.Minute)

	// 获取飞书凭证
	feishuCfg, err := feishucli.ParseConfig()
	if err != nil {
		return fmt.Errorf("读取飞书配置失败: %w", err)
	}

	fmt.Fprintf(streams.Stdout, "飞书 AppID: %s\n", feishuCfg.AppID)

	// 注册卡片回调和消息接收处理器
	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2CardActionTrigger(func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
			return handleCardAction(ctx, event, statePath, streams, feishuCfg)
		}).
		OnP1MessageReceiveV1(func(ctx context.Context, event *larkim.P1MessageReceiveV1) error {
			return handleTextMessageV1(ctx, event, statePath, streams)
		}).
		OnCustomizedEvent("im.message.receive_v1", func(ctx context.Context, req *larkevent.EventReq) error {
			return handleTextMessageV2(ctx, req, statePath, streams)
		})

	// 创建 WebSocket 长连接客户端（出站连接，无需公网 IP / 端口）
	wsClient := larkws.NewClient(feishuCfg.AppID, feishuCfg.AppSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelInfo),
		larkws.WithAutoReconnect(true),
	)

	// 本地 HTTP 服务（备用 / 直接 curl 测试）
	mux := http.NewServeMux()
	mux.HandleFunc("/feishu/callback", func(w http.ResponseWriter, r *http.Request) {
		handleLocalCallback(w, r, statePath, streams)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	srv := &http.Server{Addr: listenAddr, Handler: mux}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(streams.Stderr, "本地 HTTP 服务错误: %v\n", err)
		}
	}()

	fmt.Fprintf(streams.Stdout, "远程审批服务已启动\n")
	fmt.Fprintf(streams.Stdout, "  飞书 WebSocket 长连接模式（无需公网 IP）\n")
	fmt.Fprintf(streams.Stdout, "  本地 HTTP 备用: http://%s/feishu/callback\n", listenAddr)
	fmt.Fprintf(streams.Stdout, "  正在连接飞书 WebSocket...\n")

	// 启动 WebSocket 长连接（阻塞）
	// 此连接是出站的，通过飞书代理环境变量即可访问公网
	if err := wsClient.Start(ctx); err != nil {
		return fmt.Errorf("飞书 WebSocket 连接失败: %w", err)
	}

	return nil
}

// handleTextMessageV1 处理 v1 格式的消息接收事件（HTTP webhook 备用模式）。
// 用户在聊天中直接发送文本消息（如 y/p/a/esc），映射为对最新 pending 请求的审批操作。
func handleTextMessageV1(ctx context.Context, event *larkim.P1MessageReceiveV1, statePath string, streams Streams) error {
	if event.Event == nil {
		return nil
	}

	fmt.Fprintf(streams.Stdout, "[%s] [ws] 收到 v1 消息事件: msg_type=%s\n", time.Now().Format("15:04:05"), event.Event.MsgType)

	// 只处理文本消息
	if event.Event.MsgType != "text" {
		return nil
	}

	text := strings.TrimSpace(event.Event.TextWithoutAtBot)
	if text == "" {
		text = strings.TrimSpace(event.Event.Text)
	}
	if text == "" {
		return nil
	}

	return processTextApproval(text, statePath, streams)
}

// handleCardAction 处理飞书卡片按钮点击回调（通过 WebSocket 长连接接收）。
// v2MessageEvent 飞书 v2 消息接收事件结构（WebSocket 长连接推送的格式）。
type v2MessageEvent struct {
	Schema string `json:"schema"`
	Header struct {
		EventType string `json:"event_type"`
		AppID     string `json:"app_id"`
	} `json:"header"`
	Event struct {
		Sender struct {
			SenderID struct {
				OpenID string `json:"open_id"`
			} `json:"sender_id"`
			SenderType string `json:"sender_type"`
		} `json:"sender"`
		Message struct {
			MessageID   string `json:"message_id"`
			ChatType    string `json:"chat_type"`
			MessageType string `json:"message_type"`
			Content     string `json:"content"`
		} `json:"message"`
	} `json:"event"`
}

// handleTextMessageV2 处理用户在聊天中直接发送的文本消息（如 y/p/a/esc），
// 将其映射为对最新 pending 请求的审批操作。
// WebSocket 长连接推送的是 v2 格式事件，需要手动解析 payload。
func handleTextMessageV2(ctx context.Context, req *larkevent.EventReq, statePath string, streams Streams) error {
	var event v2MessageEvent
	if err := json.Unmarshal(req.Body, &event); err != nil {
		fmt.Fprintf(streams.Stderr, "[ws] 解析消息事件失败: %v\n", err)
		return nil
	}

	fmt.Fprintf(streams.Stdout, "[%s] [ws] 收到消息事件: msg_type=%s\n", time.Now().Format("15:04:05"), event.Event.Message.MessageType)

	// 只处理文本消息
	if event.Event.Message.MessageType != "text" {
		return nil
	}

	// 解析消息内容，text 消息的 content 格式为 {"text":"p"}
	var msgContent struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(event.Event.Message.Content), &msgContent); err != nil {
		return nil
	}
	text := strings.TrimSpace(msgContent.Text)
	if text == "" {
		return nil
	}

	return processTextApproval(text, statePath, streams)
}

// processTextApproval 将文本消息映射为审批动作，处理最新的 pending 请求。
// v1 和 v2 消息处理器共用此逻辑。
func processTextApproval(text, statePath string, streams Streams) error {
	// 只有审批字符才触发审批操作，其它文本注入到终端输入框
	action := ""
	lower := strings.ToLower(text)
	switch lower {
	case "y", "yes", "allow", "允许":
		action = "allow"
	case "p":
		action = "allow_prefix"
	case "a":
		action = "allow_session"
	case "esc", "n", "no", "reject", "拒绝":
		action = "reject"
	default:
		// 非审批字符：注入到终端输入框
		return injectTextToTTY(statePath, text, streams)
	}

	status, ok := approvalStatus(action)
	if !ok {
		return nil
	}

	// 找到最新的 pending 请求
	pendingList, err := state.ListPending(statePath)
	if err != nil || len(pendingList) == 0 {
		return nil
	}

	// 按 CreatedAt 降序找最新的 pending
	var latestReq *state.PendingRequest
	for i := range pendingList {
		if pendingList[i].Status != "pending" {
			continue
		}
		if latestReq == nil || pendingList[i].CreatedAt.After(latestReq.CreatedAt) {
			latestReq = &pendingList[i]
		}
	}
	if latestReq == nil {
		fmt.Fprintf(streams.Stderr, "[ws] 收到审批消息 %q 但无 pending 请求\n", text)
		return nil
	}

	requestID := latestReq.RequestID
	if err := state.ResolvePending(statePath, requestID, status, action); err != nil {
		fmt.Fprintf(streams.Stderr, "[ws] 更新 pending 失败: %v\n", err)
		return nil
	}

	// 失效同 session 其他 pending
	if latestReq.SessionID != "" {
		state.ExpirePendingBySession(statePath, latestReq.SessionID)
	}

	fmt.Fprintf(streams.Stdout, "[%s] [msg] %q -> %s request_id=%s\n", time.Now().Format("15:04:05"), text, status, requestID)
	return nil
}

// injectTextToTTY 将非审批文本写入所有有队列子目录的 session 注入队列。
// 每个活跃 session 的 inject-daemon 只消费自己 session 的队列，
// 不活跃 session 的队列文件会被 inject-daemon 启动时清理。
// 这样即使用户回复的是旧 session 的卡片，当前活跃 session 也能收到文本。
func injectTextToTTY(statePath, text string, streams Streams) error {
	queueBase := filepath.Join(state.BaseDir(statePath), "inject_queue")

	// 找到最新 pending 请求的 session_id（会话隔离：只注入到最新请求对应的 session）
	sessionID := ""
	pendingList, err := state.ListPending(statePath)
	if err == nil {
		var latestReq *state.PendingRequest
		for i := range pendingList {
			if latestReq == nil || pendingList[i].CreatedAt.After(latestReq.CreatedAt) {
				latestReq = &pendingList[i]
			}
		}
		if latestReq != nil {
			sessionID = latestReq.SessionID
		}
	}

	if sessionID == "" {
		fmt.Fprintf(streams.Stderr, "[ws] 无法注入文本 %q：未找到 pending 请求（无 session 上下文）\n", text)
		return nil
	}

	// 写入 session 专属队列子目录
	queueDir := filepath.Join(queueBase, sessionID)
	if err := os.MkdirAll(queueDir, 0o700); err != nil {
		fmt.Fprintf(streams.Stderr, "[ws] 创建注入队列目录失败: %v\n", err)
		return nil
	}
	writeQueueFile(queueDir, text)

	fmt.Fprintf(streams.Stdout, "[%s] [msg] 文本入队 %q -> session=%s\n", time.Now().Format("15:04:05"), text, sessionID)

	// 确保该 session 的 inject-daemon 在运行
	ensureInjectDaemon(statePath, sessionID, streams)

	return nil
}

// ensureInjectDaemon 检查指定 session 是否有 inject-daemon 在运行，如果没有则启动一个。
// daemon 使用 pending 请求中的 TtyPath（具体 pts 路径）进行 TIOCSTI 注入。
func ensureInjectDaemon(statePath, sessionID string, streams Streams) {
	// 检查 pid 文件
	// pid 文件名与 inject_daemon.go 保持一致：key 为 sessionID 或 "serve"
	daemonKey := sessionID
	if daemonKey == "" {
		daemonKey = "serve"
	}
	pidFile := filepath.Join(state.BaseDir(statePath), fmt.Sprintf("inject-daemon-%s.pid", daemonKey))
	if data, err := os.ReadFile(pidFile); err == nil {
		var oldPid int
		fmt.Sscanf(string(data), "%d", &oldPid)
		if oldPid > 0 {
			// 检查进程是否存活
			if err := syscall.Kill(oldPid, 0); err == nil {
				return // daemon 已在运行
			}
		}
	}

	// 从 pending 请求中获取 TtyPath
	ttyPath := ""
	pendingList, err := state.ListPending(statePath)
	if err == nil {
		for i := range pendingList {
			if pendingList[i].SessionID == sessionID && pendingList[i].TtyPath != "" && pendingList[i].TtyPath != "/dev/tty" {
				ttyPath = pendingList[i].TtyPath
				break
			}
		}
	}

	if ttyPath == "" {
		fmt.Fprintf(streams.Stderr, "[ws] session=%s 无可用 TtyPath，跳过启动 inject-daemon\n", sessionID)
		return
	}

	// 启动 inject-daemon（setsid，因为用 TtyPath 而非 /dev/tty）
	selfExe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(streams.Stderr, "[ws] 获取可执行路径失败: %v\n", err)
		return
	}
	daemonCmd := exec.Command(selfExe, "inject-daemon",
		"--session-id", sessionID,
		"--tty", ttyPath,
		"--timeout", fmt.Sprintf("%d", config.DefaultTimeoutSec),
	)
	daemonCmd.Stdin = nil
	daemonCmd.Stdout = nil
	daemonCmd.Stderr = nil
	if err := daemonCmd.Start(); err != nil {
		fmt.Fprintf(streams.Stderr, "[ws] 启动 inject-daemon 失败: %v\n", err)
		return
	}
	if daemonCmd.Process != nil {
		_ = daemonCmd.Process.Release()
	}
	fmt.Fprintf(streams.Stdout, "[%s] [ws] 已启动 inject-daemon session=%s tty=%s pid=%d\n",
		time.Now().Format("15:04:05"), sessionID, ttyPath, daemonCmd.Process.Pid)
}

// writeQueueFile 写入一个注入队列文件（文件名用纳秒时间戳确保有序）。
func writeQueueFile(queueDir, text string) {
	filename := fmt.Sprintf("%d.txt", time.Now().UnixNano())
	queueFile := filepath.Join(queueDir, filename)
	_ = os.WriteFile(queueFile, []byte(text), 0o600)
}

func handleCardAction(ctx context.Context, event *callback.CardActionTriggerEvent, statePath string, streams Streams, feishuCfg feishucli.Config) (*callback.CardActionTriggerResponse, error) {
	if event.Event == nil || event.Event.Action == nil {
		fmt.Fprintf(streams.Stderr, "[ws] 卡片回调缺少 action 数据\n")
		return &callback.CardActionTriggerResponse{}, nil
	}

	value := event.Event.Action.Value
	action, _ := value["action"].(string)
	requestID, _ := value["request_id"].(string)

	fmt.Fprintf(streams.Stdout, "[%s] [ws] 收到卡片回调: action=%s request_id=%s\n",
		time.Now().Format("15:04:05"), action, requestID)

	if requestID == "" || action == "" {
		fmt.Fprintf(streams.Stderr, "[ws] 回调缺少 request_id 或 action\n")
		return &callback.CardActionTriggerResponse{}, nil
	}

	// 查看详情：回复原审批卡片消息（作为回复/话题），不占聊天滚动条
	if action == "view_detail" {
		req, err := state.LoadPending(statePath, requestID)
		if err != nil {
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{Type: "warning", Content: "详情不存在或已过期"},
			}, nil
		}
		// 从回调事件中获取原审批卡片的 message_id
		openMessageID := ""
		if event.Event.Context != nil {
			openMessageID = event.Event.Context.OpenMessageID
		}
		if derr := sendDetailReply(ctx, feishuCfg, req, openMessageID); derr != nil {
			fmt.Fprintf(streams.Stderr, "[ws] 发送详情回复失败: %v\n", derr)
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{Type: "warning", Content: "发送详情失败"},
			}, nil
		}
		fmt.Fprintf(streams.Stdout, "[%s] [ws] 详情回复已发送\n", time.Now().Format("15:04:05"))
		return &callback.CardActionTriggerResponse{
			Toast: &callback.Toast{Type: "success", Content: "📋 详情已回复"},
		}, nil
	}

	status, ok := approvalStatus(action)
	if !ok {
		fmt.Fprintf(streams.Stderr, "[ws] 未知 action: %s\n", action)
		return &callback.CardActionTriggerResponse{}, nil
	}
	toastContent := "✅ 已允许"
	if status == "denied" {
		toastContent = "❌ 已拒绝"
	}

	// 尝试读取原始请求信息（可能已被清理）
	origReq, _ := state.LoadPending(statePath, requestID)
	if err := state.ResolvePending(statePath, requestID, status, action); err != nil {
		fmt.Fprintf(streams.Stderr, "[ws] 更新 pending 失败: %v\n", err)
		return &callback.CardActionTriggerResponse{
			Toast: &callback.Toast{
				Type:    "warning",
				Content: "⏰ 此审批请求已过期或已被处理",
			},
			Card: expiredCard(origReq),
		}, nil
	}

	// 当前卡片按钮被消费后，失效同 session 的历史 pending 请求
	if origReq.SessionID != "" {
		if n := state.ExpirePendingBySession(statePath, origReq.SessionID); n > 0 {
			fmt.Fprintf(streams.Stdout, "[%s] [ws] 已失效 %d 个同 session 的历史请求\n", time.Now().Format("15:04:05"), n)
		}
	}

	// inject-wait 子进程会轮询 pending 文件并自动注入 TIOCSTI 按键
	fmt.Fprintf(streams.Stdout, "[%s] [ws] %s -> %s (已通知 inject-wait)\n", time.Now().Format("15:04:05"), requestID, status)

	// 返回 toast 提示并更新原卡片（移除按钮，显示已处理状态）
	resolvedCard := expiredCard(origReq)
	if resolvedCard != nil {
		resolvedTitle := "✅ 已允许"
		if status == "denied" {
			resolvedTitle = "❌ 已拒绝"
		}
		// 标题加项目名
		if origReq.Workspace != "" {
			parts := strings.Split(origReq.Workspace, "/")
			projectName := parts[len(parts)-1]
			if status == "denied" {
				resolvedTitle = fmt.Sprintf("❌ 已拒绝 | 📦 %s", projectName)
			} else {
				resolvedTitle = fmt.Sprintf("✅ 已允许 | 📦 %s", projectName)
			}
		}
		if raw, ok := resolvedCard.Data.(map[string]any); ok {
			if header, ok := raw["header"].(map[string]any); ok {
				if title, ok := header["title"].(map[string]any); ok {
					title["content"] = resolvedTitle
				}
			}
			if template, ok := raw["header"].(map[string]any); ok {
				if status == "denied" {
					template["template"] = "red"
				} else {
					template["template"] = "green"
				}
			}
		}
	}

	return &callback.CardActionTriggerResponse{
		Toast: &callback.Toast{
			Type:    "success",
			Content: toastContent,
		},
		Card: resolvedCard,
	}, nil
}

// cardCallbackPayload 飞书卡片按钮回调的 JSON 结构（本地 HTTP 备用）。
type cardCallbackPayload struct {
	Action struct {
		Value struct {
			Action    string `json:"action"`
			RequestID string `json:"request_id"`
		} `json:"value"`
	} `json:"action"`
}

// handleLocalCallback 处理本地 HTTP 回调（备用 / 直接 curl 测试）。
func handleLocalCallback(w http.ResponseWriter, r *http.Request, statePath string, streams Streams) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload cardCallbackPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	requestID := payload.Action.Value.RequestID
	action := payload.Action.Value.Action
	if requestID == "" || action == "" {
		http.Error(w, "missing request_id or action", http.StatusBadRequest)
		return
	}

	status, ok := approvalStatus(action)
	if !ok {
		http.Error(w, "unknown action: "+action, http.StatusBadRequest)
		return
	}

	if err := state.ResolvePending(statePath, requestID, status, action); err != nil {
		fmt.Fprintf(streams.Stderr, "resolve pending error: %v\n", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"code":0,"data":{"toast":{"type":"warning","content":"⏰ 此审批请求已过期或已被处理"}}}`))
		return
	}

	origReq, _ := state.LoadPending(statePath, requestID)
	if origReq.SessionID != "" {
		state.ExpirePendingBySession(statePath, origReq.SessionID)
	}
	fmt.Fprintf(streams.Stdout, "[%s] [http] %s -> %s\n", time.Now().Format("15:04:05"), requestID, status)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"code":0}`))
}

// approvalStatus 将飞书按钮 action 映射为 pending 请求状态。
// 返回 ok=false 表示未知 action。
func approvalStatus(action string) (status string, ok bool) {
	switch action {
	case "allow", "allow_prefix", "allow_session":
		return "approved", true
	case "reject":
		return "denied", true
	default:
		return "", false
	}
}

// expiredCard 返回一张移除了按钮、显示"已过期"提示的更新卡片。
// 如果能读取到原始请求信息，在卡片中关联展示工具名和命令内容。
func expiredCard(req state.PendingRequest) *callback.Card {
	desc := "此审批请求已过期或已被处理，按钮已失效。\n请在终端中直接操作，或等待下一次审批通知。"
	if req.RequestID != "" {
		desc = fmt.Sprintf("**工具**: %s\n%s\n\n⏰ 此请求已过期或已被处理，按钮已失效。", req.ToolName, req.Body)
	}

	// 构造 body elements：始终有描述文本，RequestID 非空时才添加详情按钮
	elements := []any{
		map[string]any{
			"tag": "div",
			"text": map[string]any{
				"tag":     "lark_md",
				"content": desc,
			},
		},
	}
	if req.RequestID != "" {
		elements = append(elements, map[string]any{
			"tag": "action",
			"actions": []any{
				map[string]any{
					"tag":  "button",
					"text": map[string]any{"tag": "plain_text", "content": "📋 查看详情"},
					"type": "default",
					"value": map[string]any{
						"action":     "view_detail",
						"request_id": req.RequestID,
					},
				},
			},
		})
	}
	title := "⏰ 审批请求已过期"
	if req.RequestID != "" {
		projectName := ""
		if req.Workspace != "" {
			parts := strings.Split(req.Workspace, "/")
			projectName = parts[len(parts)-1]
		}
		title = fmt.Sprintf("⏰ 已过期 | 📦 %s\nreq: %s\nsession: %s", projectName, state.ShortID(req.RequestID), state.ShortID(req.SessionID))
	}
	return &callback.Card{
		Type: "raw",
		Data: map[string]any{
			"config": map[string]any{
				"wide_screen_mode": true,
			},
			"header": map[string]any{
				"title": map[string]any{
					"tag":     "plain_text",
					"content": title,
				},
				"template": "yellow",
			},
			"elements": elements,
		},
	}
}

// sendDetailRichText 通过飞书富文本消息（post 类型）发送完整上下文到聊天。
// 原审批卡片不受影响，用户仍可在原卡片上点击允许/拒绝。
// sendDetailReply 通过回复消息 API 把详情作为原审批卡片的回复发送，
// 详情以话题/回复形式折叠在原卡片下方，不占聊天滚动条。
func sendDetailReply(ctx context.Context, feishuCfg feishucli.Config, req state.PendingRequest, messageID string) error {
	client := lark.NewClient(feishuCfg.AppID, feishuCfg.AppSecret)

	content := req.Detail
	if content == "" {
		content = fmt.Sprintf("工具: %s\n命令: %s", req.ToolName, req.Body)
	}

	postContent := map[string]any{
		"zh_cn": map[string]any{
			"title": fmt.Sprintf("📋 详情 | %s | req:%s", req.ToolName, state.ShortID(req.RequestID)),
			"content": [][]map[string]any{
				{{"tag": "text", "text": content}},
			},
		},
	}
	postJSON, err := json.Marshal(postContent)
	if err != nil {
		return fmt.Errorf("序列化富文本失败: %w", err)
	}
	postStr := string(postJSON)
	replyInThread := true

	body := larkim.NewReplyMessageReqBodyBuilder().
		Content(postStr).
		MsgType("post").
		ReplyInThread(replyInThread).
		Build()

	reqBuilder := larkim.NewReplyMessageReqBuilder().
		Body(body)

	if messageID != "" {
		reqBuilder = reqBuilder.MessageId(messageID)
	}

	resp, err := client.Im.Message.Reply(ctx, reqBuilder.Build())
	if err != nil {
		return fmt.Errorf("回复详情失败: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("回复详情失败: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

// newSDKMessenger 创建飞书 messenger 并获取 creator open id。
func newSDKMessenger(appID, appSecret string) detailMessenger {
	return &sdkDetailMessenger{client: lark.NewClient(appID, appSecret)}
}

type detailMessenger interface {
	CreatorOpenID(ctx context.Context, appID string) (string, error)
}

type sdkDetailMessenger struct {
	client *lark.Client
}

func (m *sdkDetailMessenger) CreatorOpenID(ctx context.Context, appID string) (string, error) {
	// 复用 notify 包的逻辑
	sender := notify.NewDefaultFeishuSender()
	_ = sender
	// 直接通过飞书 API 获取应用创建者
	resp, err := m.client.Application.V6.Application.Get(ctx, larkapplication.NewGetApplicationReqBuilder().
		AppId(appID).
		Lang("zh_cn").
		UserIdType("open_id").
		Build())
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("获取应用信息失败: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.App == nil || resp.Data.App.CreatorId == nil {
		return "", fmt.Errorf("创建者OpenID为空")
	}
	return *resp.Data.App.CreatorId, nil
}
