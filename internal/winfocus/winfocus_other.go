//go:build !windows

package winfocus

import "errors"

// Capture 仅 Windows 支持。其它平台由各自的聚焦捕获路径处理（linuxfocus / macos），
// 此 stub 只为让跨平台的 dispatch 代码可编译。
func Capture() (string, error) {
	return "", errors.New("winfocus capture is only supported on windows")
}
