package dshhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DSH 与其它 agent 的接入方式根本不同：它没有「往某个配置文件写 hook 条目」
// 这回事。插件是一个 npm 包，通过 `dsh plugin add` 安装进 profile，
// 由 DSH 的 loader 按 `dsh.profile.bundles` 解析并挂载。
//
// 因此本文件把 Integration 接口映射为包管理操作：
//   - SettingsPath → profile 的 package.json（bundles 列表就是「已注册」的落点）
//   - Install / Uninstall → 转发给 `dsh plugin` 命令
//
// 刻意不手写 profile 的 package.json 或 node_modules：那两者由 pnpm 与
// DSH 的 plugin-manager 记账，手工改动会与其锁文件记账冲突（见开发计划 §4.2）。

const (
	// pluginPackageName 是本插件的 npm 包名，也是 bundles 列表里出现的名字。
	pluginPackageName = "agent-notify-dsh"

	// defaultProfile 是默认安装的 DSH profile。desktop 由 Electron 应用独占
	// 管理（DSH 的 rejectElectronProfile 会直接报错），故不可选。
	defaultProfile = "web"

	// unsupportedProfile 是被 DSH 拒绝的 profile 名，提前拦下给出清晰报错。
	unsupportedProfile = "desktop"

	// dshHomeEnv 覆盖 DSH 的家目录（DSH 自身也用这个名字）。
	dshHomeEnv = "DSH_HOME"

	// profileEnv 覆盖要安装的 profile，便于多 profile 用户与测试。
	profileEnv = "AGENT_NOTIFY_DSH_PROFILE"

	// binEnv 覆盖 dsh 可执行文件路径，便于 dsh 不在 PATH 的场景（如从源码
	// checkout 以 node apps/cli/lib/bin.js 启动）。
	binEnv = "AGENT_NOTIFY_DSH_BIN"

	// specEnv 覆盖安装说明符，便于本地开发用 link:<路径> 安装。
	specEnv = "AGENT_NOTIFY_DSH_PLUGIN_SPEC"
)

// CommandRunner 执行一次外部命令并返回合并输出。抽成变量是为了让测试注入替身，
// 避免单测真的去跑 pnpm / 联网。
type CommandRunner func(ctx context.Context, name string, args ...string) (string, error)

// runCommand 是默认的 CommandRunner 实现。
func runCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runCommandFunc 是测试注入点，形如 notify.detectBundleIDFromProcessTreeFunc。
var runCommandFunc CommandRunner = runCommand

// lookPathFunc 是 exec.LookPath 的测试注入点。
var lookPathFunc = exec.LookPath

// ProfileName 返回要安装的 DSH profile 名。
// 优先级：AGENT_NOTIFY_DSH_PROFILE → 默认 web。
func ProfileName() string {
	if p := strings.TrimSpace(os.Getenv(profileEnv)); p != "" {
		return p
	}
	return defaultProfile
}

// HomeDir 返回 DSH 家目录：$DSH_HOME → ~/.dsh。
func HomeDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv(dshHomeEnv)); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".dsh"), nil
}

// ProfileDir 返回某个 profile 的目录。
func ProfileDir(profile string) (string, error) {
	home, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "profiles", profile), nil
}

// SettingsPath 返回 profile 的 package.json —— 插件的注册落点。
//
// DSH 的 profile 一律位于 DSH 家目录下，没有「项目级」概念，因此 project
// scope 返回错误（调用方会跳过它，见 cli/clean_targets.go）。
func SettingsPath(scope string) (string, error) {
	switch scope {
	case "user":
		dir, err := ProfileDir(ProfileName())
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "package.json"), nil
	case "project":
		return "", fmt.Errorf("DSH has no project scope: plugins install into a profile under the DSH home")
	default:
		return "", fmt.Errorf("unsupported scope: %s", scope)
	}
}

// profileManifest 是 profile package.json 里我们关心的部分。
type profileManifest struct {
	Dsh struct {
		Profile struct {
			Bundles []string `json:"bundles"`
		} `json:"profile"`
	} `json:"dsh"`
}

// IsInstalled 报告 profile 的 bundles 列表里是否已注册本插件。
// profile 目录或 package.json 不存在，或 JSON 坏掉，都返回 (false, nil)：
// 「没装」与「读不出来」在调用方那里是同一个动作（去安装），不该升级成错误。
func IsInstalled(settingsPath string) (bool, error) {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, nil
	}

	var manifest profileManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return false, nil
	}
	return containsString(manifest.Dsh.Profile.Bundles, pluginPackageName), nil
}

// InstallSpec 返回要交给 `dsh plugin add` 的说明符。
// 默认是包名（走 npm）；AGENT_NOTIFY_DSH_PLUGIN_SPEC 可覆盖为 link:<路径>，
// 供本地开发使用。
func InstallSpec() string {
	if spec := strings.TrimSpace(os.Getenv(specEnv)); spec != "" {
		return spec
	}
	return pluginPackageName
}

// DSHBinary 定位 dsh 可执行文件。
//
// 三级：AGENT_NOTIFY_DSH_BIN → PATH 上的 dsh → 报错并给出指引。
// DSH 可以不经全局安装而直接从源码 checkout 启动，因此这里找不到时返回的是
// 带指引的错误而不是静默失败。
func DSHBinary() (string, error) {
	if bin := strings.TrimSpace(os.Getenv(binEnv)); bin != "" {
		return bin, nil
	}
	if path, err := lookPathFunc("dsh"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("dsh not found on PATH: install it (npm install -g @deepseek-ai/dsh) or set %s", binEnv)
}

// pluginArgs 构造 `dsh plugin` 的参数。
func pluginArgs(profile string, sub ...string) []string {
	return append([]string{"plugin", "--profile", profile}, sub...)
}

// Install 通过 `dsh plugin add` 把插件装进 profile。
//
// binaryPath 参数在本接入里刻意不用：插件的二进制定位发生在它自己的运行时
// （AGENT_NOTIFY_BINARY → ~/.agent-notify/agent-notify），而 npx launcher 与
// make local 都落在后者，因此无需把路径烘焙进 profile。
//
// 任何失败都不改动 profile（pnpm 与 DSH 自己负责原子性），只把命令输出回传，
// 以便上层给出可操作的提示。
func Install(ctx context.Context, settingsPath, binaryPath string) error {
	_ = settingsPath // 保留签名一致性；实际落点由 dsh 的 plugin-manager 决定
	_ = binaryPath

	bin, err := DSHBinary()
	if err != nil {
		return err
	}

	profile := ProfileName()
	if strings.EqualFold(profile, unsupportedProfile) {
		return fmt.Errorf("profile %q is managed exclusively by the DSH desktop app; choose another profile (e.g. web)", unsupportedProfile)
	}

	out, err := runCommandFunc(ctx, bin, pluginArgs(profile, "add", InstallSpec())...)
	if err != nil {
		return fmt.Errorf("dsh plugin add failed: %w\n%s", err, strings.TrimSpace(out))
	}
	return nil
}

// Uninstall 通过 `dsh plugin remove` 从 profile 卸载插件。
//
// 若 profile 的 bundles 里没有注册本插件，则直接成功返回。clean 会把
// 所有 agent（包括从未启用过的 DSH）都过一遍卸载，其它 agent 对不存在的
// 文件是 no-op；这里也必须 no-op，否则一台没装 dsh 可执行文件的机器每次
// clean 都会在 DSH 一行报「清理失败」。
func Uninstall(ctx context.Context, settingsPath string) error {
	installed, err := IsInstalled(settingsPath)
	if err != nil {
		return err
	}
	if !installed {
		return nil
	}

	bin, err := DSHBinary()
	if err != nil {
		return err
	}

	profile := ProfileName()
	if strings.EqualFold(profile, unsupportedProfile) {
		return fmt.Errorf("profile %q is managed exclusively by the DSH desktop app; choose another profile (e.g. web)", unsupportedProfile)
	}

	out, err := runCommandFunc(ctx, bin, pluginArgs(profile, "remove", pluginPackageName)...)
	if err != nil {
		return fmt.Errorf("dsh plugin remove failed: %w\n%s", err, strings.TrimSpace(out))
	}
	return nil
}

// containsString 报告 items 是否含 want（精确匹配）。
func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
