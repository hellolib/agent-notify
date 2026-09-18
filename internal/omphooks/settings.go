package omphooks

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hellolib/agent-notify/internal/common"
)

const (
	pluginMarker = "agent-notify OMP extension"
	binaryMarker = "__AGENT_NOTIFY_BINARY__"
)

// SettingsPath returns the extension file OMP should auto-discover.
// OMP uses ~/.omp/agent by default, supports profile-specific agent roots,
// and allows PI_CODING_AGENT_DIR to override the user root.
func SettingsPath(scope string) (string, error) {
	switch scope {
	case "user":
		dir, err := userAgentDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "extensions", "agent-notify.ts"), nil
	case "project":
		return filepath.Join(".omp", "extensions", "agent-notify.ts"), nil
	default:
		return "", fmt.Errorf("unsupported scope: %s", scope)
	}
}

func userAgentDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("PI_CODING_AGENT_DIR")); dir != "" {
		return dir, nil
	}

	// Prefer HOME when it is explicitly set. This is the conventional home
	// override in Unix shells and Git Bash, and it also makes CI/test isolation
	// work on Windows where os.UserHomeDir otherwise prefers USERPROFILE.
	home := strings.TrimSpace(os.Getenv("HOME"))
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", err
		}
	}
	profile := strings.TrimSpace(os.Getenv("OMP_PROFILE"))
	if profile == "" {
		profile = strings.TrimSpace(os.Getenv("PI_PROFILE"))
	}
	if profile != "" {
		return filepath.Join(home, ".omp", "profiles", profile, "agent"), nil
	}
	return filepath.Join(home, ".omp", "agent"), nil
}

// Install writes the OMP extension. OMP discovers .ts files directly from
// its extensions directory, so no OMP JSON/YAML configuration is modified.
func Install(path, binaryPath string) error {
	return common.WriteFileAtomic(path, renderPlugin(common.ResolveBinaryPath(binaryPath)), 0o644)
}

// binaryConstRe 匹配 OMP 扩展里烘焙的二进制路径常量（renderPlugin 用 %q 写入引号）。
// 用 \r?$ 而非 $，容忍 Windows 上 git autocrlf 产生的 CRLF 行尾（与 opencodehooks 一致）。
var binaryConstRe = regexp.MustCompile(`(?m)^const BINARY = "([^"]*)";\r?$`)

// BakedBinaryPath 从磁盘上的 OMP 扩展源码里取出已烘焙的二进制路径。
// 取不到（文件非本工具写入、格式被改坏）返回 ("", false)。
func BakedBinaryPath(content []byte) (string, bool) {
	m := binaryConstRe.FindSubmatch(content)
	if m == nil {
		return "", false
	}
	return string(m[1]), true
}

// RefreshIfStale 在磁盘上的 OMP 扩展与当前二进制内嵌的版本不一致时重写它。
//
// 背景：Install 只在用户重跑向导 / install-hooks 时调用，而二进制升级（npx 下载
// 新版本）不会触碰 ~/.omp/agent/extensions/agent-notify.ts。于是新版本新增的订阅
// 事件（如 ask 工具 → input_required）、修正的事件映射对存量用户永远不生效——
// 修好的 bug 送不到手上。
//
// 只更新扩展逻辑，保留已烘焙的二进制路径：重新指定二进制位置是 Install 的职责。
// 用内容比对而非版本号：同时覆盖升级、dev 构建和文件损坏，且不需要把 Version 传进来。
//
// 以下两种情况一律不动文件，交给向导处理：
//   - 文件不存在：用户可能刚卸载过，写回等于让集成「复活」；
//   - 认不出烘焙路径：文件已被改得面目全非，不猜测、不覆盖。
//
// 返回值表示是否实际重写过。OMP 仅在启动时加载扩展，重写后下次 OMP 会话生效。
func RefreshIfStale(extPath string) (bool, error) {
	actual, err := os.ReadFile(extPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	binaryPath, ok := BakedBinaryPath(actual)
	if !ok {
		return false, nil
	}
	if bytes.Equal(actual, renderPlugin(binaryPath)) {
		return false, nil
	}
	return true, Install(extPath, binaryPath)
}

// IsInstalled checks that the target file is an agent-notify-owned extension.
func IsInstalled(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.Contains(string(data), pluginMarker), nil
}

// Uninstall removes only an extension containing our ownership marker.
func Uninstall(path string) error {
	installed, err := IsInstalled(path)
	if err != nil || !installed {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	cleanEmptyParents(path)
	return nil
}

func cleanEmptyParents(path string) {
	dir := filepath.Dir(path)
	if entries, err := os.ReadDir(dir); err == nil && len(entries) == 0 {
		_ = os.Remove(dir)
	}
}

func renderPlugin(binaryPath string) []byte {
	// common.ResolveBinaryPath normalizes Windows separators. JSON string
	// escaping is still needed for spaces, quotes, and backslashes.
	quoted := fmt.Sprintf("%q", binaryPath)
	return []byte(strings.ReplaceAll(PluginTS, binaryMarker, quoted))
}

// BuildPluginSettings returns the path in the same shape used by print-hooks
// commands for the other integrations.
func BuildPluginSettings(path string) map[string]any {
	return map[string]any{"extension": path}
}
