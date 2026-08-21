package opencodehooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hellolib/agent-notify/internal/common"
)

// pluginMarker 用于识别本插件写入的 plugin 数组条目。
const pluginMarker = "agent-notify"

// Install 把 agent-notify 插件路径写入 opencode 配置的 plugin 数组，
// 并把插件 JS 文件写到 pluginPath 指定位置。
// 已存在带 marker 的条目则跳过，不覆盖用户自己挂载的其他插件。
func Install(configPath, pluginPath, binaryPath string) error {
	// 1. 写插件 JS 文件
	if err := WritePluginFile(pluginPath, binaryPath); err != nil {
		return err
	}

	// 2. 把 pluginPath 写入 opencode 配置的 plugin 数组
	settings, err := common.ReadOrderedSettings(configPath)
	if err != nil {
		return err
	}

	pluginEntries, err := readPluginArray(settings)
	if err != nil {
		return err
	}

	if !containsPlugin(pluginEntries, pluginPath) {
		encoded, err := json.Marshal(pluginPath)
		if err != nil {
			return err
		}
		pluginEntries = append(pluginEntries, encoded)
		if err := setPluginArray(&settings, pluginEntries); err != nil {
			return err
		}
	}

	return common.WriteOrderedSettings(configPath, settings)
}

// IsInstalled 检查 opencode 配置的 plugin 数组中是否已包含 agent-notify 插件。
func IsInstalled(configPath string) (bool, error) {
	settings, err := common.ReadOrderedSettings(configPath)
	if err != nil {
		return false, err
	}
	entries, err := readPluginArray(settings)
	if err != nil {
		return false, err
	}
	for _, raw := range entries {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil && strings.Contains(s, pluginMarker) {
			return true, nil
		}
	}
	return false, nil
}

// Uninstall 从 opencode 配置的 plugin 数组中移除 agent-notify 插件条目，
// 并删除插件 JS 文件。用户挂在数组里的其他插件原样保留。
func Uninstall(configPath, pluginPath string) error {
	// 1. 从配置中移除插件条目
	if _, err := os.Stat(configPath); err == nil {
		settings, err := common.ReadOrderedSettings(configPath)
		if err != nil {
			return err
		}
		entries, err := readPluginArray(settings)
		if err != nil {
			return err
		}
		kept := make([]json.RawMessage, 0, len(entries))
		for _, raw := range entries {
			var s string
			if err := json.Unmarshal(raw, &s); err == nil {
				if strings.Contains(s, pluginMarker) || s == pluginPath {
					continue
				}
			}
			kept = append(kept, raw)
		}
		if len(kept) == 0 {
			settings.Delete("plugin")
		} else if err := setPluginArray(&settings, kept); err != nil {
			return err
		}
		if err := common.WriteOrderedSettings(configPath, settings); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// 2. 删除插件 JS 文件
	if err := os.Remove(pluginPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// 3. 清理空目录（plugin 所在目录为 ~/.agent-notify 时不删）
	pluginDir := filepath.Dir(pluginPath)
	if entries, derr := os.ReadDir(pluginDir); derr == nil && len(entries) == 0 {
		home, _ := os.UserHomeDir()
		if pluginDir != filepath.Join(home, ".agent-notify") {
			_ = os.Remove(pluginDir)
		}
	}

	return nil
}

// readPluginArray 从 settings 中读取 "plugin" 键的数组。
// 键不存在或为 null 时返回空数组。
func readPluginArray(settings common.OrderedObject) ([]json.RawMessage, error) {
	raw, ok := settings.Get("plugin")
	if !ok {
		return nil, nil
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	var entries []json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &entries); err != nil {
		return nil, fmt.Errorf("plugin %w;无法安全追加 agent-notify 插件,请先手动改成数组形式后重试", err)
	}
	return entries, nil
}

func setPluginArray(settings *common.OrderedObject, entries []json.RawMessage) error {
	encoded, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	settings.Set("plugin", encoded)
	return nil
}

func containsPlugin(entries []json.RawMessage, pluginPath string) bool {
	for _, raw := range entries {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			if s == pluginPath || strings.Contains(s, pluginMarker) {
				return true
			}
		}
	}
	return false
}

// renderPlugin 渲染最终写盘的插件 JS 内容，binaryPath 烘焙进占位符。
// WritePluginFile 与 RefreshPluginIfStale 共用，确保「写入」与「比对」用的是同一份内容。
func renderPlugin(binaryPath string) []byte {
	return []byte(strings.ReplaceAll(PluginJS, "__AGENT_NOTIFY_BINARY__", binaryPath))
}

// WritePluginFile 把插件 JS 内容写到 pluginPath，binaryPath 烘焙进 JS。
func WritePluginFile(pluginPath, binaryPath string) error {
	if err := os.MkdirAll(filepath.Dir(pluginPath), 0o755); err != nil {
		return err
	}
	return common.WriteFileAtomic(pluginPath, renderPlugin(binaryPath), 0o644)
}

// binaryConstRe 匹配插件 JS 里烘焙的二进制路径常量。
var binaryConstRe = regexp.MustCompile(`(?m)^const BINARY = "([^"]*)";$`)

// bakedBinaryPath 从磁盘上的插件 JS 中取出已烘焙的二进制路径。
func bakedBinaryPath(content []byte) (string, bool) {
	m := binaryConstRe.FindSubmatch(content)
	if m == nil {
		return "", false
	}
	return string(m[1]), true
}

// RefreshPluginIfStale 在磁盘上的插件文件与当前二进制内嵌的版本不一致时重写它。
//
// 背景：Install 只在用户重跑向导时调用，而二进制升级（npx 下载新版本）不会触碰
// ~/.agent-notify/opencode-plugin.js。于是新版本新增的订阅事件对存量用户永远不
// 生效——修好的 bug 送不到手上。
//
// 只更新 JS 逻辑，保留文件里已烘焙的二进制路径：重新指定二进制位置是 Install
// 的职责（用户在向导里明确选过）。若改用 os.Executable()，任何碰巧在跑的二进制
// ——dev 构建、go test 的临时二进制——都会把用户的插件劫持到一个转瞬即逝的路径上。
//
// 用内容比对而非版本号：同时覆盖版本升级、本地 dev 构建和文件被截断损坏，且不需要
// 把 Version 从 internal/cli 传进来（cli 已 import 本包，反向引用会形成循环依赖）。
//
// 以下两种情况一律不动文件，交给向导处理：
//   - 文件不存在：用户可能刚卸载过，写回等于让集成「复活」；
//   - 认不出烘焙路径：文件已被改得面目全非，不猜测、不覆盖。
//
// 返回值表示是否实际重写过。OpenCode 仅在启动时加载插件，重写后下次重启生效。
func RefreshPluginIfStale(pluginPath string) (bool, error) {
	actual, err := os.ReadFile(pluginPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	binaryPath, ok := bakedBinaryPath(actual)
	if !ok {
		return false, nil
	}
	if bytes.Equal(actual, renderPlugin(binaryPath)) {
		return false, nil
	}
	return true, WritePluginFile(pluginPath, binaryPath)
}

// BuildPluginSettings 生成用于 print-hooks 命令的插件配置 JSON 结构。
func BuildPluginSettings(binaryPath, pluginPath string) map[string]any {
	return map[string]any{
		"plugin": []string{pluginPath},
	}
}
