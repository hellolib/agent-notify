package notify

import (
	"strings"
	"testing"
)

func TestBuildWindowsToastXMLContainsChinese(t *testing.T) {
	xml := buildWindowsToastXML("Grok 运行完成", "学习ai编程项目/agent-notify\n任务已完成，请查看结果", "protocol", "")
	if !strings.Contains(xml, "Grok 运行完成") {
		t.Fatalf("xml missing Chinese title: %s", xml)
	}
	if !strings.Contains(xml, "任务已完成，请查看结果") {
		t.Fatalf("xml missing Chinese body: %s", xml)
	}
	if !strings.Contains(xml, "学习ai编程项目/agent-notify") {
		t.Fatalf("xml missing Chinese path segment: %s", xml)
	}
	// Multi-line body should become separate <text> nodes
	if strings.Count(xml, "<text>") < 3 {
		t.Fatalf("expected >=3 text nodes for title+path+body, xml=%s", xml)
	}
	if !strings.Contains(xml, `encoding="utf-8"`) {
		t.Fatalf("xml missing utf-8 declaration: %s", xml)
	}
	if !strings.Contains(xml, "<![CDATA[") {
		t.Fatalf("xml should use CDATA for text: %s", xml)
	}
}

func TestBuildWindowsToastXMLEscapesCDATATerminator(t *testing.T) {
	xml := buildWindowsToastXML("t", "a]]>b", "protocol", "")
	if strings.Contains(xml, "a]]>b") {
		t.Fatalf("raw CDATA terminator must be escaped: %s", xml)
	}
	if !strings.Contains(xml, "a]]]]><![CDATA[>b") {
		t.Fatalf("expected CDATA escape form, got: %s", xml)
	}
}

func TestBuildWindowsToastXMLEscapesAttr(t *testing.T) {
	xml := buildWindowsToastXML("t", "b", `proto"col`, `arg&1`)
	if !strings.Contains(xml, `activationType="proto&quot;col"`) {
		t.Fatalf("activationType not escaped: %s", xml)
	}
	if !strings.Contains(xml, `launch="arg&amp;1"`) {
		t.Fatalf("launch not escaped: %s", xml)
	}
}

func TestPowershellSingleQuote(t *testing.T) {
	if got := powershellSingleQuote(`agent'notify`); got != `agent''notify` {
		t.Fatalf("got %q", got)
	}
}
