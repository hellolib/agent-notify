package antigravityhooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/hellolib/agent-notify/internal/common"
)

// hookName 是写入 Antigravity hooks.json 顶层的具名 hook 键。
// Antigravity 的 hooks.json 按具名 hook 组织，使用独立键名与用户自己的其它 hook
// 天然隔离，安装与卸载互不干扰。
const hookName = "agent-notify"

// hookCommandMarker 用于识别本插件写入的 Antigravity hook。
// 卸载、增量更新以及 doctor 二进制存在性检查都按此子串匹配 command 字段。
const hookCommandMarker = "handle-antigravity-hook"

// preToolUseMatcher 仅拦截需要用户交互/授权的内置工具，避免在每个普通工具调用上
// 同步拉起子进程拖慢 Agent 循环。
const preToolUseMatcher = "ask_question|ask_permission"

var (
	// flatEvents 使用扁平 handler 数组：[{"type":"command","command":"..."}]。
	flatEvents = []string{"SessionStart", "Stop"}
	// groupedEvents 使用分组结构：[{"matcher":"...","hooks":[{"type":"command","command":"..."}]}]。
	groupedEvents = []string{"PreToolUse"}
)

// BuildHookSettings 生成 Antigravity hooks.json 结构：
//
//	{
//	  "agent-notify": {
//	    "SessionStart": [{"type": "command", "command": "..."}],
//	    "PreToolUse": [{"matcher": "ask_question|ask_permission", "hooks": [{"type": "command", "command": "..."}]}],
//	    "Stop": [{"type": "command", "command": "..."}]
//	  }
//	}
func BuildHookSettings(binaryPath string) map[string]any {
	command := hookCommand(binaryPath)
	handler := func() map[string]any {
		return map[string]any{
			"type":    "command",
			"command": command,
		}
	}
	return map[string]any{
		hookName: map[string]any{
			"SessionStart": []map[string]any{handler()},
			"PreToolUse": []map[string]any{
				{
					"matcher": preToolUseMatcher,
					"hooks":   []map[string]any{handler()},
				},
			},
			"Stop": []map[string]any{handler()},
		},
	}
}

func hookCommand(binaryPath string) string {
	binaryPath = common.ResolveBinaryPath(binaryPath)
	return common.QuotePathForShell(binaryPath) + " " + hookCommandMarker
}

// Install 以增量方式写入 Antigravity hooks.json：
//   - 保留文件中用户定义的其它具名 hook 原样不变；
//   - 在 "agent-notify" 键下写入/刷新 SessionStart、PreToolUse 与 Stop 三个事件；
//   - 若 "agent-notify" 已存在但形态不是 JSON 对象，报错中止以免无声覆盖用户配置。
func Install(path string, binaryPath string) error {
	settings, err := common.ReadOrderedSettings(path)
	if err != nil {
		return err
	}

	// 验证已有 "agent-notify" 键的形态（若存在且非 null，则必须是对象）。
	if _, err := common.ChildObject(settings, hookName); err != nil {
		return fmt.Errorf("%s %w; 无法安全写入 agent-notify hook，请先检查 hooks.json 后重试", hookName, err)
	}

	command := hookCommand(binaryPath)
	hookDef := common.OrderedObject{}

	flatEntry, err := json.Marshal([]map[string]any{
		{
			"type":    "command",
			"command": command,
		},
	})
	if err != nil {
		return err
	}
	groupedEntry, err := json.Marshal([]map[string]any{
		{
			"matcher": preToolUseMatcher,
			"hooks": []map[string]any{
				{
					"type":    "command",
					"command": command,
				},
			},
		},
	})
	if err != nil {
		return err
	}

	hookDef.Set("SessionStart", flatEntry)
	hookDef.Set("PreToolUse", groupedEntry)
	hookDef.Set("Stop", flatEntry)

	if err := common.SetChildObject(&settings, hookName, hookDef); err != nil {
		return err
	}

	return common.WriteOrderedSettings(path, settings)
}

// IsInstalled 检查 hooks.json 中是否已注册 agent-notify 的 hook。
// 遍历顶层所有具名 hook，只要任一启用的 hook 含 handle-antigravity-hook 命令即视为已安装。
func IsInstalled(path string) (bool, error) {
	settings, err := common.ReadOrderedSettings(path)
	if err != nil {
		return false, err
	}
	for _, name := range settings.Keys() {
		hookObj, err := common.ChildObject(settings, name)
		if err != nil {
			continue
		}
		if isHookDisabled(hookObj) {
			continue
		}
		if hookObjectHasManagedCommand(hookObj) {
			return true, nil
		}
	}
	return false, nil
}

// Uninstall 仅移除本插件写入的 hook 条目（command 含 handle-antigravity-hook）。
// 用户自定义的其它具名 hook 原样保留；文件不存在时是 no-op；
// 若卸载后 hooks.json 变为空对象，则删除整个文件。
func Uninstall(path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	settings, err := common.ReadOrderedSettings(path)
	if err != nil {
		return err
	}

	for _, name := range settings.Keys() {
		hookObj, err := common.ChildObject(settings, name)
		if err != nil {
			continue
		}

		changed := false
		for _, event := range hookObj.Keys() {
			if event == "enabled" {
				continue
			}
			raw, _ := hookObj.Get(event)
			updated, modified, empty := stripEventHandlers(raw, hookCommandMarker)
			if !modified {
				continue
			}
			changed = true
			if empty {
				hookObj.Delete(event)
			} else {
				hookObj.Set(event, updated)
			}
		}

		if !changed {
			continue
		}

		if hasOnlyMetadataKeys(hookObj) {
			settings.Delete(name)
			continue
		}
		if err := common.SetChildObject(&settings, name, hookObj); err != nil {
			return err
		}
	}

	if settings.Len() == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	return common.WriteOrderedSettings(path, settings)
}

func isHookDisabled(hookObj common.OrderedObject) bool {
	raw, ok := hookObj.Get("enabled")
	if !ok {
		return false
	}
	var enabled bool
	if err := json.Unmarshal(raw, &enabled); err != nil {
		return false
	}
	return !enabled
}

func hookObjectHasManagedCommand(hookObj common.OrderedObject) bool {
	for _, event := range append(append([]string(nil), flatEvents...), groupedEvents...) {
		raw, ok := hookObj.Get(event)
		if !ok {
			continue
		}
		if eventArrayHasMarker(raw, hookCommandMarker) {
			return true
		}
	}
	return false
}

func eventArrayHasMarker(raw json.RawMessage, marker string) bool {
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return false
	}
	for _, item := range items {
		if flatHandlerHasMarker(item, marker) || common.EntryHasManagedHook(item, marker) {
			return true
		}
	}
	return false
}

func flatHandlerHasMarker(raw json.RawMessage, marker string) bool {
	var h struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(raw, &h); err != nil {
		return false
	}
	return strings.Contains(h.Command, marker)
}

// stripEventHandlers 同时兼容扁平 handler 数组（SessionStart/Stop/PreInvocation/PostInvocation）
// 与分组数组（PreToolUse/PostToolUse），移除含 marker 的条目并保留其余条目。
func stripEventHandlers(raw json.RawMessage, marker string) (json.RawMessage, bool, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return raw, false, false
	}

	var items []json.RawMessage
	if err := json.Unmarshal(trimmed, &items); err != nil {
		return raw, false, false
	}

	kept := make([]json.RawMessage, 0, len(items))
	modified := false
	for _, item := range items {
		if flatHandlerHasMarker(item, marker) {
			modified = true
			continue
		}
		if common.EntryHasManagedHook(item, marker) {
			modified = true
			stripped, keep := common.StripManagedHooks(item, marker)
			if keep {
				kept = append(kept, stripped)
			}
			continue
		}
		kept = append(kept, item)
	}

	if !modified {
		return raw, false, false
	}
	if len(kept) == 0 {
		return nil, true, true
	}
	encoded, err := json.Marshal(kept)
	if err != nil {
		return raw, false, false
	}
	return encoded, true, false
}

func hasOnlyMetadataKeys(hookObj common.OrderedObject) bool {
	for _, key := range hookObj.Keys() {
		if key != "enabled" {
			return false
		}
	}
	return true
}
