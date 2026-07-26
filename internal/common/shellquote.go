package common

import "strings"

// QuotePathForShell 用双引号包裹路径,使含空格的路径在各 agent 执行 hook
// command 的所有 shell 下都是单个 token(issue #30)。
//
// 选双引号而非单引号:cmd.exe 只认双引号,而 POSIX sh / PowerShell / Git Bash
// 同样接受双引号——这是三平台唯一的公共子集。
// POSIX 双引号内 $ ` \ " 仍有特殊含义,逐一反斜杠转义,使语义完全封闭
// (路径来自 os.Executable(),这些字符出现概率极低,但不赌概率)。
func QuotePathForShell(path string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"$", `\$`,
		"`", "\\`",
	)
	return `"` + r.Replace(path) + `"`
}
