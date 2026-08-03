//go:build !linux

package codexhooks

import (
	"fmt"
	"os"
	"time"
)

// FindControllingTTY 在非 Linux 平台返回错误。
func FindControllingTTY() (string, error) {
	return "", fmt.Errorf("TIOCSTI 仅支持 Linux")
}

// InjectKeystroke 在非 Linux 平台返回错误。
func InjectKeystroke(ttyPath string, key byte) error {
	return fmt.Errorf("TIOCSTI 仅支持 Linux")
}

// InjectKeystrokeToTTY 在非 Linux 平台返回错误。
func InjectKeystrokeToTTY(key byte) error {
	return fmt.Errorf("TIOCSTI 仅支持 Linux")
}

// AppendLogSimple 追加日志到指定文件。
func AppendLogSimple(logPath, msg string) error {
	if logPath == "" {
		return nil
	}
	line := fmt.Sprintf("[%s] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}
