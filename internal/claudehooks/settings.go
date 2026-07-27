package claudehooks

import (
	"errors"
	"os"

	"github.com/hellolib/agent-notify/internal/common"
)

// hookCommandMarker 用于识别本插件写入的 hook。
// 卸载 / 增量安装时按此子串匹配 command 字段。
const hookCommandMarker = "handle-claude-hook"

// managedEvents 是本插件托管的 Claude Code 事件列表。
// SessionStart 仅用于 Linux 点击聚焦的窗口捕获（见 agenthooks.Dispatch），
// 不产生任何通知；其它平台收到即 no-op。
var managedEvents = []string{
	"SessionStart",
	"PermissionRequest",
	"Notification",
	"Stop",
	"PostToolUseFailure",
}

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

// Install 以增量方式写入 hooks：若某事件下已存在 agent-notify 的 hook 则跳过，
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
		common.SkipNonArrayEvent); err != nil {
		return err
	}
	if err := common.SetChildObject(&settings, "hooks", hooks); err != nil {
		return err
	}

	return common.WriteOrderedSettings(path, settings)
}

// IsInstalled 检查 settings 中是否已挂载 agent-notify 的 hook。
// 只要任一托管事件下存在标记命令就视为已安装。
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

// Uninstall 仅移除本插件写入的 hook 条目（command 含 handle-claude-hook）。
// 用户挂在同一事件下的其他 hook 原样保留。文件不存在时是 no-op。
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
		// hooks 不是对象:不是我们写的形态,不动它
		return nil
	}

	if err := common.UninstallManagedHooks(&hooks, hookCommandMarker); err != nil {
		return err
	}

	if hooks.Len() == 0 {
		settings.Delete("hooks")
	} else if err := common.SetChildObject(&settings, "hooks", hooks); err != nil {
		return err
	}

	return common.WriteOrderedSettings(path, settings)
}
