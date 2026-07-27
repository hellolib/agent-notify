package common

// TruncateRunes 把 s 限制在 max 个 rune(而非字节)内,超长时以 "..." 结尾。
// 按 rune 截断保证 CJK 多字节字符不被从中切断产生乱码(issue #19 / #33)。
// max <= 3 时直接硬截,不加省略号。
func TruncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}
