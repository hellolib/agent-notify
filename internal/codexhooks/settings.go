package codexhooks

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/hellolib/agent-notify/internal/common"
)

// hookCommandMarker 用于识别本插件写入的 Codex hook。
const hookCommandMarker = "handle-codex-hook"

// managedEvents 是本插件托管的 Codex 事件列表。
// PostToolUse 仅用于取消小度权限提醒，不产生用户通知。
var managedEvents = []string{
	"PermissionRequest",
	"Stop",
	"PostToolUse",
}

// BuildHookSettings 生成 Codex hooks.json 所需的 settings 结构。
func BuildHookSettings(binaryPath string) map[string]any {
	binaryPath = common.ResolveBinaryPath(binaryPath)
	command := binaryPath + " " + hookCommandMarker

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

	hooks := map[string]any{}
	for _, event := range managedEvents {
		hooks[event] = buildEntry()
	}
	return map[string]any{"hooks": hooks}
}

// Install 以增量方式写入 hooks.json：已存在 agent-notify 的 hook 则跳过，
// 不覆盖用户自己挂载的其他 hook。同时确保 config.toml 中 [features] hooks = true。
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
		entries := removeManagedHooks(toAnySlice(hooks[event]))
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

	if err := writeSettings(path, settings); err != nil {
		return err
	}

	// 同步启用 config.toml 中的 [features] hooks = true
	configTomlPath := filepath.Join(filepath.Dir(path), "config.toml")
	if err := EnableHooksFeature(configTomlPath); err != nil {
		log.Printf("warning: failed to enable hooks feature in %s: %v", configTomlPath, err)
	}

	return nil
}

// IsInstalled 检查 hooks.json 中是否已挂载 agent-notify 的 hook。
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

// Uninstall 仅移除本插件写入的 hook 条目（command 含 handle-codex-hook）。
// config.toml 中的 [features] hooks 开关不动 —— 那是通用开关，可能被其他 hook 使用。
// 文件不存在时是 no-op。
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
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooks
	}

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

func removeManagedHooks(entries []any) []any {
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
	return cleaned
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
