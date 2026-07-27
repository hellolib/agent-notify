package common

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteFileAtomicCreatesFileWithContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.json")

	if err := WriteFileAtomic(path, []byte(`{"a":1}`), 0o644); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(data) != `{"a":1}` {
		t.Fatalf("content = %q", data)
	}
}

func TestWriteFileAtomicOverwritesAndLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := WriteFileAtomic(path, []byte("old"), 0o644); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := WriteFileAtomic(path, []byte("new"), 0o644); err != nil {
		t.Fatalf("second write: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "new" {
		t.Fatalf("content = %q, want new", data)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}

func TestWriteFileAtomicWithBackupKeepsPreviousVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")

	// 首次写入:目标不存在,不应产生备份
	if err := WriteFileAtomicWithBackup(path, []byte("v1"), 0o644); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if _, err := os.Stat(path + BackupSuffix); !os.IsNotExist(err) {
		t.Fatalf("backup should not exist after first write")
	}

	// 第二次写入:备份 v1
	if err := WriteFileAtomicWithBackup(path, []byte("v2"), 0o644); err != nil {
		t.Fatalf("second write: %v", err)
	}
	bak, err := os.ReadFile(path + BackupSuffix)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(bak) != "v1" {
		t.Fatalf("backup = %q, want v1", bak)
	}

	// 第三次写入:备份滚动为 v2(覆盖式,保留上一版)
	if err := WriteFileAtomicWithBackup(path, []byte("v3"), 0o644); err != nil {
		t.Fatalf("third write: %v", err)
	}
	bak, _ = os.ReadFile(path + BackupSuffix)
	if string(bak) != "v2" {
		t.Fatalf("backup = %q, want v2", bak)
	}
	cur, _ := os.ReadFile(path)
	if string(cur) != "v3" {
		t.Fatalf("current = %q, want v3", cur)
	}
}

// skipOnWindows 跳过 POSIX 权限断言:Windows 无权限位,os.Stat 对可写文件
// 恒返回 0666,0600 断言永远不成立(与 config.TestSaveUsesOwnerOnlyPermissions 同理)。
func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not applicable on Windows")
	}
}

func TestWriteFileAtomicPreservesExistingPermissions(t *testing.T) {
	skipOnWindows(t)
	path := filepath.Join(t.TempDir(), "settings.json")

	// 模拟 Claude Code 写下的 ~/.claude/settings.json:0600,因为 env 段存 API key
	if err := os.WriteFile(path, []byte(`{"env":{"ANTHROPIC_API_KEY":"sk-secret"}}`), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := WriteFileAtomic(path, []byte(`{"env":{"ANTHROPIC_API_KEY":"sk-secret"},"hooks":{}}`), 0o644); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}

	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := st.Mode().Perm(); got != 0o600 {
		t.Fatalf("perm = %v, want 0600 (装一次 hook 就把用户的密钥放宽给同机用户)", got)
	}
}

func TestWriteFileAtomicAppliesPermissionsToNewFile(t *testing.T) {
	skipOnWindows(t)
	path := filepath.Join(t.TempDir(), "sub", "fresh.json")

	if err := WriteFileAtomic(path, []byte("{}"), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}

	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := st.Mode().Perm(); got != 0o600 {
		t.Fatalf("perm = %v, want 0600", got)
	}
}

func TestWriteFileAtomicWithBackupGivesBackupTheSourcePermissions(t *testing.T) {
	skipOnWindows(t)
	path := filepath.Join(t.TempDir(), "settings.json")

	if err := os.WriteFile(path, []byte(`{"env":{"KEY":"sk-secret"}}`), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := WriteFileAtomicWithBackup(path, []byte(`{"env":{"KEY":"sk-secret"},"hooks":{}}`), 0o644); err != nil {
		t.Fatalf("WriteFileAtomicWithBackup: %v", err)
	}

	// 备份内容就是原文件内容,同一份密钥换个文件名摆在 0644 上等于没修
	st, err := os.Stat(path + BackupSuffix)
	if err != nil {
		t.Fatalf("stat backup: %v", err)
	}
	if got := st.Mode().Perm(); got != 0o600 {
		t.Fatalf("backup perm = %v, want 0600", got)
	}
}

func TestWriteFileAtomicWithBackupTightensLegacyBackupPermissions(t *testing.T) {
	skipOnWindows(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	if err := os.WriteFile(path, []byte(`{"env":{"KEY":"sk-secret"}}`), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// 旧版本以 0644 落下的备份:本次写入应把它一并收紧,而不是沿用错误权限
	if err := os.WriteFile(path+BackupSuffix, []byte(`{"stale":true}`), 0o644); err != nil {
		t.Fatalf("seed backup: %v", err)
	}

	if err := WriteFileAtomicWithBackup(path, []byte(`{"env":{"KEY":"sk-secret"},"hooks":{}}`), 0o644); err != nil {
		t.Fatalf("WriteFileAtomicWithBackup: %v", err)
	}

	st, err := os.Stat(path + BackupSuffix)
	if err != nil {
		t.Fatalf("stat backup: %v", err)
	}
	if got := st.Mode().Perm(); got != 0o600 {
		t.Fatalf("legacy backup perm = %v, want 0600", got)
	}
}
