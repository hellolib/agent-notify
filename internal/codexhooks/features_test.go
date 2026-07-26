package codexhooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/hellolib/agent-notify/internal/common"
)

func TestEnableHooksFeature_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	if err := EnableHooksFeature(path); err != nil {
		t.Fatalf("EnableHooksFeature() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config.toml: %v", err)
	}

	config := map[string]any{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("failed to parse config.toml: %v", err)
	}

	features, ok := config["features"].(map[string]any)
	if !ok {
		t.Fatal("features section missing or wrong type")
	}
	if features["hooks"] != true {
		t.Fatalf("features.hooks = %v, want true", features["hooks"])
	}
}

func TestEnableHooksFeature_AppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	existing := `model = "gpt-4"
`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnableHooksFeature(path); err != nil {
		t.Fatalf("EnableHooksFeature() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config.toml: %v", err)
	}

	config := map[string]any{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("failed to parse config.toml: %v", err)
	}

	if config["model"] != "gpt-4" {
		t.Fatalf("model = %v, want gpt-4 (existing config should be preserved)", config["model"])
	}

	features, ok := config["features"].(map[string]any)
	if !ok {
		t.Fatal("features section missing or wrong type")
	}
	if features["hooks"] != true {
		t.Fatalf("features.hooks = %v, want true", features["hooks"])
	}
}

func TestEnableHooksFeature_AddsToExistingFeatures(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	existing := `[features]
fast_mode = true
`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnableHooksFeature(path); err != nil {
		t.Fatalf("EnableHooksFeature() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config.toml: %v", err)
	}

	config := map[string]any{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("failed to parse config.toml: %v", err)
	}

	features, ok := config["features"].(map[string]any)
	if !ok {
		t.Fatal("features section missing or wrong type")
	}
	if features["hooks"] != true {
		t.Fatalf("features.hooks = %v, want true", features["hooks"])
	}
	if features["fast_mode"] != true {
		t.Fatalf("features.fast_mode = %v, want true (existing feature should be preserved)", features["fast_mode"])
	}
}

func TestEnableHooksFeature_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	if err := EnableHooksFeature(path); err != nil {
		t.Fatalf("first call error = %v", err)
	}
	if err := EnableHooksFeature(path); err != nil {
		t.Fatalf("second call error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read config.toml: %v", err)
	}

	config := map[string]any{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("failed to parse config.toml: %v", err)
	}

	features, ok := config["features"].(map[string]any)
	if !ok {
		t.Fatal("features section missing or wrong type")
	}
	if features["hooks"] != true {
		t.Fatalf("features.hooks = %v, want true", features["hooks"])
	}
}

// --- issue #31: 定向文本编辑,注释与格式必须原样保留 ---

func TestEnableHooksFeature_PreservesCommentsAndLayout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	existing := `# my codex config
model = "gpt-4"   # pinned on purpose

[mcp_servers.search]
command = "npx"
args = ["-y", "@some/mcp"]

[features]
# experimental switches
fast_mode = true
`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnableHooksFeature(path); err != nil {
		t.Fatalf("EnableHooksFeature() error = %v", err)
	}

	data, _ := os.ReadFile(path)
	text := string(data)

	// 所有注释、原有段落、行内注释必须原样存在
	for _, want := range []string{
		"# my codex config",
		`model = "gpt-4"   # pinned on purpose`,
		"[mcp_servers.search]",
		`args = ["-y", "@some/mcp"]`,
		"# experimental switches",
		"fast_mode = true",
		"hooks = true",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output lost %q:\n%s", want, text)
		}
	}

	// 语义校验:hooks 必须落在 [features] 段内
	config := map[string]any{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("output is not valid TOML: %v", err)
	}
	features := config["features"].(map[string]any)
	if features["hooks"] != true || features["fast_mode"] != true {
		t.Fatalf("features = %v", features)
	}
}

func TestEnableHooksFeature_ReplacesHooksFalseInPlace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	existing := `[features]
hooks = false # disabled while debugging
fast_mode = true
`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnableHooksFeature(path); err != nil {
		t.Fatalf("EnableHooksFeature() error = %v", err)
	}

	data, _ := os.ReadFile(path)
	config := map[string]any{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("output is not valid TOML: %v", err)
	}
	features := config["features"].(map[string]any)
	if features["hooks"] != true {
		t.Fatalf("hooks = %v, want true", features["hooks"])
	}
	if features["fast_mode"] != true {
		t.Fatal("fast_mode lost")
	}
}

func TestEnableHooksFeature_AlreadyEnabledIsZeroWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	existing := "# untouched\n[features]\nhooks = true\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := EnableHooksFeature(path); err != nil {
		t.Fatalf("EnableHooksFeature() error = %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != existing {
		t.Fatalf("file changed on already-enabled config:\n%s", data)
	}
	after, _ := os.Stat(path)
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("file rewritten despite hooks already enabled (want zero-write)")
	}
	// 零写入也不应产生备份
	if _, err := os.Stat(path + common.BackupSuffix); !os.IsNotExist(err) {
		t.Fatal("backup created on zero-write path")
	}
}

func TestEnableHooksFeature_InlineTableRefusesToRewrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	existing := `features = { fast_mode = true }
`
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	err := EnableHooksFeature(path)
	if err == nil {
		t.Fatal("expected error for inline-table features, got nil")
	}

	// 文件必须原封不动
	data, _ := os.ReadFile(path)
	if string(data) != existing {
		t.Fatalf("file modified despite refusal:\n%s", data)
	}
}

func TestEnableHooksFeature_InvalidTomlRefusesToRewrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	existing := "model = \"unclosed\n[features"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnableHooksFeature(path); err == nil {
		t.Fatal("expected parse error, got nil")
	}
	data, _ := os.ReadFile(path)
	if string(data) != existing {
		t.Fatal("file modified despite parse failure")
	}
}

func TestEnableHooksFeature_BacksUpBeforeModifying(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	existing := "model = \"gpt-4\"\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnableHooksFeature(path); err != nil {
		t.Fatalf("EnableHooksFeature() error = %v", err)
	}

	bak, err := os.ReadFile(path + common.BackupSuffix)
	if err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	if string(bak) != existing {
		t.Fatalf("backup = %q, want original", bak)
	}
}
