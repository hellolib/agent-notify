package dshhooks

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildInstallPlanReportsProfileAndCommands(t *testing.T) {
	home := t.TempDir()
	t.Setenv(dshHomeEnv, home)
	t.Setenv(profileEnv, "web")
	t.Setenv(specEnv, "")

	plan := BuildInstallPlan()

	if plan.Profile != "web" {
		t.Errorf("Profile = %q, want web", plan.Profile)
	}
	if plan.ProfileDir != filepath.Join(home, "profiles", "web") {
		t.Errorf("ProfileDir = %q, want the profile under DSH_HOME", plan.ProfileDir)
	}
	if plan.ManifestPath != filepath.Join(home, "profiles", "web", "package.json") {
		t.Errorf("ManifestPath = %q, want the profile manifest", plan.ManifestPath)
	}
	if plan.PackageName != pluginPackageName {
		t.Errorf("PackageName = %q, want %q", plan.PackageName, pluginPackageName)
	}
	if plan.InstallSpec != pluginPackageName {
		t.Errorf("InstallSpec = %q, want the package name by default", plan.InstallSpec)
	}
	if plan.Installed {
		t.Error("Installed = true for a profile with no manifest, want false")
	}
}

// TestBuildInstallPlanCommandsAreCopyPasteable 锁住展示用命令串的完整形态。
// 这里曾经把 argv[0] 之外的前两个参数当成前缀跳过，产出 "dsh web add ..."，
// 少了 plugin --profile —— 用户照着敲会失败。
func TestBuildInstallPlanCommandsAreCopyPasteable(t *testing.T) {
	t.Setenv(dshHomeEnv, t.TempDir())
	t.Setenv(profileEnv, "web")
	t.Setenv(specEnv, "")

	plan := BuildInstallPlan()

	wantInstall := "dsh plugin --profile web add " + pluginPackageName
	if plan.InstallCommand != wantInstall {
		t.Errorf("InstallCommand = %q, want %q", plan.InstallCommand, wantInstall)
	}

	wantUninstall := "dsh plugin --profile web remove " + pluginPackageName
	if plan.UninstallCommand != wantUninstall {
		t.Errorf("UninstallCommand = %q, want %q", plan.UninstallCommand, wantUninstall)
	}
}

// TestBuildInstallPlanInstallCommandMatchesRealArgv 直接把展示命令与真正会执行的
// argv 对齐，避免两者再次漂移。
func TestBuildInstallPlanInstallCommandMatchesRealArgv(t *testing.T) {
	t.Setenv(dshHomeEnv, t.TempDir())
	t.Setenv(profileEnv, "web")
	t.Setenv(specEnv, "")

	plan := BuildInstallPlan()

	// pluginArgs returns arguments only (no binary name), so the displayed
	// command is "dsh" plus every argument.
	argv := pluginArgs(ProfileName(), "add", InstallSpec())
	want := "dsh " + strings.Join(argv, " ")
	if plan.InstallCommand != want {
		t.Errorf("InstallCommand = %q, want %q (the argv actually run)", plan.InstallCommand, want)
	}
}

func TestBuildInstallPlanHonoursLinkSpec(t *testing.T) {
	t.Setenv(dshHomeEnv, t.TempDir())
	t.Setenv(specEnv, "link:/tmp/agent-notify-dsh")

	plan := BuildInstallPlan()

	if !strings.HasSuffix(plan.InstallCommand, "link:/tmp/agent-notify-dsh") {
		t.Errorf("InstallCommand = %q, want it to end with the link spec", plan.InstallCommand)
	}
}

// TestBuildInstallPlanReportsAlreadyInstalled 断言已注册时 Installed=true。
func TestBuildInstallPlanReportsAlreadyInstalled(t *testing.T) {
	home := t.TempDir()
	t.Setenv(dshHomeEnv, home)
	t.Setenv(profileEnv, "web")

	dir := filepath.Join(home, "profiles", "web")
	if err := mkdirAll(dir); err != nil {
		t.Fatal(err)
	}
	manifest := `{"dsh":{"profile":{"bundles":["agent-notify-dsh"]}}}`
	if err := writeFile(filepath.Join(dir, "package.json"), manifest); err != nil {
		t.Fatal(err)
	}

	plan := BuildInstallPlan()
	if !plan.Installed {
		t.Error("Installed = false, want true when the bundle is listed")
	}
}
