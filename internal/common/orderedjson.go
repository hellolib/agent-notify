package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Member 是有序 JSON 对象里的一个键值对。Raw 保存值的原始字节。
type Member struct {
	Key string
	Raw json.RawMessage
}

// OrderedObject 是保留键顺序的 JSON 对象。
//
// 存在的理由:Go 的 encoding/json 对 map 强制按键字母排序,且把所有数字
// 解成 float64。用 map[string]any 往返改写用户的 settings.json 会
// (1) 把 9007199254740993 静默改成 ...992、把大整数变成科学计数法、
// 把 1.10 抹成 1.1;(2) 把全文件键序重排,让 dotfiles 用户每次安装都
// 拿到一个全文件 diff。
//
// 值以 json.RawMessage 原样持有,所以我们没动过的子树在重新序列化时
// 逐字节还原——内部键序、数值精度、尾随零一并保留。MarshalIndent 会
// 统一重新缩进,格式跟随文件整体风格。
type OrderedObject []Member

// DecodeOrderedObject 把 data 解析成保序对象。data 不是单个 JSON 对象
// (是数组 / 标量 / 后面还跟着别的内容)时报错。
func DecodeOrderedObject(data []byte) (OrderedObject, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("期望 JSON 对象,实际读到 %v", tok)
	}

	obj := OrderedObject{}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("对象的键不是字符串: %v", keyTok)
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		obj = append(obj, Member{Key: key, Raw: raw})
	}

	// 读掉收尾的 '}'
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	// 一个对象之后不该还有内容
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("JSON 对象之后存在多余内容")
	}
	return obj, nil
}

// MarshalJSON 按成员的当前顺序序列化。值原样写出,不做二次编码。
func (o OrderedObject) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, m := range o {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(m.Key)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		if len(m.Raw) == 0 {
			buf.WriteString("null")
			continue
		}
		buf.Write(m.Raw)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// Get 返回 key 的原始值字节。第二个返回值区分「值是 null」与「键不存在」。
func (o OrderedObject) Get(key string) (json.RawMessage, bool) {
	for _, m := range o {
		if m.Key == key {
			return m.Raw, true
		}
	}
	return nil, false
}

// Set 写入 key。键已存在时原位替换——被改写的键不该因为改写就跑到文件末尾。
// 不存在时追加到末尾。
func (o *OrderedObject) Set(key string, raw json.RawMessage) {
	for i := range *o {
		if (*o)[i].Key == key {
			(*o)[i].Raw = raw
			return
		}
	}
	*o = append(*o, Member{Key: key, Raw: raw})
}

// Delete 移除 key,保持其余成员的相对顺序。key 不存在时是 no-op。
func (o *OrderedObject) Delete(key string) {
	out := (*o)[:0]
	for _, m := range *o {
		if m.Key != key {
			out = append(out, m)
		}
	}
	*o = out
}

// Len 返回成员个数。
func (o OrderedObject) Len() int { return len(o) }

// Keys 按当前顺序返回所有键。
func (o OrderedObject) Keys() []string {
	keys := make([]string, 0, len(o))
	for _, m := range o {
		keys = append(keys, m.Key)
	}
	return keys
}
