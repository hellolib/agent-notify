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
//
// perm 只在 path 不存在时生效;path 已存在则沿用它当前的权限(与 os.WriteFile
// 的 O_CREATE 语义一致)。~/.claude/settings.json 由 Claude Code 建成 0600——
// env 段存的是 ANTHROPIC_API_KEY——调用方统一传的 0644 会把它放宽给同机
// 其它用户,装一次 hook 就泄漏一次。
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	if st, err := os.Stat(path); err == nil {
		perm = st.Mode().Perm()
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
//
// 备份的权限跟随源文件而非 perm:备份内容就是源文件内容,源文件是 0600 时
// 备份若落成 0644,等于把同一份密钥换个文件名泄漏出去。这里写完后显式
// Chmod,好把旧版本以 0644 落下的历史备份一并收紧——只靠 WriteFileAtomic
// 的「已存在则保留」语义会把错误权限锁死。
func WriteFileAtomicWithBackup(path string, data []byte, perm os.FileMode) error {
	if old, err := os.ReadFile(path); err == nil {
		backupPath := path + BackupSuffix
		backupPerm := perm
		if st, statErr := os.Stat(path); statErr == nil {
			backupPerm = st.Mode().Perm()
		}
		if writeErr := WriteFileAtomic(backupPath, old, backupPerm); writeErr == nil {
			_ = os.Chmod(backupPath, backupPerm)
		}
	}
	return WriteFileAtomic(path, data, perm)
}
