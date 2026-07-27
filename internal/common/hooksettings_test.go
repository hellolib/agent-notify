package common

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRefuseNonArrayEventNamesThePathAndShape(t *testing.T) {
	events, err := DecodeOrderedObject([]byte(`{"Stop":{"hooks":[]}}`))
	if err != nil {
		t.Fatal(err)
	}

	err = InstallManagedHooks(&events, []string{"Stop"}, "handle-claude-hook",
		"/bin/an handle-claude-hook", RefuseNonArrayEvent("hooks"))

	if err == nil {
		t.Fatal("非数组的事件值必须让安装失败")
	}
	for _, want := range []string{"hooks.Stop", "对象", "数组"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("错误信息 %q 应当提到 %q", err, want)
		}
	}
}

func TestRefuseNonArrayEventUsesGivenPathPrefix(t *testing.T) {
	events, _ := DecodeOrderedObject([]byte(`{"Stop":"a string"}`))

	err := InstallManagedHooks(&events, []string{"Stop"}, "m", "cmd",
		RefuseNonArrayEvent("hooks.events"))

	if err == nil || !strings.Contains(err.Error(), "hooks.events.Stop") {
		t.Fatalf("错误信息应指向 hooks.events.Stop,实际是 %v", err)
	}
}

func TestInstallManagedHooksLeavesEventsUntouchedWhenRefusing(t *testing.T) {
	original := `{"Stop":{"mine":true},"Notification":[]}`
	events, _ := DecodeOrderedObject([]byte(original))

	_ = InstallManagedHooks(&events, []string{"Stop"}, "m", "cmd", RefuseNonArrayEvent("hooks"))

	out, _ := json.Marshal(events)
	if string(out) != original {
		t.Fatalf("拒绝写入时不应改动 events:\n got %s\nwant %s", out, original)
	}
}

func TestUninstallManagedHooksKeepsNonArrayEvent(t *testing.T) {
	// 卸载不该被用户的无关配置阻塞:非数组里不可能有我们的 entry
	events, _ := DecodeOrderedObject([]byte(
		`{"Stop":{"mine":true},"Notification":[{"hooks":[{"type":"command","command":"/x handle-claude-hook"}]}]}`))

	if err := UninstallManagedHooks(&events, "handle-claude-hook"); err != nil {
		t.Fatalf("卸载不应报错: %v", err)
	}

	raw, ok := events.Get("Stop")
	if !ok || string(raw) != `{"mine":true}` {
		t.Fatalf("非数组事件应原样保留,实际 %s (present=%v)", raw, ok)
	}
	if _, ok := events.Get("Notification"); ok {
		t.Fatal("清空后的 Notification 应被移除")
	}
}
