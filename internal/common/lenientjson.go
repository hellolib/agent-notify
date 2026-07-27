package common

import "encoding/json"

// LenientObject 把 hook 载荷中的多态字段解析为 map。
// agent 的 tool_response / tool_input 依工具而异:对象、字符串、数组都有
// (MCP 工具返回内容数组,Bash 类返回字符串)。把它们声明为 map[string]any
// 会让整条事件在 json.Unmarshal 就失败,通知静默丢失(issue #32)。
// 因此载荷里此类字段应声明为 json.RawMessage,再经本函数按需解析:
// 是 JSON 对象返回 map;是其它类型或解析失败返回 nil,绝不报错。
func LenientObject(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}

// LenientBool 宽容解析布尔字段:接受 true/false 及字符串 "true"/"false";
// 其它形态(数字、缺失、解析失败)返回 false。
func LenientBool(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s == "true"
	}
	return false
}
