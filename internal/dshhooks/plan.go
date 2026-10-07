package dshhooks

// InstallPlan 描述一次安装会做什么，供 `dsh print-hooks` 输出。
//
// 与其它 agent 不同，DSH 没有"把 hook 条目写进某个 JSON"这回事：安装是一个
// 包管理动作。所以 print-hooks 不去伪造一份 settings JSON，而是如实报告落点、
// 将执行的命令与当前是否已注册——这正是用户在安装前想确认的信息。
type InstallPlan struct {
	// Profile 是目标 DSH profile 名。
	Profile string `json:"profile"`
	// ProfileDir 是 profile 目录的绝对路径。
	ProfileDir string `json:"profile_dir"`
	// ManifestPath 是注册落点（profile 的 package.json）。
	ManifestPath string `json:"manifest_path"`
	// PackageName 是插件包名，也是 bundles 列表里的名字。
	PackageName string `json:"package_name"`
	// InstallSpec 是交给包管理器的说明符（默认包名；本地开发为 link:<路径>）。
	InstallSpec string `json:"install_spec"`
	// InstallCommand 是安装将执行的完整命令。
	InstallCommand string `json:"install_command"`
	// UninstallCommand 是卸载将执行的完整命令。
	UninstallCommand string `json:"uninstall_command"`
	// Installed 表示 manifest 里当前是否已注册该插件。
	Installed bool `json:"installed"`
}

// BuildInstallPlan 汇总当前环境下的安装计划。
//
// 任何一步取不到值（如 HOME 不可解析）都不报错，而是留空字段——print-hooks
// 是诊断命令，给出一份不完整但可读的输出比直接失败更有用。
func BuildInstallPlan() InstallPlan {
	profile := ProfileName()
	plan := InstallPlan{
		Profile:     profile,
		PackageName: pluginPackageName,
		InstallSpec: InstallSpec(),
	}

	if dir, err := ProfileDir(profile); err == nil {
		plan.ProfileDir = dir
	}

	if path, err := SettingsPath("user"); err == nil {
		plan.ManifestPath = path
		plan.Installed, _ = IsInstalled(path)
	}

	// 命令串用于展示，因而用固定的 "dsh" 而不是本机的绝对路径——
	// 用户要照着敲的是 dsh，不是某台机器上的解析结果。
	plan.InstallCommand = displayCommand(pluginArgs(profile, "add", plan.InstallSpec))
	plan.UninstallCommand = displayCommand(pluginArgs(profile, "remove", pluginPackageName))

	return plan
}

// displayCommand 把 pluginArgs 的输出拼成可复制粘贴的一行命令。
//
// 注意 pluginArgs 返回的是「参数」，不含可执行文件名（RunCommand 另行接收 name）。
// 因此这里把字面的 "dsh" 接在全部参数之前，一个都不能跳过——早先误以为 argv[0]
// 是程序名而跳过它，产出的是 "dsh --profile web add …"（丢了 plugin 子命令）。
func displayCommand(argv []string) string {
	out := "dsh"
	for _, arg := range argv {
		out += " " + arg
	}
	return out
}
