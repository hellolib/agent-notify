package common

import (
	"os"
	"path/filepath"
)

// BackupSuffix 是改写用户配置前落备份的文件后缀。
// 带 agent-notify 前缀,避免与用户或其它工具的 .bak 撞名。
const BackupSuffix = ".agent-notify.bak"

// WriteFileAtomic 以「同目录临时文件 + rename」原子地写入 path。
// rename 在同一文件系统内是原子操作,写入中断(断电 / kill / 磁盘满)
// 只会留下临时文件,原文件要么是旧内容要么是完整新内容,不会被截断。
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	// 任何一步失败都清理临时文件,避免残留
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

// WriteFileAtomicWithBackup 在原子写入前,把 path 的现有内容备份为
// path + BackupSuffix(覆盖式,始终保留上一版)。用于改写用户的 agent
// 配置文件——即使我们写入了逻辑错误的内容,用户也有一条恢复路径。
// path 不存在时不落备份。备份失败不阻塞写入(备份是尽力而为)。
func WriteFileAtomicWithBackup(path string, data []byte, perm os.FileMode) error {
	if old, err := os.ReadFile(path); err == nil {
		_ = WriteFileAtomic(path+BackupSuffix, old, perm)
	}
	return WriteFileAtomic(path, data, perm)
}
