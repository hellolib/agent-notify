//go:build windows

package toast

import (
	"encoding/base64"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// WithAppID
//
// The name of your app. This value shows up in Windows 10's Action Centre, so make it
// something readable for your users. It can contain spaces, however special characters
// (eg. é) are not supported.
func WithAppID(appID string) NotificationOption {
	return func(n *notification) {
		n.AppID = appID
	}
}

// WithIcon
//
// An optional path to an image on the OS to display to the left of the title & message.
func WithIcon(pathIcon string) NotificationOption {
	return func(n *notification) {
		n.Icon = pathIcon
	}
}

func WithIconRaw(raw []byte) NotificationOption {
	return func(n *notification) {
		randBytes := make([]byte, 4)
		_r.Read(randBytes)
		n._tmpIconFilename = filepath.Join(os.TempDir(), fmt.Sprintf("go-toast-logo-%x.png", randBytes))
		if err := os.WriteFile(n._tmpIconFilename, raw, 0600); err != nil {
			return
		}
		n.Icon = n._tmpIconFilename
	}
}

// WithActivationType
//
// The type of notification level action (like Action)
func WithActivationType(activationType string) NotificationOption {
	return func(n *notification) {
		n.ActivationType = activationType
	}
}

// WithActivationArguments
//
// // The activation/action arguments (invoked when the user clicks the notification)
func WithActivationArguments(activationArguments string) NotificationOption {
	return func(n *notification) {
		n.ActivationArguments = activationArguments
	}
}

// WithProtocolAction
//
// Defines an actionable button.
// See https://msdn.microsoft.com/en-us/windows/uwp/controls-and-patterns/tiles-and-notifications-adaptive-interactive-toasts for more info.
//
// Only protocol type action buttons are actually useful, as there's no way of receiving feedback from the
// user's choice. Examples of protocol type action buttons include: "bingmaps:?q=sushi" to open up Windows 10's
// maps app with a pre-populated search field set to "sushi".
//
//	Action{"protocol", "Open Maps", "bingmaps:?q=sushi"}
func WithProtocolAction(label string, arguments ...string) NotificationOption {
	return func(n *notification) {
		if len(n.Actions) == 0 {
			n.Actions = make([]Action, 0, 5)
		}
		if len(n.Actions) == 5 {
			return
		}
		if len(arguments) == 0 {
			arguments = []string{""}
		}
		n.Actions = append(n.Actions, Action{
			Type:      "protocol",
			Label:     label,
			Arguments: arguments[0],
		})
	}
}

// WithAudioLoop
//
// Whether to loop the audio (default false)
func WithAudioLoop(b bool) NotificationOption {
	return func(n *notification) {
		n.Loop = b
	}
}

// WithDuration
//
// How long the notification should show up for (short/long)
func WithDuration(nd NotificationDuration) NotificationOption {
	return func(n *notification) {
		n.Duration = nd
	}
}

func WithLongDuration() NotificationOption {
	return func(n *notification) {
		n.Duration = Long
	}
}

func WithShortDuration() NotificationOption {
	return func(n *notification) {
		n.Duration = Short
	}
}

type NotificationDuration string

const (
	Short NotificationDuration = "short"
	Long  NotificationDuration = "long"
)

const (
	Silent         Audio = "silent"
	Default        Audio = "ms-winsoundevent:Notification.Default"
	IM             Audio = "ms-winsoundevent:Notification.IM"
	Mail           Audio = "ms-winsoundevent:Notification.Mail"
	Reminder       Audio = "ms-winsoundevent:Notification.Reminder"
	SMS            Audio = "ms-winsoundevent:Notification.SMS"
	LoopingAlarm   Audio = "ms-winsoundevent:Notification.Looping.Alarm"
	LoopingAlarm2  Audio = "ms-winsoundevent:Notification.Looping.Alarm2"
	LoopingAlarm3  Audio = "ms-winsoundevent:Notification.Looping.Alarm3"
	LoopingAlarm4  Audio = "ms-winsoundevent:Notification.Looping.Alarm4"
	LoopingAlarm5  Audio = "ms-winsoundevent:Notification.Looping.Alarm5"
	LoopingAlarm6  Audio = "ms-winsoundevent:Notification.Looping.Alarm6"
	LoopingAlarm7  Audio = "ms-winsoundevent:Notification.Looping.Alarm7"
	LoopingAlarm8  Audio = "ms-winsoundevent:Notification.Looping.Alarm8"
	LoopingAlarm9  Audio = "ms-winsoundevent:Notification.Looping.Alarm9"
	LoopingAlarm10 Audio = "ms-winsoundevent:Notification.Looping.Alarm10"
	LoopingCall    Audio = "ms-winsoundevent:Notification.Looping.Call"
	LoopingCall2   Audio = "ms-winsoundevent:Notification.Looping.Call2"
	LoopingCall3   Audio = "ms-winsoundevent:Notification.Looping.Call3"
	LoopingCall4   Audio = "ms-winsoundevent:Notification.Looping.Call4"
	LoopingCall5   Audio = "ms-winsoundevent:Notification.Looping.Call5"
	LoopingCall6   Audio = "ms-winsoundevent:Notification.Looping.Call6"
	LoopingCall7   Audio = "ms-winsoundevent:Notification.Looping.Call7"
	LoopingCall8   Audio = "ms-winsoundevent:Notification.Looping.Call8"
	LoopingCall9   Audio = "ms-winsoundevent:Notification.Looping.Call9"
	LoopingCall10  Audio = "ms-winsoundevent:Notification.Looping.Call10"
)

var _ notifier = (*notification)(nil)

func newNotification(message string, opts ...NotificationOption) *notification {
	n := &notification{
		AppID:          "GO APP",
		Title:          "GO APP",
		Message:        message,
		ActivationType: "protocol",
		Duration:       Short,
		Audio:          Silent,
	}
	for _, fn := range opts {
		fn(n)
	}
	return n
}

func (n *notification) push() error {
	appID := n.AppID
	if appID == "" {
		appID = "GO APP"
	}
	// Register the AppUserModelID (once) with the default icon so the toast's
	// attribution row (the top line, next to the app name) shows the logo.
	if iconPath := materializeDefaultIcon(); iconPath != "" {
		ensureAppIDRegistered(appID, appID, iconPath)
	}

	// appLogoOverride — the large image left of the text — is emitted ONLY when
	// the caller explicitly supplies WithIcon. The default logo lives in the
	// attribution row (via the registered AppUserModelID above), not here.
	iconURI := ""
	if n.Icon != "" {
		iconURI = fileURI(n.Icon)
	}

	xml := buildToastXML(toastContent{
		Title:               n.Title,
		Message:             n.Message,
		IconURI:             iconURI,
		ActivationType:      n.ActivationType,
		ActivationArguments: n.ActivationArguments,
		Audio:               string(n.Audio),
		Loop:                n.Loop,
		Duration:            string(n.Duration),
		Actions:             n.Actions,
	})

	err := pushToastXMLBase64(appID, xml)
	if n._tmpIconFilename != "" {
		_ = os.Remove(n._tmpIconFilename)
	}
	return err
}

// _r seeds temp filenames for WithIconRaw.
var _r = rand.New(rand.NewSource(time.Now().Unix()))

// pushToastXMLBase64 shows a toast by decoding Base64 UTF-8 XML in PowerShell.
//
// Passing the XML payload as Base64 to an ASCII-only -Command script avoids the
// script-file encoding pitfalls that garble CJK title/body on Chinese Windows
// (PowerShell 5.1 mis-decodes a UTF-8 .ps1 written without a BOM).
func pushToastXMLBase64(appID, xml string) error {
	if appID == "" {
		appID = "GO APP"
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(xml))
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

	cmd := exec.Command("powershell.exe",
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

type notification struct {
	// The name of your app. This value shows up in Windows 10's Action Centre, so make it
	// something readable for your users. It can contain spaces, however special characters
	// (eg. é) are not supported.
	AppID string

	// The main title/heading for the notification.
	Title string

	// The single/multi line message to display for the notification.
	Message string

	// An optional path to an image on the OS to display to the left of the title & message.
	Icon             string
	_tmpIconFilename string

	// The type of notification level action (like Action)
	ActivationType string

	// The activation/action arguments (invoked when the user clicks the notification)
	ActivationArguments string

	// Optional action buttons to display below the notification title & message.
	Actions []Action

	// The audio to play when displaying the notification
	Audio Audio

	// Whether to loop the audio (default false)
	Loop bool

	// How long the notification should show up for (short/long)
	Duration NotificationDuration
}
