package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// MaxHookPayloadBytes 是单个 hook 事件 payload 的上限。
//
// 每个事件起一个进程,而 io.ReadAll 会把整个 stdin 缓冲下来:agent 传来的
// tool_response 可能带着整份文件内容或一整段构建日志,50MB 的 payload 实测
// 要分配 300MB。上限之外还得留余量——json.Decoder 必须把整个顶层对象读进
// 内部 buffer 才开始解码,buffer 翻倍增长导致约 2.5 倍峰值,16MiB 对应约
// 40MB 峰值。真实 payload 远小于此,这道闸只为挡住失控输入。
const MaxHookPayloadBytes = 16 << 20

// DecodeHookPayload 从 r 流式解码一个 hook 事件到 dst。
//
// 用 json.Decoder 而非 io.ReadAll + Unmarshal:payload 结构体里没有声明的
// 字段会被 Decoder 直接跳过,不为其分配——tool_input 这类可能装着整个文件
// 内容的大字段因此不会在内存里落第二份。
//
// 超过上限时给出点名上限的错误,而不是让调用方把它记成笼统的解析失败。
func DecodeHookPayload(r io.Reader, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r, MaxHookPayloadBytes+1))
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return fmt.Errorf("hook payload 超过 %d MiB 上限,已丢弃", MaxHookPayloadBytes>>20)
		}
		return err
	}
	return nil
}
