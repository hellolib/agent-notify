package common

import (
	"encoding/json"
	"testing"
)

func TestDecodeOrderedObjectPreservesKeyOrder(t *testing.T) {
	src := []byte(`{"zzz":1,"permissions":2,"aaa":3,"hooks":{}}`)

	obj, err := DecodeOrderedObject(src)
	if err != nil {
		t.Fatalf("DecodeOrderedObject: %v", err)
	}

	want := []string{"zzz", "permissions", "aaa", "hooks"}
	got := obj.Keys()
	if len(got) != len(want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("keys = %v, want %v", got, want)
		}
	}
}

func TestOrderedObjectRoundTripKeepsNumbersAndNestedOrderByteForByte(t *testing.T) {
	// 普通 map[string]any 往返会把 bigInt 改成 ...992、huge 变成科学计数法、
	// 1.10 变成 1.1,并把所有键重排成字母序。
	src := []byte(`{"zzz":{"inner_z":1,"inner_a":2},"bigInt":9007199254740993,` +
		`"huge":123456789012345678901234,"keep":1.10,"aaa":true}`)

	obj, err := DecodeOrderedObject(src)
	if err != nil {
		t.Fatalf("DecodeOrderedObject: %v", err)
	}
	out, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if string(out) != string(src) {
		t.Fatalf("round-trip changed the document:\n got %s\nwant %s", out, src)
	}
}

func TestOrderedObjectSetReplacesInPlace(t *testing.T) {
	obj, err := DecodeOrderedObject([]byte(`{"a":1,"hooks":{"Stop":[]},"z":2}`))
	if err != nil {
		t.Fatalf("DecodeOrderedObject: %v", err)
	}

	obj.Set("hooks", json.RawMessage(`{"SessionStart":[]}`))

	out, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// hooks 必须留在原位,不能因为被改写就跑到末尾
	want := `{"a":1,"hooks":{"SessionStart":[]},"z":2}`
	if string(out) != want {
		t.Fatalf("got %s, want %s", out, want)
	}
}

func TestOrderedObjectSetAppendsNewKeyAtEnd(t *testing.T) {
	obj, err := DecodeOrderedObject([]byte(`{"a":1,"z":2}`))
	if err != nil {
		t.Fatalf("DecodeOrderedObject: %v", err)
	}

	obj.Set("hooks", json.RawMessage(`{}`))

	out, _ := json.Marshal(obj)
	want := `{"a":1,"z":2,"hooks":{}}`
	if string(out) != want {
		t.Fatalf("got %s, want %s", out, want)
	}
}

func TestOrderedObjectDelete(t *testing.T) {
	obj, err := DecodeOrderedObject([]byte(`{"a":1,"hooks":{},"z":2}`))
	if err != nil {
		t.Fatalf("DecodeOrderedObject: %v", err)
	}

	obj.Delete("hooks")

	if _, ok := obj.Get("hooks"); ok {
		t.Fatal("hooks should be gone")
	}
	out, _ := json.Marshal(obj)
	if string(out) != `{"a":1,"z":2}` {
		t.Fatalf("got %s", out)
	}
	if obj.Len() != 2 {
		t.Fatalf("Len = %d, want 2", obj.Len())
	}
}

func TestOrderedObjectGetReturnsRawBytes(t *testing.T) {
	obj, err := DecodeOrderedObject([]byte(`{"hooks":{"Stop":[{"matcher":"Bash"}]}}`))
	if err != nil {
		t.Fatalf("DecodeOrderedObject: %v", err)
	}

	raw, ok := obj.Get("hooks")
	if !ok {
		t.Fatal("hooks missing")
	}
	if string(raw) != `{"Stop":[{"matcher":"Bash"}]}` {
		t.Fatalf("raw = %s", raw)
	}
	if _, ok := obj.Get("nope"); ok {
		t.Fatal("missing key reported as present")
	}
}

func TestDecodeOrderedObjectEmptyObject(t *testing.T) {
	obj, err := DecodeOrderedObject([]byte(`{}`))
	if err != nil {
		t.Fatalf("DecodeOrderedObject: %v", err)
	}
	if obj.Len() != 0 {
		t.Fatalf("Len = %d, want 0", obj.Len())
	}
	out, _ := json.Marshal(obj)
	if string(out) != `{}` {
		t.Fatalf("got %s, want {}", out)
	}
}

func TestDecodeOrderedObjectRejectsNonObject(t *testing.T) {
	for _, src := range []string{`[1,2]`, `"text"`, `42`, `null`, `true`} {
		if _, err := DecodeOrderedObject([]byte(src)); err == nil {
			t.Fatalf("DecodeOrderedObject(%s) should fail", src)
		}
	}
}

func TestDecodeOrderedObjectRejectsTrailingGarbage(t *testing.T) {
	if _, err := DecodeOrderedObject([]byte(`{"a":1} {"b":2}`)); err == nil {
		t.Fatal("trailing content should fail")
	}
}

func TestOrderedObjectMarshalIndentReindentsNestedRawBytes(t *testing.T) {
	// 未触碰的子树以原始字节透传,但 MarshalIndent 会统一重新缩进——
	// 内部键序与数值保留,格式跟着文件的整体风格走。
	obj, err := DecodeOrderedObject([]byte(`{"env":{"z":9007199254740993,"a":1}}`))
	if err != nil {
		t.Fatalf("DecodeOrderedObject: %v", err)
	}

	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent: %v", err)
	}
	want := "{\n  \"env\": {\n    \"z\": 9007199254740993,\n    \"a\": 1\n  }\n}"
	if string(out) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}
