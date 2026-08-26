package codexhooks

import (
	"encoding/json"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/hellolib/agent-notify/internal/common"
	toml "github.com/pelletier/go-toml/v2"
)

const notifyCommandMarker = "handle-codex-notify"

var rootNotifyKeyRe = regexp.MustCompile(`^\s*notify\s*=`)

// EnsureNotifyCommand 为没有自定义 notify 的用户启用 IDE/app-server 完成通知。
// Codex Desktop 的 turn-ended 命令必须保留在最外层，并通过 --previous-notify 串联
// agent-notify；否则 Desktop 下次启动或更新时会重新接管 notify，导致 IDE 通知失效。
// 其它自定义命令保持不动，避免破坏用户已有 webhook/自动化。
func EnsureNotifyCommand(configTomlPath, binaryPath string) error {
	data, err := os.ReadFile(configTomlPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var parsed map[string]any
	if len(data) > 0 {
		if err := toml.Unmarshal(data, &parsed); err != nil {
			return err
		}
	}

	existing, hasNotify := parsed["notify"]
	if hasNotify && !replaceableNotify(existing) {
		return nil
	}

	binaryPath = common.ResolveBinaryPath(binaryPath)
	line := buildNotifyLine(binaryPath)
	if argv, ok := notifyArgv(existing); hasNotify && ok && desktopNotifyArgv(argv) {
		line = buildDesktopNotifyLine(argv, binaryPath)
	}
	text := string(data)
	lines := strings.Split(text, "\n")
	sectionStart := len(lines)
	for i, current := range lines {
		if anySectionHeaderRe.MatchString(current) {
			sectionStart = i
			break
		}
	}

	for i := 0; i < sectionStart; i++ {
		if rootNotifyKeyRe.MatchString(lines[i]) {
			lines[i] = line
			return common.WriteFileAtomicWithBackup(configTomlPath, []byte(strings.Join(lines, "\n")), 0o644)
		}
	}

	insert := line + "\n"
	if sectionStart == len(lines) {
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += insert
	} else {
		prefix := strings.Join(lines[:sectionStart], "\n")
		suffix := strings.Join(lines[sectionStart:], "\n")
		if prefix != "" && !strings.HasSuffix(prefix, "\n") {
			prefix += "\n"
		}
		text = prefix + insert + suffix
	}
	return common.WriteFileAtomicWithBackup(configTomlPath, []byte(text), 0o644)
}

func RemoveNotifyCommand(configTomlPath string) error {
	data, err := os.ReadFile(configTomlPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var parsed map[string]any
	if err := toml.Unmarshal(data, &parsed); err != nil {
		return err
	}
	value := parsed["notify"]
	removeLine := managedNotify(value)
	replacement, unwrap := removeManagedPreviousNotify(value)
	if !removeLine && !unwrap {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if anySectionHeaderRe.MatchString(line) {
			break
		}
		if rootNotifyKeyRe.MatchString(line) {
			if removeLine {
				lines = append(lines[:i], lines[i+1:]...)
			} else {
				lines[i] = buildNotifyArgvLine(replacement)
			}
			return common.WriteFileAtomicWithBackup(configTomlPath, []byte(strings.Join(lines, "\n")), 0o644)
		}
	}
	return nil
}

func buildNotifyLine(binaryPath string) string {
	return buildNotifyArgvLine([]string{binaryPath, notifyCommandMarker})
}

func buildDesktopNotifyLine(argv []string, binaryPath string) string {
	managedJSON, _ := json.Marshal([]string{binaryPath, notifyCommandMarker})
	wrapped := append([]string{}, argv...)
	for i := 2; i < len(wrapped); i++ {
		if wrapped[i] != "--previous-notify" {
			continue
		}
		if i+1 < len(wrapped) {
			wrapped[i+1] = string(managedJSON)
			return buildNotifyArgvLine(wrapped)
		}
	}
	wrapped = append(wrapped, "--previous-notify", string(managedJSON))
	return buildNotifyArgvLine(wrapped)
}

func buildNotifyArgvLine(argv []string) string {
	quoted := make([]string, len(argv))
	for i, arg := range argv {
		quoted[i] = strconv.Quote(arg)
	}
	return "notify = [ " + strings.Join(quoted, ", ") + " ]"
}

func replaceableNotify(value any) bool {
	argv, ok := notifyArgv(value)
	if !ok {
		return false
	}
	if managedNotifyArgv(argv) {
		return true
	}
	if !desktopNotifyArgv(argv) {
		return false
	}
	previous, present, valid := previousNotifyArgv(argv)
	return !present || (valid && (len(previous) == 0 || managedNotifyArgv(previous)))
}

func managedNotify(value any) bool {
	argv, ok := notifyArgv(value)
	return ok && managedNotifyArgv(argv)
}

func managedNotifyArgv(argv []string) bool {
	return len(argv) >= 2 && argv[1] == notifyCommandMarker
}

func desktopNotifyArgv(argv []string) bool {
	if len(argv) < 2 {
		return false
	}
	executable := path.Base(strings.ReplaceAll(argv[0], `\`, "/"))
	return strings.EqualFold(executable, "codex-computer-use.exe") && argv[1] == "turn-ended"
}

func previousNotifyArgv(argv []string) (previous []string, present, valid bool) {
	for i := 2; i < len(argv); i++ {
		if argv[i] != "--previous-notify" {
			continue
		}
		if i+1 >= len(argv) {
			return nil, true, false
		}
		if err := json.Unmarshal([]byte(argv[i+1]), &previous); err != nil {
			return nil, true, false
		}
		return previous, true, true
	}
	return nil, false, true
}

// removeManagedPreviousNotify 在 Codex Desktop 重新包装托管 notify 后，卸载时保留
// Desktop 自己的 turn-ended 命令，仅去掉 --previous-notify 及其 agent-notify 参数。
func removeManagedPreviousNotify(value any) ([]string, bool) {
	argv, ok := notifyArgv(value)
	if !ok || !desktopNotifyArgv(argv) {
		return nil, false
	}
	previous, present, valid := previousNotifyArgv(argv)
	if !present || !valid || !managedNotifyArgv(previous) {
		return nil, false
	}
	for i := 2; i+1 < len(argv); i++ {
		if argv[i] == "--previous-notify" {
			return append(append([]string{}, argv[:i]...), argv[i+2:]...), true
		}
	}
	return nil, false
}

func notifyArgv(value any) ([]string, bool) {
	items, ok := value.([]any)
	if !ok {
		return nil, false
	}
	argv := make([]string, len(items))
	for i, item := range items {
		arg, ok := item.(string)
		if !ok {
			return nil, false
		}
		argv[i] = arg
	}
	return argv, true
}
