package grokhooks

import (
	"errors"
	"os"

	"github.com/hellolib/agent-notify/internal/common"
)

// hookCommandMarker 用于识别本插件写入的 Grok hook。
// 卸载 / 增量安装时按此子串匹配 command 字段。
const hookCommandMarker = "handle-grok-hook"

// managedEvents 是本插件托管的 Grok 事件列表。
// Grok hooks 配置使用 PascalCase 事件名（与 Claude Code 兼容），
// stdin 投递的 hookEventName 则为 snake_case（如 session_start）。
// Grok 没有 PermissionRequest；等待授权/输入主要靠 Notification。
// StopFailure / PostToolUseFailure 映射为 run_failed。
var managedEvents = []string{
	"SessionStart",
	"Notification",
	"Stop",
	"StopFailure",
	"PostToolUseFailure",
}

// BuildHookSettings 生成 Grok hooks JSON 结构。
// Grok 从 ~/.grok/hooks/*.json 加载 hooks，结构与 Claude settings.json 的 hooks 段一致。
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

	hooks := map[string]any{}
	for _, name := range managedEvents {
		hooks[name] = entry()
	}
	return map[string]any{"hooks": hooks}
}

// Install 以增量方式写入 Grok hook 文件：若某事件下已存在 agent-notify 的 hook 则跳过，
// 不覆盖用户自己挂载的其他 hook。
func Install(path string, binaryPath string) error {
	settings, err := common.ReadOrderedSettings(path)
	if err != nil {
		return err
	}

	binaryPath = common.ResolveBinaryPath(binaryPath)
	command := common.QuotePathForShell(binaryPath) + " " + hookCommandMarker

	hooks, err := common.ChildObject(settings, "hooks")
	if err != nil {
		return err
	}
	if err := common.InstallManagedHooks(&hooks, managedEvents, hookCommandMarker, command,
		common.RefuseNonArrayEvent("hooks")); err != nil {
		return err
	}
	if err := common.SetChildObject(&settings, "hooks", hooks); err != nil {
		return err
	}

	return common.WriteOrderedSettings(path, settings)
}

// IsInstalled 检查 hook 文件中是否已挂载 agent-notify 的 hook。
func IsInstalled(path string) (bool, error) {
	settings, err := common.ReadOrderedSettings(path)
	if err != nil {
		return false, err
	}
	hooks, err := common.ChildObject(settings, "hooks")
	if err != nil {
		return false, nil
	}
	return common.HasManagedHook(hooks, managedEvents, hookCommandMarker), nil
}

// Uninstall 仅移除本插件写入的 hook 条目（command 含 handle-grok-hook）。
// 用户挂在同一事件下的其他 hook 原样保留。文件不存在时是 no-op。
// 若卸载后 hooks 为空且文件仅为本插件服务，则删除整个文件。
func Uninstall(path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	settings, err := common.ReadOrderedSettings(path)
	if err != nil {
		return err
	}
	if _, ok := settings.Get("hooks"); !ok {
		return nil
	}
	hooks, err := common.ChildObject(settings, "hooks")
	if err != nil {
		return nil
	}

	if err := common.UninstallManagedHooks(&hooks, hookCommandMarker); err != nil {
		return err
	}

	if hooks.Len() == 0 {
		// 文件已空：删除专用 hook 文件，避免留下空 JSON
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	if err := common.SetChildObject(&settings, "hooks", hooks); err != nil {
		return err
	}
	return common.WriteOrderedSettings(path, settings)
}
