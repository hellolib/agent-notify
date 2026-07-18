//go:build windows

package notify

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/hellolib/toast"
)

func defaultWindowsToastPush(ctx context.Context, req windowsToastRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	activationType := "protocol"
	activationArgs := ""
	if req.ClickToFocus {
		if act, diag, err := toast.PrepareFocusActivationVerbose(os.Getppid(), req.LogPath); err == nil {
			activationType = "protocol"
			activationArgs = act.Arguments
			if req.FocusDebug && req.LogPath != "" {
				appendFocusSendDiag(req.LogPath, diag.String())
			}
		}
	}

	xml := buildWindowsToastXML(req.Title, req.Body, activationType, activationArgs)
	return pushWindowsToastXML(ctx, "agent-notify", xml)
}

// appendFocusSendDiag 把 send 诊断写入与 helper 同一日志文件；失败一律吞掉。
func appendFocusSendDiag(path, text string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(text)
}

// pushWindowsToastXML shows a toast by decoding Base64 UTF-8 XML in PowerShell.
//
// hellolib/toast writes a .ps1 via os.WriteFile (UTF-8 without BOM). On Chinese
// Windows, PowerShell 5.1 often mis-decodes that file and garbles title/body.
// Passing the payload as Base64 avoids script-file encoding entirely.
func pushWindowsToastXML(ctx context.Context, appID, xml string) error {
	if appID == "" {
		appID = "agent-notify"
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(xml))
	// Keep the script ASCII-only so the -Command argument never hits code-page issues.
	script := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.UI.Notifications.ToastNotification, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
$xmlText = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'))
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($xmlText)
$toast = New-Object Windows.UI.Notifications.ToastNotification $xml
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('%s').Show($toast)
`, encoded, powershellSingleQuote(appID))

	cmd := exec.CommandContext(ctx, "powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-Command", script,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("windows toast: %w", err)
		}
		return fmt.Errorf("windows toast: %w: %s", err, msg)
	}
	return nil
}
