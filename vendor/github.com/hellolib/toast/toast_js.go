//go:build js && wasm

package toast

import (
	"syscall/js"
	"time"
)

// WithIcon
//
// The URL of the image used as an icon of the notification
func WithIcon(urlIcon string) NotificationOption {
	return func(n *notification) {
		n._options["icon"] = urlIcon
	}
}

// WithTextDirection
//
// The text direction of the notification
func WithTextDirection(dir TextDirection) NotificationOption {
	return func(n *notification) {
		n._options["dir"] = string(dir)
	}
}

// WithLang
//
// The language code of the notification
func WithLang(lang string) NotificationOption {
	return func(n *notification) {
		n._options["lang"] = lang
	}
}

// WithNotificationID
//
// The ID of the notification (if any)
func WithNotificationID(tag string) NotificationOption {
	return func(n *notification) {
		n._options["tag"] = tag
	}
}

// WithImage
//
// The URL of an image to be displayed as part of the notification
func WithImage(urlImage string) NotificationOption {
	return func(n *notification) {
		n._options["image"] = urlImage
	}
}

// WithRenotify
//
// Specifies whether the user should be notified after a new notification replaces an old one
func WithRenotify(b bool) NotificationOption {
	return func(n *notification) {
		n._options["renotify"] = b
	}
}

// WithRequireInteraction
//
// indicating that a notification should remain active until the user clicks or dismisses it,
// rather than closing automatically.
func WithRequireInteraction(b bool) NotificationOption {
	return func(n *notification) {
		n._options["requireInteraction"] = b
	}
}

// WithSilent
//
// Specifies whether the notification should be silent — i.e., no sounds or vibrations should be issued,
// regardless of the device settings.
func WithSilent(b bool) NotificationOption {
	return func(n *notification) {
		n._options["silent"] = b
	}
}

// WithTimestamp
//
// Specifies the time at which a notification is created or applicable (past, present, or future).
func WithTimestamp(t time.Time) NotificationOption {
	return func(n *notification) {
		n._options["timestamp"] = t.Unix()
	}
}

// WithVibrate
//
// Specifies a vibration pattern for devices with vibration hardware to emit.
func WithVibrate(v []int) NotificationOption {
	return func(n *notification) {
		_v := make([]interface{}, len(v))
		for i := range v {
			_v[i] = v[i]
		}
		n._options["vibrate"] = _v
	}
}

// WithOnClick
//
// A handler for the click event.
// It is triggered each time the user clicks on the notification.
func WithOnClick(fn func(event interface{})) NotificationOption {
	return func(n *notification) {
		n._onClick = fn
	}
}

// WithOnShow
//
// A handler for the show event.
// It is triggered when the notification is displayed.
func WithOnShow(fn func()) NotificationOption {
	return func(n *notification) {
		n._onShow = fn
	}
}

// WithOnClose
//
// A handler for the close event.
// It is triggered when the user closes the notification.
func WithOnClose(fn func()) NotificationOption {
	return func(n *notification) {
		n._onClose = fn
	}
}

// WithOnError
//
// A handler for the error event.
// It is triggered each time the notification encounters an error.
func WithOnError(fn func()) NotificationOption {
	return func(n *notification) {
		n._onError = fn
	}
}

type TextDirection string

const (
	// Auto adopts the browser's language setting behavior (the default.)
	Auto TextDirection = "auto"
	// LTR left to right
	LTR TextDirection = "ltr"
	// RTL right to left
	RTL TextDirection = "rtl"
)

var _ notifier = (*notification)(nil)

func newNotification(message string, opts ...NotificationOption) *notification {
	n := &notification{
		Title:    js.Global().Get("location").Get("href").String(),
		Message:  message,
		_options: make(map[string]interface{}, 16),
	}
	for _, fn := range opts {
		fn(n)
	}
	return n
}

func (n *notification) push() error {
	// check if the browser supports notifications
	if !isSupported() {
		alert("This browser does not support desktop notification")
		return nil
	}
	// check whether notification permissions have already been granted
	if isGranted() {
		n.createNotification()
		return nil
	}
	// need to ask the user for permission
	if !isDenied() {
		// If the user accepts, let's create a notification
		js.Global().Get("Notification").
			Call("requestPermission").
			Call("then",
				js.FuncOf(func(this js.Value, args []js.Value) interface{} {
					n.createNotification()
					return nil
				}),
			)
	}
	return nil
}

func (n *notification) createNotification() {
	notify := js.Global().Get("Notification").New(n.Title, js.ValueOf(n.generateOptions()))
	if n._onClick != nil {
		notify.Set("onclick", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			n._onClick(this)
			return nil
		}))
	}
	if n._onShow != nil {
		notify.Set("onshow", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			n._onShow()
			return nil
		}))
	}
	if n._onClose != nil {
		notify.Set("onclose", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			n._onClose()
			return nil
		}))
	}
	if n._onError != nil {
		notify.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			n._onError()
			return nil
		}))
	}
}

func (n *notification) generateOptions() (options map[string]interface{}) {
	options = make(map[string]interface{}, 16)
	options["body"] = n.Message
	for k, v := range n._options {
		options[k] = v
	}
	return
}

// https://developer.mozilla.org/en-US/docs/Web/API/notification
type notification struct {
	// The title of the notification
	Title string

	// The body string of the notification
	Message string
	Audio   Audio

	_options map[string]interface{}
	_onClick func(event interface{})
	_onShow  func()
	_onClose func()
	_onError func()
}

func alert(msg string) {
	js.Global().Call("alert", msg)
}

func isSupported() bool {
	return js.Global().Call("hasOwnProperty", "Notification").Bool()
}

func isGranted() bool {
	return js.Global().Get("Notification").Get("permission").String() == "granted"
}

func isDenied() bool {
	return js.Global().Get("Notification").Get("permission").String() == "denied"
}
