package grokhooks

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

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
	command := binaryPath + " " + hookCommandMarker

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
	settings, err := readSettings(path)
	if err != nil {
		return err
	}

	binaryPath = common.ResolveBinaryPath(binaryPath)
	command := binaryPath + " " + hookCommandMarker

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}

	for _, event := range managedEvents {
		if eventHasManagedHook(hooks, event) {
			continue
		}
		entries := toAnySlice(hooks[event])
		entries = append(entries, map[string]any{
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": command,
				},
			},
		})
		hooks[event] = entries
	}
	settings["hooks"] = hooks

	return writeSettings(path, settings)
}

// IsInstalled 检查 hook 文件中是否已挂载 agent-notify 的 hook。
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

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return false, nil
	}

	for _, event := range managedEvents {
		if eventHasManagedHook(hooks, event) {
			return true, nil
		}
	}
	return false, nil
}

// Uninstall 仅移除本插件写入的 hook 条目（command 含 handle-grok-hook）。
// 用户挂在同一事件下的其他 hook 原样保留。文件不存在时是 no-op。
// 若卸载后 hooks 为空且文件仅为本插件服务，则删除整个文件。
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

	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return nil
	}

	for event, raw := range hooks {
		entries := toAnySlice(raw)
		cleaned := entries[:0]
		for _, entry := range entries {
			entryMap, ok := entry.(map[string]any)
			if !ok {
				cleaned = append(cleaned, entry)
				continue
			}
			inner := toAnySlice(entryMap["hooks"])
			keptInner := inner[:0]
			for _, h := range inner {
				if !isManagedHook(h) {
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
			delete(hooks, event)
		} else {
			hooks[event] = cleaned
		}
	}

	if len(hooks) == 0 {
		// 文件已空：删除专用 hook 文件，避免留下空 JSON
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	settings["hooks"] = hooks
	return writeSettings(path, settings)
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
	return os.WriteFile(path, out, 0o644)
}

func eventHasManagedHook(hooks map[string]any, event string) bool {
	for _, entry := range toAnySlice(hooks[event]) {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		for _, h := range toAnySlice(entryMap["hooks"]) {
			if isManagedHook(h) {
				return true
			}
		}
	}
	return false
}

func isManagedHook(hook any) bool {
	m, ok := hook.(map[string]any)
	if !ok {
		return false
	}
	cmd, _ := m["command"].(string)
	return strings.Contains(cmd, hookCommandMarker)
}

func toAnySlice(v any) []any {
	switch s := v.(type) {
	case []any:
		return s
	case []map[string]any:
		out := make([]any, 0, len(s))
		for _, item := range s {
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}
