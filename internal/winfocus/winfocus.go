// Package winfocus 负责 Windows 点击聚焦的「会话启动抓窗口 + 发送时复核」。
//
// 与 linuxfocus 对称：捕获逻辑住在 agent-notify 侧，仅通过已导出的 toast API
// （EnumerateAncestorWindows / IsUsableWindow / FocusActivationForWindow）复用底层
// Win32 处理，不在 toast 里落地捕获。本文件为跨平台可编译部分（纯 JSON 编解码），
// 系统调用在 winfocus_windows.go，其它平台的 stub 在 winfocus_other.go。
package winfocus

import (
	"encoding/json"
	"strconv"
)

// capture 是 SessionStart 抓到的宿主窗口快照。序列化后存入 state.FocusStore，再经
// Message.FocusCapture 传给 Windows sender。hwnd 用十六进制字符串存：uintptr 可能超出
// JSON number 的安全精度，且十六进制与 anfocus: URI 的 %x 表示一致。
type capture struct {
	HWND  string `json:"hwnd"`
	Title string `json:"title"`
}

// Encode 把 (hwnd, title) 序列化为 FocusStore 存储用的 JSON 快照。
func Encode(hwnd uintptr, title string) string {
	b, err := json.Marshal(capture{HWND: strconv.FormatUint(uint64(hwnd), 16), Title: title})
	if err != nil {
		return ""
	}
	return string(b)
}

// Decode 解析 Encode 产出的 JSON 快照。JSON 非法 / hwnd 非法 / hwnd==0 时 ok=false。
func Decode(s string) (hwnd uintptr, title string, ok bool) {
	var c capture
	if json.Unmarshal([]byte(s), &c) != nil {
		return 0, "", false
	}
	h, err := strconv.ParseUint(c.HWND, 16, 64)
	if err != nil || h == 0 {
		return 0, "", false
	}
	return uintptr(h), c.Title, true
}
