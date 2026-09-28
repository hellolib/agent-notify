package dshhooks

import "os"

func mkdirAll(dir string) error { return os.MkdirAll(dir, 0o755) }

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
