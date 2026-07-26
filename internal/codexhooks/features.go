package codexhooks

import (
	"os"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/hellolib/agent-notify/internal/common"
)

// EnableHooksFeature 确保 config.toml 中 [features] hooks = true，
// 使 Codex CLI 启用 hooks 功能。如果文件不存在则创建，已有内容则保留并追加。
func EnableHooksFeature(configTomlPath string) error {
	data, err := os.ReadFile(configTomlPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	config := map[string]any{}
	if len(data) > 0 {
		if err := toml.Unmarshal(data, &config); err != nil {
			return err
		}
	}

	features, ok := config["features"].(map[string]any)
	if !ok {
		features = map[string]any{}
	}
	features["hooks"] = true
	config["features"] = features

	out, err := toml.Marshal(config)
	if err != nil {
		return err
	}

	// 用户配置文件:原子写 + 覆盖式 .bak 备份(issue #29)
	return common.WriteFileAtomicWithBackup(configTomlPath, out, 0o644)
}
