package codexhooks

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/hellolib/agent-notify/internal/common"
)

// EnableHooksFeature 确保 config.toml 中 [features] hooks = true,
// 使 Codex CLI 启用 hooks 功能。
//
// config.toml 是用户手工维护的主配置(注释、格式都有价值),因此不做
// 整文件 parse→marshal 往返(会丢掉全部注释与排版,issue #31),而是:
//  1. 用 TOML 解析器**只读**判定当前状态——已启用则零写入直接返回;
//  2. 需要修改时做行级最小编辑:有 [features] 段则在段内替换/插入
//     hooks 一行,无段则在文件末尾追加,其余行原样保留;
//  3. 写入走原子写 + .bak 备份。
//
// features 以内联表(features = { ... })或点键形式存在时,行级编辑无法
// 安全处理,返回错误提示用户手动修改,而不是冒险重写整个文件。
func EnableHooksFeature(configTomlPath string) error {
	data, err := os.ReadFile(configTomlPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// 文件不存在或为空:创建最小内容
	if len(data) == 0 {
		return common.WriteFileAtomic(configTomlPath, []byte("[features]\nhooks = true\n"), 0o644)
	}

	// 只读判定:解析结果仅用于判断,绝不回写
	var config map[string]any
	if err := toml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析 %s 失败(未做任何修改): %w", configTomlPath, err)
	}
	if features, ok := config["features"].(map[string]any); ok {
		if enabled, ok := features["hooks"].(bool); ok && enabled {
			return nil
		}
	}

	lines := strings.Split(string(data), "\n")
	headerIdx := -1
	for i, line := range lines {
		if featuresHeaderRe.MatchString(line) {
			headerIdx = i
			break
		}
	}

	if headerIdx == -1 {
		if _, ok := config["features"]; ok {
			// 解析器看得到 features,文本里却没有 [features] 段头 →
			// 内联表或点键写法,不冒险改写
			return fmt.Errorf("%s 中 features 使用了内联表写法,请手动加入 hooks = true", configTomlPath)
		}
		// 无 [features] 段:末尾追加
		text := string(data)
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += "\n[features]\nhooks = true\n"
		return common.WriteFileAtomicWithBackup(configTomlPath, []byte(text), 0o644)
	}

	// 段的边界:下一个 [xxx] 段头或文件末尾
	sectionEnd := len(lines)
	for i := headerIdx + 1; i < len(lines); i++ {
		if anySectionHeaderRe.MatchString(lines[i]) {
			sectionEnd = i
			break
		}
	}

	// 段内已有 hooks 键(值必为 false,true 已在上面早退):原位替换
	for i := headerIdx + 1; i < sectionEnd; i++ {
		if hooksKeyRe.MatchString(lines[i]) {
			lines[i] = "hooks = true"
			return common.WriteFileAtomicWithBackup(configTomlPath, []byte(strings.Join(lines, "\n")), 0o644)
		}
	}

	// 段内无 hooks 键:紧随段头插入一行
	tail := append([]string{"hooks = true"}, lines[headerIdx+1:]...)
	lines = append(lines[:headerIdx+1], tail...)
	return common.WriteFileAtomicWithBackup(configTomlPath, []byte(strings.Join(lines, "\n")), 0o644)
}

var (
	featuresHeaderRe   = regexp.MustCompile(`^\s*\[features\]\s*(#.*)?$`)
	anySectionHeaderRe = regexp.MustCompile(`^\s*\[`)
	hooksKeyRe         = regexp.MustCompile(`^\s*hooks\s*=`)
)
