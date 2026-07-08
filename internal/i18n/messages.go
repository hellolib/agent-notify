package i18n

var catalog = map[string]map[Lang]string{
	// ── Main menu ──────────────────────────────────────────────
	"menu.agent_config":   {ZhCN: "Agent通知配置", EnUS: "Agent Setup"},
	"menu.channel_config": {ZhCN: "消息渠道配置", EnUS: "Channel Config"},
	"menu.test":           {ZhCN: "测试通知", EnUS: "Test Notification"},
	"menu.doctor":         {ZhCN: "环境诊断", EnUS: "Diagnostics"},
	"menu.view_config":    {ZhCN: "查看配置", EnUS: "View Config"},
	"menu.clean_config":   {ZhCN: "清理配置", EnUS: "Reset Config"},
	"menu.language":       {ZhCN: "语言[Language]", EnUS: "Language[语言]"},
	"menu.quit":           {ZhCN: "退出", EnUS: "Quit"},

	// ── Test sub-menu ──────────────────────────────────────────
	"test.title":    {ZhCN: "测试通知", EnUS: "Test Notification"},
	"test.system":   {ZhCN: "系统通知", EnUS: "System"},
	"test.feishu":   {ZhCN: "飞书", EnUS: "Feishu"},
	"test.wechat":   {ZhCN: "企业微信", EnUS: "WeChat Work"},
	"test.dingtalk": {ZhCN: "钉钉", EnUS: "DingTalk"},
	"test.bark":     {ZhCN: "Bark", EnUS: "Bark"},
	"test.ntfy":     {ZhCN: "Ntfy", EnUS: "Ntfy"},
	"test.slack":    {ZhCN: "Slack", EnUS: "Slack"},
	"test.xiaodu":   {ZhCN: "小度音箱", EnUS: "Xiaodu"},
	"test.back":     {ZhCN: "返回", EnUS: "Back"},

	// ── Channel sub-menu ───────────────────────────────────────
	"channel.title":    {ZhCN: "消息渠道配置", EnUS: "Channel Config"},
	"channel.feishu":   {ZhCN: "飞书", EnUS: "Feishu"},
	"channel.wechat":   {ZhCN: "企业微信", EnUS: "WeChat Work"},
	"channel.dingtalk": {ZhCN: "钉钉", EnUS: "DingTalk"},
	"channel.bark":     {ZhCN: "Bark", EnUS: "Bark"},
	"channel.ntfy":     {ZhCN: "Ntfy", EnUS: "Ntfy"},
	"channel.slack":    {ZhCN: "Slack", EnUS: "Slack"},
	"channel.xiaodu":   {ZhCN: "小度音箱", EnUS: "Xiaodu"},
	"channel.back":     {ZhCN: "返回", EnUS: "Back"},

	// ── Setup flow prompts ─────────────────────────────────────
	"setup.select_agent":    {ZhCN: "选择要配置的 Agent", EnUS: "Select an agent to configure"},
	"setup.select_channels": {ZhCN: "启用通知渠道", EnUS: "Enable notification channels"},
	"setup.select_events":   {ZhCN: "通知事件", EnUS: "Notification events"},

	// Channel option labels (used in multi-select during setup)
	"channel.system": {ZhCN: "系统通知", EnUS: "System"},

	// Event option labels
	"event.permission_required": {ZhCN: "需要授权 (permission_required)", EnUS: "Permission Required"},
	"event.input_required":      {ZhCN: "等待输入 (input_required)", EnUS: "Input Required"},
	"event.run_completed":       {ZhCN: "任务完成 (run_completed)", EnUS: "Task Completed"},
	"event.run_failed":          {ZhCN: "任务失败 (run_failed)", EnUS: "Task Failed"},
	"event.session_start":       {ZhCN: "会话开始 (session_start)", EnUS: "Session Start"},

	// ── Webhook URL prompts ────────────────────────────────────
	"prompt.wechat_webhook":                 {ZhCN: "企业微信群机器人 Webhook URL", EnUS: "WeChat Work Bot Webhook URL"},
	"prompt.dingtalk_webhook":               {ZhCN: "钉钉群机器人 Webhook URL", EnUS: "DingTalk Bot Webhook URL"},
	"prompt.bark_webhook":                   {ZhCN: "Bark Webhook URL", EnUS: "Bark Webhook URL"},
	"prompt.ntfy_topic_url":                 {ZhCN: "Ntfy Topic URL", EnUS: "Ntfy Topic URL"},
	"prompt.slack_webhook":                  {ZhCN: "Slack Incoming Webhook URL", EnUS: "Slack Incoming Webhook URL"},
	"prompt.xiaodu_api_base_url":            {ZhCN: "小度 MCP/JSON-RPC API 地址", EnUS: "Xiaodu MCP/JSON-RPC API URL"},
	"prompt.xiaodu_access_token":            {ZhCN: "小度 Access Token", EnUS: "Xiaodu Access Token"},
	"prompt.xiaodu_refresh_token":           {ZhCN: "小度 Refresh Token（可留空）", EnUS: "Xiaodu Refresh Token (optional)"},
	"prompt.xiaodu_client_id":               {ZhCN: "小度 Client ID / API Key（自动刷新需要）", EnUS: "Xiaodu Client ID / API Key (required for auto-refresh)"},
	"prompt.xiaodu_client_secret":           {ZhCN: "小度 Client Secret / Secret Key（自动刷新需要）", EnUS: "Xiaodu Client Secret / Secret Key (required for auto-refresh)"},
	"prompt.xiaodu_expires_at":              {ZhCN: "小度 Access Token 过期时间 Unix 秒（留空禁用自动刷新）", EnUS: "Xiaodu Access Token expires_at Unix seconds (empty disables auto-refresh)"},
	"prompt.xiaodu_device_id":               {ZhCN: "小度设备 client_id（留空自动选择）", EnUS: "Xiaodu device client_id (empty for auto-select)"},
	"prompt.xiaodu_cuid":                    {ZhCN: "小度设备 CUID（留空自动选择）", EnUS: "Xiaodu device CUID (empty for auto-select)"},
	"prompt.xiaodu_speak_completed":         {ZhCN: "小度是否播报任务完成", EnUS: "Speak completed tasks on Xiaodu"},
	"prompt.xiaodu_repeat_count":            {ZhCN: "小度需要操作事件总播报次数（含首次）", EnUS: "Xiaodu repeat count for action-needed events (including first speech)"},
	"prompt.xiaodu_repeat_interval_seconds": {ZhCN: "小度重复播报间隔秒数", EnUS: "Xiaodu repeat interval seconds"},

	// ── Survey help text ───────────────────────────────────────
	"prompt.help.multiselect": {ZhCN: "[↑↓ 移动, 空格 选择/取消, Enter 确认] ", EnUS: "[↑↓ navigate, space toggle, enter confirm] "},

	// ── Success / info messages ────────────────────────────────
	"msg.config_done":     {ZhCN: "✅ 配置完成", EnUS: "✅ Configuration complete"},
	"msg.feishu_cli_done": {ZhCN: "✅ 飞书 CLI 初始化完成", EnUS: "✅ Feishu CLI initialized"},
	"msg.config_file":     {ZhCN: "配置文件: %s", EnUS: "Config file: %s"},
	"msg.test_sent":       {ZhCN: "✅ %s", EnUS: "✅ %s"},

	// ── Error messages ─────────────────────────────────────────
	"err.config_failed":           {ZhCN: "❌ 配置失败", EnUS: "❌ Configuration failed"},
	"err.test_failed":             {ZhCN: "❌ 测试失败", EnUS: "❌ Test failed"},
	"err.doctor_failed":           {ZhCN: "❌ 诊断失败", EnUS: "❌ Diagnostics failed"},
	"err.view_failed":             {ZhCN: "❌ 读取配置失败", EnUS: "❌ Failed to read config"},
	"err.clean_failed":            {ZhCN: "❌ 清理失败", EnUS: "❌ Reset failed"},
	"err.save_failed":             {ZhCN: "保存配置失败", EnUS: "failed to save config"},
	"err.wechat_not_configured":   {ZhCN: "未配置企业微信 Webhook URL，请先运行配置向导", EnUS: "WeChat Work Webhook URL not configured; please run setup first"},
	"err.dingtalk_not_configured": {ZhCN: "未配置钉钉 Webhook URL，请先运行配置向导", EnUS: "DingTalk Webhook URL not configured; please run setup first"},
	"err.bark_not_configured":     {ZhCN: "未配置 Bark Webhook URL，请先运行配置向导", EnUS: "Bark Webhook URL not configured; please run setup first"},
	"err.ntfy_not_configured":     {ZhCN: "未配置 Ntfy Topic URL，请先运行配置向导", EnUS: "Ntfy Topic URL not configured; please run setup first"},
	"err.slack_not_configured":    {ZhCN: "未配置 Slack Webhook URL，请先运行配置向导", EnUS: "Slack Webhook URL not configured; please run setup first"},
	"err.xiaodu_not_configured":   {ZhCN: "未配置小度 API 地址或 Access Token，请先运行配置向导", EnUS: "Xiaodu API URL or Access Token not configured; please run setup first"},

	// ── Clean / reset flow ─────────────────────────────────────
	"clean.confirm":          {ZhCN: "确认清理所有配置？", EnUS: "Reset all configuration?"},
	"clean.cancelled":        {ZhCN: "已取消", EnUS: "Cancelled"},
	"clean.save_default_err": {ZhCN: "保存默认配置失败", EnUS: "failed to save default config"},
	"clean.delete_failed":    {ZhCN: "删除配置文件失败", EnUS: "failed to delete config file"},
	"clean.done":             {ZhCN: "✅ 配置已清理，下次配置时需要重新初始化飞书", EnUS: "✅ Config reset; Feishu will need re-initialization next time"},
	"clean.skip_hooks":       {ZhCN: "⚠️  跳过 %s hooks 清理: %v\n", EnUS: "⚠️  Skipped %s hooks cleanup: %v\n"},
	"clean.hooks_failed":     {ZhCN: "⚠️  清理 %s hooks 失败 (%s): %v\n", EnUS: "⚠️  Failed to clean %s hooks (%s): %v\n"},
	"clean.hooks_done":       {ZhCN: "✅ 已清理 %s hooks (%s)\n", EnUS: "✅ Cleaned %s hooks (%s)\n"},
	"clean.agent_closed":     {ZhCN: "%s 通知已关闭\n", EnUS: "%s notifications disabled\n"},

	// ── WeChat Work init ───────────────────────────────────────
	"wechat.init_done": {ZhCN: "✅ 企业微信 Webhook 配置完成", EnUS: "✅ WeChat Work Webhook configured"},

	// ── DingTalk init ──────────────────────────────────────────
	"dingtalk.init_done": {ZhCN: "✅ 钉钉 Webhook 配置完成", EnUS: "✅ DingTalk Webhook configured"},

	// ── Bark init ──────────────────────────────────────────────
	"bark.init_done": {ZhCN: "✅ Bark Webhook 配置完成", EnUS: "✅ Bark Webhook configured"},

	// ── Ntfy init ──────────────────────────────────────────────
	"ntfy.init_done": {ZhCN: "✅ Ntfy Topic 配置完成", EnUS: "✅ Ntfy Topic configured"},

	// ── Slack init ─────────────────────────────────────────────
	"slack.init_done": {ZhCN: "✅ Slack Webhook 配置完成", EnUS: "✅ Slack Webhook configured"},

	// ── Xiaodu init ────────────────────────────────────────────
	"xiaodu.init_done": {ZhCN: "✅ 小度音箱配置完成", EnUS: "✅ Xiaodu configured"},

	// ── View config table ──────────────────────────────────────
	"view.header":     {ZhCN: "| Agent        | 飞书 | 系统 | 企业微信 | 钉钉 | Bark | Ntfy | Slack | 小度 |", EnUS: "| Agent        | Feishu|System|  WeCom  | DingT.| Bark | Ntfy | Slack |Xiaodu|"},
	"view.separator":  {ZhCN: "+--------------+------+------+----------+------+------+------+-------+------+", EnUS: "+--------------+------+------+----------+------+------+------+-------+------+"},
	"view.row_format": {ZhCN: "| %-12s |  %s  |  %s  |    %s    |  %s  |  %s  |  %s  |  %s  |  %s  |", EnUS: "| %-12s |  %s  |  %s  |    %s    |  %s  |  %s  |  %s  |  %s  |  %s  |"},

	// ── Doctor output ──────────────────────────────────────────
	"doctor.config_file":      {ZhCN: "配置文件: %s\n\n", EnUS: "Config file: %s\n\n"},
	"doctor.agent_status":     {ZhCN: "【Agent 安装状态】", EnUS: "【Agent Installation Status】"},
	"doctor.agent_sep":        {ZhCN: "+--------------+----------+----------------+", EnUS: "+--------------+----------+----------------+"},
	"doctor.agent_header":     {ZhCN: "| Agent        | 安装状态 | 集成配置       |", EnUS: "| Agent        | Installed| Integration    |"},
	"doctor.channel_status":   {ZhCN: "【通知渠道状态】", EnUS: "【Notification Channels】"},
	"doctor.channel_sep":      {ZhCN: "+--------------+------+------+----------+------+------+------+-------+------+", EnUS: "+--------------+------+------+----------+------+------+------+-------+------+"},
	"doctor.channel_header":   {ZhCN: "| Agent        | 飞书 | 系统 | 企业微信 | 钉钉 | Bark | Ntfy | Slack | 小度 |", EnUS: "| Agent        | Feishu|System|  WeCom  | DingT.| Bark | Ntfy | Slack |Xiaodu|"},
	"doctor.system_env":       {ZhCN: "【系统环境】", EnUS: "【System Environment】"},
	"doctor.env_sep":          {ZhCN: "+----------------------+------------+", EnUS: "+----------------------+------------+"},
	"doctor.env_header":       {ZhCN: "| 检查项               | 状态       |", EnUS: "| Check Item           | Status     |"},
	"doctor.item_config":      {ZhCN: "配置文件", EnUS: "Config file"},
	"doctor.item_feishu_cli":  {ZhCN: "飞书 CLI", EnUS: "Feishu CLI"},
	"doctor.item_click_focus": {ZhCN: "点击聚焦", EnUS: "Click-Focus"},
	"doctor.row_format":       {ZhCN: "| %-12s | %s | %s |", EnUS: "| %-12s | %s | %s |"},
	"doctor.env_row_format":   {ZhCN: "| %s | %s |", EnUS: "| %s | %s |"},

	// ── Doctor status labels ───────────────────────────────────
	"status.installed":                  {ZhCN: "✅ 已安装", EnUS: "✅ Installed"},
	"status.not_installed":              {ZhCN: "❌ 未安装", EnUS: "❌ Not installed"},
	"status.config_present":             {ZhCN: "✅ 已存在", EnUS: "✅ Present"},
	"status.config_missing":             {ZhCN: "❌ 不存在", EnUS: "❌ Missing"},
	"status.available":                  {ZhCN: "✅ 可用", EnUS: "✅ Available"},
	"status.unavailable":                {ZhCN: "❌ 不可用", EnUS: "❌ Unavailable"},
	"status.ready":                      {ZhCN: "✅ 已就绪", EnUS: "✅ Ready"},
	"status.not_configured":             {ZhCN: "❌ 未配置", EnUS: "❌ Not configured"},
	"status.integration_installed":      {ZhCN: "✅ 已安装", EnUS: "✅ Installed"},
	"status.integration_agent_missing":  {ZhCN: "❌ 未安装 Agent", EnUS: "❌ Agent not found"},
	"status.integration_config_missing": {ZhCN: "❌ 缺少配置", EnUS: "❌ Config missing"},
	"status.integration_not_integrated": {ZhCN: "❌ 未集成", EnUS: "❌ Not integrated"},
	"status.integration_unknown":        {ZhCN: "❌ 未知", EnUS: "❌ Unknown"},
	"doctor.system_notify_name":         {ZhCN: "系统通知", EnUS: "System Notification"},

	// ── Test notification content ──────────────────────────────
	"test.msg_title":         {ZhCN: "Agent Notify 测试", EnUS: "Agent Notify Test"},
	"test.msg_body":          {ZhCN: "这是一条测试消息", EnUS: "This is a test notification"},
	"test.msg_body_wechat":   {ZhCN: "这是一条企业微信测试消息", EnUS: "This is a WeChat Work test notification"},
	"test.msg_body_dingtalk": {ZhCN: "这是一条钉钉测试消息", EnUS: "This is a DingTalk test notification"},
	"test.msg_body_bark":     {ZhCN: "这是一条 Bark 测试消息", EnUS: "This is a Bark test notification"},
	"test.msg_body_ntfy":     {ZhCN: "这是一条 Ntfy 测试消息", EnUS: "This is a Ntfy test notification"},
	"test.msg_body_slack":    {ZhCN: "这是一条 Slack 测试消息", EnUS: "This is a Slack test notification"},
	"test.msg_body_xiaodu":   {ZhCN: "这是一条小度音箱测试消息", EnUS: "This is a Xiaodu test notification"},
	"test.feishu_sent":       {ZhCN: "飞书测试通知已发送", EnUS: "Feishu test notification sent"},
	"test.system_sent":       {ZhCN: "系统测试通知已发送", EnUS: "System test notification sent"},
	"test.wechat_sent":       {ZhCN: "企业微信测试通知已发送", EnUS: "WeChat Work test notification sent"},
	"test.dingtalk_sent":     {ZhCN: "钉钉测试通知已发送", EnUS: "DingTalk test notification sent"},
	"test.bark_sent":         {ZhCN: "Bark 测试通知已发送", EnUS: "Bark test notification sent"},
	"test.ntfy_sent":         {ZhCN: "Ntfy 测试通知已发送", EnUS: "Ntfy test notification sent"},
	"test.slack_sent":        {ZhCN: "Slack 测试通知已发送", EnUS: "Slack test notification sent"},
	"test.xiaodu_sent":       {ZhCN: "小度音箱测试通知已发送", EnUS: "Xiaodu test notification sent"},

	// ── Setup service messages ─────────────────────────────────
	"setup.config_file":        {ZhCN: "配置文件: %s\n", EnUS: "Config file: %s\n"},
	"setup.codex_tip":          {ZhCN: "提示: 请在 codex 内运行 /hooks 完成 trust 审核\n", EnUS: "Tip: Run /hooks inside Codex to complete the trust review\n"},
	"setup.feishu_init_err":    {ZhCN: "飞书初始化失败", EnUS: "Feishu initialization failed"},
	"setup.claude_hooks_err":   {ZhCN: "获取 claude settings 路径失败", EnUS: "failed to get Claude settings path"},
	"setup.claude_install_err": {ZhCN: "安装 claude hooks 失败", EnUS: "failed to install Claude hooks"},
	"setup.codex_hooks_err":    {ZhCN: "获取 codex hooks 路径失败", EnUS: "failed to get Codex hooks path"},
	"setup.codex_install_err":  {ZhCN: "安装 codex hooks 失败", EnUS: "failed to install Codex hooks"},
	"setup.claude_hooks_done":  {ZhCN: "claude hooks 安装: %s\n", EnUS: "Claude hooks installed: %s\n"},
	"setup.codex_hooks_done":   {ZhCN: "codex hooks 安装: %s\n", EnUS: "Codex hooks installed: %s\n"},
	"setup.zcode_tip":          {ZhCN: "提示: 请重启 ZCode 使 hooks 配置生效\n", EnUS: "Tip: Restart ZCode for the hooks configuration to take effect\n"},
	"setup.zcode_hooks_err":    {ZhCN: "获取 zcode config 路径失败", EnUS: "failed to get ZCode config path"},
	"setup.zcode_install_err":  {ZhCN: "安装 zcode hooks 失败", EnUS: "failed to install ZCode hooks"},
	"setup.zcode_hooks_done":   {ZhCN: "zcode hooks 安装: %s\n", EnUS: "ZCode hooks installed: %s\n"},
}
