package common

import (
	"io"
	"strings"
	"testing"
)

type probePayload struct {
	HookEventName string `json:"hook_event_name"`
	SessionID     string `json:"session_id"`
}

func TestDecodeHookPayloadReadsWantedFields(t *testing.T) {
	var p probePayload
	err := DecodeHookPayload(strings.NewReader(`{"hook_event_name":"Stop","session_id":"s1"}`), &p)
	if err != nil {
		t.Fatalf("DecodeHookPayload: %v", err)
	}
	if p.HookEventName != "Stop" || p.SessionID != "s1" {
		t.Fatalf("got %+v", p)
	}
}

func TestDecodeHookPayloadSkipsUndeclaredFieldsWithoutAllocating(t *testing.T) {
	// tool_input 没有在 probePayload 里声明:Decoder 应当跳过它,
	// 不为这 4MB 字符串再分配一份。声明过的小字段照常解析。
	big := strings.Repeat("x", 4<<20)
	src := `{"hook_event_name":"Stop","tool_input":"` + big + `","session_id":"s1"}`

	var p probePayload
	if err := DecodeHookPayload(strings.NewReader(src), &p); err != nil {
		t.Fatalf("DecodeHookPayload: %v", err)
	}
	if p.HookEventName != "Stop" || p.SessionID != "s1" {
		t.Fatalf("大字段影响了其它字段的解析: %+v", p)
	}
}

func TestDecodeHookPayloadRejectsOversizedPayloadWithANamedError(t *testing.T) {
	// 超限时给出点名上限的错误,而不是让调用方记成笼统的「解析失败」
	oversized := `{"hook_event_name":"Stop","tool_response":"` +
		strings.Repeat("x", MaxHookPayloadBytes+1024) + `"}`

	var p probePayload
	err := DecodeHookPayload(strings.NewReader(oversized), &p)

	if err == nil {
		t.Fatal("超过上限的 payload 应当报错")
	}
	for _, want := range []string{"上限", "16"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("错误信息 %q 应当提到 %q", err, want)
		}
	}
}

func TestDecodeHookPayloadAcceptsPayloadJustUnderTheLimit(t *testing.T) {
	// 边界:恰好在上限之内的 payload 必须正常解析,不能误伤
	filler := strings.Repeat("x", MaxHookPayloadBytes-128)
	src := `{"hook_event_name":"Stop","tool_response":"` + filler + `","session_id":"s1"}`
	if len(src) > MaxHookPayloadBytes {
		t.Fatalf("测试数据本身超限了: %d", len(src))
	}

	var p probePayload
	if err := DecodeHookPayload(strings.NewReader(src), &p); err != nil {
		t.Fatalf("上限之内的 payload 被拒了: %v", err)
	}
	if p.SessionID != "s1" {
		t.Fatalf("got %+v", p)
	}
}

func TestDecodeHookPayloadSurfacesEmptyInput(t *testing.T) {
	var p probePayload
	err := DecodeHookPayload(strings.NewReader(""), &p)
	if err != io.EOF {
		t.Fatalf("空输入应返回 io.EOF,实际 %v", err)
	}
}

func TestDecodeHookPayloadSurfacesMalformedJSON(t *testing.T) {
	var p probePayload
	if err := DecodeHookPayload(strings.NewReader(`{"hook_event_name":`), &p); err == nil {
		t.Fatal("残缺 JSON 应当报错")
	}
}
