package zcodehooks

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/hellolib/agent-notify/internal/common"
)

// hookCommandMarker 用于识别本插件写入的 ZCode hook。
const hookCommandMarker = "handle-zcode-hook"

// managedEvents 是本插件托管的 ZCode 事件列表。
// ZCode 内置的 hook 事件枚举共 7 个：
//
//	SessionStart / UserPromptSubmit / PreToolUse / PermissionRequest /
//	PostToolUse / PostToolUseFailure / Stop
//
// 注意：ZCode 没有 Claude Code 的 Notification 事件，且其 schema 为 strict，
// 任何未知事件名都会导致整个 hooks 配置加载失败，因此这里只能列合法事件。
// 这里托管与通知最相关的 4 个；PostToolUse（成功）默认不托管，避免噪音。
var managedEvents = []string{
	"SessionStart",
	"PermissionRequest",
	"PostToolUseFailure",
	"Stop",
}

// BuildHookSettings 生成 ZCode config.json 所需的 hooks 结构。
//
// ZCode 与 Claude Code 的关键差异：事件挂在 hooks.events.<Event> 下，
// 且需要 hooks.enabled = true，否则配置不生效。
// 结构示例：
//
//	{
//	  "hooks": {
//	    "enabled": true,
//	    "events": {
//	      "Stop": [{ "hooks": [{ "type":"command","command":"<bin> handle-zcode-hook" }] }]
//	    }
//	  }
//	}
func BuildHookSettings(binaryPath string) map[string]any {
	binaryPath = common.ResolveBinaryPath(binaryPath)
	command := common.QuotePathForShell(binaryPath) + " " + hookCommandMarker

	buildEntry := func() []map[string]any {
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

	events := map[string]any{}
	for _, event := range managedEvents {
		events[event] = buildEntry()
	}
	hooks := map[string]any{
		"enabled": true,
		"events":  events,
	}
	return map[string]any{"hooks": hooks}
}

// Install 以增量方式写入 ZCode config.json：已存在 agent-notify 的 hook 则跳过，
// 不覆盖用户自己挂载的其他 hook，也不破坏 config.json 里的其它顶层键（如 mcp）。
func Install(path string, binaryPath string) error {
	settings, err := readSettings(path)
	if err != nil {
		return err
	}

	binaryPath = common.ResolveBinaryPath(binaryPath)
	command := common.QuotePathForShell(binaryPath) + " " + hookCommandMarker

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	hooks["enabled"] = true

	events, _ := hooks["events"].(map[string]any)
	if events == nil {
		events = map[string]any{}
	}

	for _, event := range managedEvents {
		if common.EventHasManagedHook(events, event, hookCommandMarker) {
			continue
		}
		entries := common.ToAnySlice(events[event])
		entries = append(entries, map[string]any{
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": command,
				},
			},
		})
		events[event] = entries
	}
	hooks["events"] = events
	settings["hooks"] = hooks

	return writeSettings(path, settings)
}

// IsInstalled 检查 ZCode config.json 中是否已挂载 agent-notify 的 hook。
func IsInstalled(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	settings := map[string]any{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return false, err
		}
	}

	events := eventsOf(settings)
	for _, event := range managedEvents {
		if common.EventHasManagedHook(events, event, hookCommandMarker) {
			return true, nil
		}
	}
	return false, nil
}

// Uninstall 仅移除本插件写入的 hook 条目（command 含 handle-zcode-hook）。
// 文件不存在时是 no-op；config.json 中的其它顶层键与用户自定义事件原样保留。
func Uninstall(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}

	settings := map[string]any{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return err
		}
	}

	events := eventsOf(settings)
	if events == nil {
		return nil
	}

	for event, raw := range events {
		entries := common.ToAnySlice(raw)
		cleaned := entries[:0]
		for _, entry := range entries {
			entryMap, ok := entry.(map[string]any)
			if !ok {
				cleaned = append(cleaned, entry)
				continue
			}
			inner := common.ToAnySlice(entryMap["hooks"])
			keptInner := inner[:0]
			for _, h := range inner {
				if !common.IsManagedHook(h, hookCommandMarker) {
					keptInner = append(keptInner, h)
				}
			}
			if len(keptInner) == 0 {
				continue
			}
			entryMap["hooks"] = keptInner
			cleaned = append(cleaned, entryMap)
		}
		if len(cleaned) == 0 {
			delete(events, event)
		} else {
			events[event] = cleaned
		}
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if len(events) == 0 {
		if hooks != nil {
			delete(hooks, "events")
		}
	} else if hooks != nil {
		hooks["events"] = events
	}
	// 若 hooks 整个对象已空，则移除 hooks 键，保持配置整洁
	if hooks != nil {
		if len(hooks) == 0 {
			delete(settings, "hooks")
		} else {
			settings["hooks"] = hooks
		}
	}

	return writeSettings(path, settings)
}

// eventsOf 从 settings 中取出 hooks.events 子对象（兼容历史/手写配置）。
func eventsOf(settings map[string]any) map[string]any {
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return nil
	}
	events, _ := hooks["events"].(map[string]any)
	return events
}

func readSettings(path string) (map[string]any, error) {
	settings := map[string]any{}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return settings, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return settings, nil
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}
	return settings, nil
}

func writeSettings(path string, settings map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	// 用户配置文件:原子写 + 覆盖式 .bak 备份(issue #29)
	return common.WriteFileAtomicWithBackup(path, out, 0o644)
}
