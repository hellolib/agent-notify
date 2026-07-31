package droidhooks

import (
	"errors"
	"os"

	"github.com/hellolib/agent-notify/internal/common"
)

// hookCommandMarker 用于识别本插件写入的 Droid hook。
// 卸载 / 增量安装时按此子串匹配 command 字段。
const hookCommandMarker = "handle-droid-hook"

// managedEvents 是本插件托管的 Droid 事件列表。
// Droid hooks 配置使用 PascalCase 事件名（与 Claude Code 兼容），
// 存储在 ~/.factory/hooks.json（user）或 .factory/hooks.json（project）。
// SessionStart 仅用于点击聚焦的窗口捕获（见 agenthooks.Dispatch），不产生通知。
// Notification 事件通过 notification_type 区分：permission_prompt → 授权等待，
// idle_prompt → 输入等待；auth_success / elicitation_dialog 在 ParseMessage 中静默丢弃。
// Stop 映射为 run_completed。
// Droid 没有 PostToolUseFailure 等失败事件，故不产生 run_failed 通知。
var managedEvents = []string{
	"SessionStart",
	"Notification",
	"Stop",
}

// BuildHookSettings 生成 Droid hooks JSON 结构。
// Droid 的 hooks.json 中事件直接位于顶层（不嵌套在 "hooks" 下），例如：
//
//	{"SessionStart": [...], "Notification": [...], "Stop": [...]}
func BuildHookSettings(binaryPath string) map[string]any {
	binaryPath = common.ResolveBinaryPath(binaryPath)
	command := common.QuotePathForShell(binaryPath) + " " + hookCommandMarker

	entry := func() []map[string]any {
		return []map[string]any{
			{
				"hooks": []map[string]any{
					{
						"type":    "command",
						"command": command,
					},
				},
			},
		}
	}

	settings := map[string]any{}
	for _, name := range managedEvents {
		settings[name] = entry()
	}
	return settings
}

// Install 以增量方式写入 Droid hook 文件：若某事件下已存在 agent-notify 的 hook 则跳过，
// 不覆盖用户自己挂载的其他 hook。
// Droid 的 hooks.json 事件直接位于顶层，不嵌套在 "hooks" 对象中。
func Install(path string, binaryPath string) error {
	settings, err := common.ReadOrderedSettings(path)
	if err != nil {
		return err
	}

	binaryPath = common.ResolveBinaryPath(binaryPath)
	command := common.QuotePathForShell(binaryPath) + " " + hookCommandMarker

	if err := common.InstallManagedHooks(&settings, managedEvents, hookCommandMarker, command,
		common.RefuseNonArrayEvent("(root)")); err != nil {
		return err
	}

	return common.WriteOrderedSettings(path, settings)
}

// IsInstalled 检查 hook 文件中是否已挂载 agent-notify 的 hook。
// 只要任一托管事件下存在标记命令就视为已安装。
func IsInstalled(path string) (bool, error) {
	settings, err := common.ReadOrderedSettings(path)
	if err != nil {
		return false, err
	}
	return common.HasManagedHook(settings, managedEvents, hookCommandMarker), nil
}

// Uninstall 仅移除本插件写入的 hook 条目（command 含 handle-droid-hook）。
// 用户挂在同一事件下的其他 hook 原样保留。文件不存在时是 no-op。
// 若卸载后所有托管事件均已清除，且文件中仅剩非事件键（如 hooksDisabled、showHookOutput），
// 则删除整个文件。
func Uninstall(path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	settings, err := common.ReadOrderedSettings(path)
	if err != nil {
		return err
	}

	if err := common.UninstallManagedHooks(&settings, hookCommandMarker); err != nil {
		return err
	}

	if settings.Len() == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	return common.WriteOrderedSettings(path, settings)
}
