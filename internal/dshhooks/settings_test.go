package dshhooks

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// stubRunner 记录调用并把预设结果回放给被测代码。
type stubRunner struct {
	calls  [][]string
	output string
	err    error
}

func (s *stubRunner) run(_ context.Context, name string, args ...string) (string, error) {
	s.calls = append(s.calls, append([]string{name}, args...))
	return s.output, s.err
}

// withStubs 替换注入点并在测试结束恢复。
func withStubs(t *testing.T, runner CommandRunner, lookPath func(string) (string, error)) {
	t.Helper()

	origRunner, origLookPath := runCommandFunc, lookPathFunc
	runCommandFunc, lookPathFunc = runner, lookPath
	t.Cleanup(func() {
		runCommandFunc, lookPathFunc = origRunner, origLookPath
	})
}

func TestProfileNameDefaultsToWeb(t *testing.T) {
	t.Setenv(profileEnv, "")
	if got := ProfileName(); got != "web" {
		t.Errorf("ProfileName() = %q, want web", got)
	}
}

func TestProfileNameHonoursEnv(t *testing.T) {
	t.Setenv(profileEnv, "headless")
	if got := ProfileName(); got != "headless" {
		t.Errorf("ProfileName() = %q, want headless", got)
	}
}

// TestHomeDirHonoursDSHHome 锁住 DSH_HOME 的优先级——这是测试隔离与多实例
// 用户的关键开关。
func TestHomeDirHonoursDSHHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(dshHomeEnv, dir)

	got, err := HomeDir()
	if err != nil {
		t.Fatalf("HomeDir() error = %v", err)
	}
	if got != dir {
		t.Errorf("HomeDir() = %q, want %q", got, dir)
	}
}

func TestHomeDirFallsBackToDotDsh(t *testing.T) {
	home := t.TempDir()
	t.Setenv(dshHomeEnv, "")
	t.Setenv("HOME", home)

	got, err := HomeDir()
	if err != nil {
		t.Fatalf("HomeDir() error = %v", err)
	}
	want := filepath.Join(home, ".dsh")
	if got != want {
		t.Errorf("HomeDir() = %q, want %q", got, want)
	}
}

// TestSettingsPathPointsAtProfileManifest 断言「注册落点」是 profile 的
// package.json —— 那是 bundles 列表所在处。
func TestSettingsPathPointsAtProfileManifest(t *testing.T) {
	home := t.TempDir()
	t.Setenv(dshHomeEnv, home)
	t.Setenv(profileEnv, "web")

	got, err := SettingsPath("user")
	if err != nil {
		t.Fatalf("SettingsPath() error = %v", err)
	}
	want := filepath.Join(home, "profiles", "web", "package.json")
	if got != want {
		t.Errorf("SettingsPath() = %q, want %q", got, want)
	}
}

// TestSettingsPathRejectsProjectScope 断言 project scope 明确报错：
// DSH 没有项目级插件安装，返回一个看似有效的路径会误导 clean 逻辑。
func TestSettingsPathRejectsProjectScope(t *testing.T) {
	if _, err := SettingsPath("project"); err == nil {
		t.Fatal("SettingsPath(project) error = nil, want an explanatory error")
	}
}

func TestSettingsPathRejectsUnknownScope(t *testing.T) {
	if _, err := SettingsPath("galaxy"); err == nil {
		t.Fatal("SettingsPath(galaxy) error = nil, want an error")
	}
}

// TestIsInstalledReadsBundles 是核心断言：插件是否注册 = bundles 列表里是否
// 有包名。
func TestIsInstalledReadsBundles(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "registered",
			content: `{"dsh":{"profile":{"bundles":["@deepseek-ai/dsh-base","agent-notify-dsh"]}}}`,
			want:    true,
		},
		{
			name:    "not registered",
			content: `{"dsh":{"profile":{"bundles":["@deepseek-ai/dsh-base"]}}}`,
			want:    false,
		},
		{
			name:    "no bundles key",
			content: `{"name":"dsh-profile-web"}`,
			want:    false,
		},
		{
			name:    "empty file",
			content: "",
			want:    false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "package.json")
			if err := os.WriteFile(path, []byte(c.content), 0o644); err != nil {
				t.Fatal(err)
			}

			got, err := IsInstalled(path)
			if err != nil {
				t.Fatalf("IsInstalled() error = %v", err)
			}
			if got != c.want {
				t.Errorf("IsInstalled() = %v, want %v", got, c.want)
			}
		})
	}
}

// TestIsInstalledMissingFileIsNotAnError 断言文件不存在返回 (false, nil)。
// 「还没装」与「读不出来」在调用方那里是同一个动作（去安装），不该升级成错误。
func TestIsInstalledMissingFileIsNotAnError(t *testing.T) {
	got, err := IsInstalled(filepath.Join(t.TempDir(), "nope", "package.json"))
	if err != nil {
		t.Fatalf("IsInstalled() error = %v, want nil", err)
	}
	if got {
		t.Error("IsInstalled() = true for a missing file, want false")
	}
}

// TestIsInstalledCorruptJSONIsNotAnError 断言坏 JSON 也不报错（同上）。
func TestIsInstalledCorruptJSONIsNotAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "package.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := IsInstalled(path)
	if err != nil {
		t.Fatalf("IsInstalled() error = %v, want nil", err)
	}
	if got {
		t.Error("IsInstalled() = true for corrupt JSON, want false")
	}
}

func TestInstallSpecDefaultsToPackageName(t *testing.T) {
	t.Setenv(specEnv, "")
	if got := InstallSpec(); got != pluginPackageName {
		t.Errorf("InstallSpec() = %q, want %q", got, pluginPackageName)
	}
}

func TestInstallSpecHonoursLinkOverride(t *testing.T) {
	t.Setenv(specEnv, "link:/tmp/plugin")
	if got := InstallSpec(); got != "link:/tmp/plugin" {
		t.Errorf("InstallSpec() = %q, want the override", got)
	}
}

// TestInstallRunsDSHPluginAdd 断言安装就是把正确参数交给 dsh。
func TestInstallRunsDSHPluginAdd(t *testing.T) {
	runner := &stubRunner{}
	withStubs(t, runner.run, func(string) (string, error) { return "/usr/local/bin/dsh", nil })

	t.Setenv(profileEnv, "web")
	t.Setenv(specEnv, "")

	if err := Install(context.Background(), "/ignored/package.json", "/ignored/bin"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	want := []string{"/usr/local/bin/dsh", "plugin", "--profile", "web", "add", "agent-notify-dsh"}
	if len(runner.calls) != 1 {
		t.Fatalf("ran %d commands, want 1: %v", len(runner.calls), runner.calls)
	}
	got := runner.calls[0]
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("ran %v, want %v", got, want)
	}
}

// TestInstallPassesLinkSpec 断言本地开发路径（link:）被原样传递。
func TestInstallPassesLinkSpec(t *testing.T) {
	runner := &stubRunner{}
	withStubs(t, runner.run, func(string) (string, error) { return "dsh", nil })

	t.Setenv(specEnv, "link:/tmp/agent-notify-dsh")

	if err := Install(context.Background(), "", ""); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if len(runner.calls) != 1 || runner.calls[0][5] != "link:/tmp/agent-notify-dsh" {
		t.Errorf("ran %v, want the link spec in the last position", runner.calls)
	}
}

// TestUninstallRunsDSHPluginRemove 断言卸载用包名（而不是可能的 link: 说明符），
// 否则本地链接装上的插件永远卸载不掉。
func TestUninstallRunsDSHPluginRemove(t *testing.T) {
	runner := &stubRunner{}
	withStubs(t, runner.run, func(string) (string, error) { return "dsh", nil })

	t.Setenv(specEnv, "link:/tmp/agent-notify-dsh")

	manifest := filepath.Join(t.TempDir(), "package.json")
	if err := os.WriteFile(manifest, []byte(`{"dsh":{"profile":{"bundles":["agent-notify-dsh"]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Uninstall(context.Background(), manifest); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}

	want := []string{"dsh", "plugin", "--profile", "web", "remove", "agent-notify-dsh"}
	if len(runner.calls) != 1 || strings.Join(runner.calls[0], " ") != strings.Join(want, " ") {
		t.Errorf("ran %v, want %v", runner.calls, want)
	}
}

// TestUninstallSkipsWhenNotRegistered 锁住 clean 的关键场景：profile 里没有
// 注册本插件时卸载必须 no-op——此时连 dsh 二进制都不该去解析（lookPath 故意
// 报错），否则没装 DSH 的机器每次 clean 都会打一行「清理失败」。
func TestUninstallSkipsWhenNotRegistered(t *testing.T) {
	runner := &stubRunner{}
	withStubs(t, runner.run, func(string) (string, error) { return "", errors.New("dsh not on PATH") })

	t.Run("manifest missing", func(t *testing.T) {
		manifest := filepath.Join(t.TempDir(), "package.json")
		if err := Uninstall(context.Background(), manifest); err != nil {
			t.Fatalf("Uninstall() error = %v, want nil", err)
		}
		if len(runner.calls) != 0 {
			t.Fatalf("ran %v, want no command", runner.calls)
		}
	})

	t.Run("not in bundles", func(t *testing.T) {
		manifest := filepath.Join(t.TempDir(), "package.json")
		if err := os.WriteFile(manifest, []byte(`{"dsh":{"profile":{"bundles":["@deepseek-ai/dsh-base"]}}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := Uninstall(context.Background(), manifest); err != nil {
			t.Fatalf("Uninstall() error = %v, want nil", err)
		}
		if len(runner.calls) != 0 {
			t.Fatalf("ran %v, want no command", runner.calls)
		}
	})
}

// TestInstallSurfacesCommandFailure 断言失败时把命令输出带进错误，
// 让上层能给出可操作提示而不是一句「失败」。
func TestInstallSurfacesCommandFailure(t *testing.T) {
	runner := &stubRunner{output: "pnpm: command not found", err: errors.New("exit status 127")}
	withStubs(t, runner.run, func(string) (string, error) { return "dsh", nil })

	err := Install(context.Background(), "", "")
	if err == nil {
		t.Fatal("Install() error = nil, want a failure")
	}
	if !strings.Contains(err.Error(), "pnpm: command not found") {
		t.Errorf("error = %v, want it to carry the command output", err)
	}
}

// TestDSHBinaryPrefersEnvOverride 断言显式配置优先于 PATH。
func TestDSHBinaryPrefersEnvOverride(t *testing.T) {
	withStubs(t, (&stubRunner{}).run, func(string) (string, error) { return "/from/path/dsh", nil })
	t.Setenv(binEnv, "/explicit/dsh")

	got, err := DSHBinary()
	if err != nil {
		t.Fatalf("DSHBinary() error = %v", err)
	}
	if got != "/explicit/dsh" {
		t.Errorf("DSHBinary() = %q, want the explicit override", got)
	}
}

func TestDSHBinaryFallsBackToPath(t *testing.T) {
	withStubs(t, (&stubRunner{}).run, func(string) (string, error) { return "/from/path/dsh", nil })
	t.Setenv(binEnv, "")

	got, err := DSHBinary()
	if err != nil {
		t.Fatalf("DSHBinary() error = %v", err)
	}
	if got != "/from/path/dsh" {
		t.Errorf("DSHBinary() = %q, want the PATH result", got)
	}
}

// TestDSHBinaryMissingIsActionable 断言找不到 dsh 时错误里带安装指引，
// 而不是一句干巴巴的 not found。
func TestDSHBinaryMissingIsActionable(t *testing.T) {
	withStubs(t, (&stubRunner{}).run, func(string) (string, error) { return "", errors.New("not found") })
	t.Setenv(binEnv, "")

	_, err := DSHBinary()
	if err == nil {
		t.Fatal("DSHBinary() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), binEnv) {
		t.Errorf("error = %v, want it to name the override variable", err)
	}
}

// TestInstallRefusesDesktopProfile 断言 desktop profile 被提前拦下并给出可读原因。
// DSH 自己也会拒绝它，但提前拦下能避免用户看到一句来自 pnpm 的晦涩报错。
func TestInstallRefusesDesktopProfile(t *testing.T) {
	runner := &stubRunner{}
	withStubs(t, runner.run, func(string) (string, error) { return "dsh", nil })
	t.Setenv(profileEnv, "desktop")

	err := Install(context.Background(), "", "")
	if err == nil {
		t.Fatal("Install() error = nil, want a refusal for the desktop profile")
	}
	if !strings.Contains(err.Error(), "desktop") {
		t.Errorf("error = %v, want it to name the profile", err)
	}
	if len(runner.calls) != 0 {
		t.Errorf("ran %v, want no command for a refused profile", runner.calls)
	}
}

// TestInstallRefusesWhenDSHMissing 断言 dsh 缺失时不执行任何命令。
func TestInstallRefusesWhenDSHMissing(t *testing.T) {
	runner := &stubRunner{}
	withStubs(t, runner.run, func(string) (string, error) { return "", errors.New("not found") })
	t.Setenv(binEnv, "")

	if err := Install(context.Background(), "", ""); err == nil {
		t.Fatal("Install() error = nil, want a missing-binary error")
	}
	if len(runner.calls) != 0 {
		t.Errorf("ran %v, want no command when dsh is missing", runner.calls)
	}
}
