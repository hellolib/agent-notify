package common

import (
	"os"
	"path/filepath"
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
