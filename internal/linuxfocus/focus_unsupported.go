//go:build !linux

package linuxfocus

import (
	"context"
	"errors"
)

type Request struct {
	Title    string
	Body     string
	WindowID string
	Icon     string
}

func ResolveWindowID(context.Context, int) (string, error) {
	return "", errors.New("linux focus is only supported on linux")
}

func CaptureActiveWindow(context.Context) (string, error) {
	return "", errors.New("linux focus is only supported on linux")
}

func StartDetached(context.Context, Request) error {
	return errors.New("linux focus is only supported on linux")
}

func WaitNotifyAndFocus(context.Context, Request) error {
	return errors.New("linux focus is only supported on linux")
}

func SendNotification(context.Context, string, string, string) error {
	return errors.New("linux focus is only supported on linux")
}

func CommandPath(name string) string {
	return name
}
