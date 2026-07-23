package codexhooks

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"

	"github.com/hellolib/agent-notify/internal/common"
)

// hookCommandMarker 用于识别本插件写入的 Codex hook。
const hookCommandMarker = "handle-codex-hook"

// requestUserInputMatcher limits the PreToolUse hook to Codex's interactive
// question tool. Without this matcher every tool call would start a notifier
// process, even though only request_user_input can produce input_required.
const requestUserInputMatcher = "^request_user_input$"

// managedEvents 是本插件托管的 Codex 事件列表。
// PermissionRequest / PreToolUse(request_user_input) / Stop 对应项目里的
// permission_required / input_required / run_completed。
// SessionStart 仅用于 Linux 点击聚焦的窗口捕获（见 agenthooks.Dispatch），
// 不产生任何通知；其它平台收到即 no-op。
var managedEvents = []string{
	"SessionStart",
	"PermissionRequest",
	"PreToolUse",
	"Stop",
}

// BuildHookSettings 生成 Codex hooks.json 所需的 settings 结构。
func BuildHookSettings(binaryPath string) map[string]any {
	binaryPath = common.ResolveBinaryPath(binaryPath)
	command := binaryPath + " " + hookCommandMarker

	buildEntry := func(event string) []map[string]any {
		entry := map[string]any{
			"hooks": []map[string]any{
				{
					"type":    "command",
					"command": command,
				},
			},
		}
		if event == "PreToolUse" {
			entry["matcher"] = requestUserInputMatcher
		}
		return []map[string]any{entry}
	}

	hooks := map[string]any{}
	for _, event := range managedEvents {
		hooks[event] = buildEntry(event)
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
		entries := common.ToAnySlice(hooks[event])
		if event == "PreToolUse" {
			var found bool
			entries, found = repairManagedPreToolUseMatcher(entries)
			if found {
				hooks[event] = entries
				continue
			}
		}
		if common.EventHasManagedHook(hooks, event, hookCommandMarker) {
			continue
		}
		entry := map[string]any{
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": command,
				},
			},
		}
		if event == "PreToolUse" {
			entry["matcher"] = requestUserInputMatcher
		}
		entries = append(entries, entry)
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

// repairManagedPreToolUseMatcher upgrades existing agent-notify PreToolUse
// entries and returns the normalized entries plus whether a managed command
// was found. Older or manually edited installations may have the managed
// command without the exact matcher; keeping that broad entry would launch
// agent-notify for every tool call even though the handler only consumes
// request_user_input.
//
// A user can place multiple hooks in one matcher group. If such a group also
// contains our command, split the managed hook into its own exact-matcher
// group instead of changing the user's matcher.
func repairManagedPreToolUseMatcher(entries []any) ([]any, bool) {
	if len(entries) == 0 {
		return entries, false
	}

	normalized := make([]any, 0, len(entries)+1)
	found := false
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok {
			normalized = append(normalized, raw)
			continue
		}

		var managed, user []any
		for _, hook := range common.ToAnySlice(entry["hooks"]) {
			if common.IsManagedHook(hook, hookCommandMarker) {
				managed = append(managed, hook)
			} else {
				user = append(user, hook)
			}
		}
		if len(managed) == 0 {
			normalized = append(normalized, raw)
			continue
		}

		found = true
		if len(user) > 0 {
			// Keep the user's original matcher and all non-hook fields on the
			// original entry; only our command moves to the exact group.
			entry["hooks"] = user
			normalized = append(normalized, entry)

			managedEntry := make(map[string]any, len(entry)+1)
			for key, value := range entry {
				if key != "hooks" && key != "matcher" {
					managedEntry[key] = value
				}
			}
			managedEntry["matcher"] = requestUserInputMatcher
			managedEntry["hooks"] = managed
			normalized = append(normalized, managedEntry)
			continue
		}

		entry["matcher"] = requestUserInputMatcher
		normalized = append(normalized, entry)
	}
	return normalized, found
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
		if common.EventHasManagedHook(hooks, event, hookCommandMarker) {
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
