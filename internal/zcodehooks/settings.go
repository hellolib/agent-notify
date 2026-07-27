package zcodehooks

import (
	"encoding/json"
	"errors"
	"fmt"
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

// validEvents 是 ZCode 认可的全部 7 个事件名。schema 为 strict,
// hooks 对象里出现任何这之外的键都会让整个 hooks 配置被静默丢弃,
// 因此写入前必须校验既有内容的形状(issue #35)。
var validEvents = map[string]bool{
	"SessionStart":       true,
	"UserPromptSubmit":   true,
	"PreToolUse":         true,
	"PermissionRequest":  true,
	"PostToolUse":        true,
	"PostToolUseFailure": true,
	"Stop":               true,
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

	events, _ := hooks["events"].(map[string]any)
	if events == nil {
		events = map[string]any{}
	}

	// 写入前先规整既有形状:把历史/手写的扁平事件键迁进 events,
	// 遇到无法识别的键则拒绝写入——留着它会让 ZCode 丢弃整个 hooks 配置。
	if err := normalizeHooksShape(hooks, events); err != nil {
		return err
	}

	// enabled 是用户所有 ZCode hook 的总开关,不属于我们。
	// 仅在缺失时创建;用户显式关闭时拒绝安装并说明,而不是静默替他打开。
	switch enabled := hooks["enabled"].(type) {
	case nil:
		hooks["enabled"] = true
	case bool:
		if !enabled {
			return fmt.Errorf("%s 中 hooks.enabled = false(ZCode hook 总开关已关闭),"+
				"agent-notify 的通知不会触发;请先将其改为 true 再重新安装", path)
		}
	default:
		return fmt.Errorf("%s 中 hooks.enabled 不是布尔值(%T),请手动修正后重试", path, enabled)
	}

	for _, event := range managedEvents {
		// 已有托管 hook:同步 command(路径可能已过期,issue #34)后跳过追加
		if found, _ := common.SyncManagedHookCommand(events, event, hookCommandMarker, command); found {
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
			// events 已空说明没有任何 hook 了,此时留着 enabled=true 只会把
			// 「我们安装时创建的开关」永久留在用户配置里。仅在其为 true 时清理:
			// false 一定是用户自己设的(安装时我们从不写 false),必须保留。
			if enabled, ok := hooks["enabled"].(bool); ok && enabled {
				delete(hooks, "enabled")
			}
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

// normalizeHooksShape 把 hooks 对象规整成 ZCode 认可的 {enabled, events} 形状。
//
// 历史版本或手写配置可能留下 Claude 风格的扁平事件键(hooks.Stop = [...]),
// 它对 ZCode 的 strict schema 是未知键,会导致整个 hooks 配置被静默丢弃——
// 连我们刚写进去的 hook 一起失效。合法事件名的扁平键迁移进 events;
// 其余无法识别的键无法安全处理,直接报错拒绝写入,把决定权交还用户。
func normalizeHooksShape(hooks, events map[string]any) error {
	for key, value := range hooks {
		if key == "enabled" || key == "events" {
			continue
		}
		if !validEvents[key] {
			return fmt.Errorf("hooks 中存在无法识别的键 %q,ZCode 会因此丢弃整个 hooks 配置;"+
				"请先手动移除该键或将其移入 hooks.events 后重试", key)
		}
		entries := common.ToAnySlice(value)
		if entries == nil {
			return fmt.Errorf("hooks.%s 不是数组,无法自动迁移到 hooks.events;请手动调整后重试", key)
		}
		events[key] = append(common.ToAnySlice(events[key]), entries...)
		delete(hooks, key)
	}
	return nil
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
