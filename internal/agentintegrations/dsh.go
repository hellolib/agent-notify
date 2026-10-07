package agentintegrations

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/hellolib/agent-notify/internal/dshhooks"
)

// DshIntegration implements Integration for DeepSeek Harness.
//
// DSH is the one agent here that is not configured by editing a hooks file: its
// plugin is an npm package installed into a profile, and DSH's loader resolves
// it from that profile's `dsh.profile.bundles` list. So the interface maps onto
// package management — SettingsPath yields the profile manifest (where
// registration is recorded) and Install/Uninstall drive `dsh plugin`.
type DshIntegration struct{}

// NewDshIntegration creates a new DeepSeek Harness integration.
func NewDshIntegration() *DshIntegration {
	return &DshIntegration{}
}

// Name returns the display name for DeepSeek Harness.
func (d *DshIntegration) Name() string {
	return "DeepSeek Harness"
}

// installTimeout bounds the package-manager call. `dsh plugin add` forwards to
// pnpm and may hit the network, so the budget is generous; the point is to fail
// with a clear message instead of hanging the setup wizard forever.
const installTimeout = 10 * time.Minute

// DetectInstalled 检查 DSH 是否可用。
//
// 三个信号，任一命中即视为已安装：
//  1. AGENT_NOTIFY_DSH_BIN 显式指向 dsh（从源码 checkout 启动的常见形态）；
//  2. PATH 上有 dsh；
//  3. DSH 家目录（$DSH_HOME 或 ~/.dsh）已存在——覆盖「装过但当前不在 PATH」。
func (d *DshIntegration) DetectInstalled() bool {
	if strings.TrimSpace(os.Getenv("AGENT_NOTIFY_DSH_BIN")) != "" {
		return true
	}
	if _, err := dshhooks.DSHBinary(); err == nil {
		return true
	}
	home, err := dshhooks.HomeDir()
	if err != nil {
		return false
	}
	info, err := os.Stat(home)
	return err == nil && info.IsDir()
}

// SettingsPath 返回 profile 的 package.json —— 插件的注册落点。
func (d *DshIntegration) SettingsPath(scope string) (string, error) {
	return dshhooks.SettingsPath(scope)
}

// Install 通过 `dsh plugin add` 把插件装进 profile。
//
// Integrated 接口没有 context 参数，此处补一个带超时的 background context：
// 安装是用户主动触发的向导动作，不存在需要透传的上游取消信号。
func (d *DshIntegration) Install(settingsPath, binaryPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), installTimeout)
	defer cancel()
	return dshhooks.Install(ctx, settingsPath, binaryPath)
}

// Uninstall 通过 `dsh plugin remove` 从 profile 卸载插件。
func (d *DshIntegration) Uninstall(settingsPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), installTimeout)
	defer cancel()
	return dshhooks.Uninstall(ctx, settingsPath)
}

// IsHookInstalled 检查 profile 的 bundles 列表里是否已注册本插件。
func (d *DshIntegration) IsHookInstalled(settingsPath string) (bool, error) {
	return dshhooks.IsInstalled(settingsPath)
}

var _ Integration = (*DshIntegration)(nil)
