package notify

import "strings"

// buildWindowsToastXML builds a ToastGeneric XML payload.
// Text is placed in CDATA so CJK characters do not need entity escaping.
func buildWindowsToastXML(title, body, activationType, activationArgs string) string {
	if activationType == "" {
		activationType = "protocol"
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	b.WriteString(`<toast activationType="`)
	b.WriteString(xmlAttrEscape(activationType))
	b.WriteString(`" launch="`)
	b.WriteString(xmlAttrEscape(activationArgs))
	b.WriteString(`" duration="long">`)
	b.WriteString(`<visual><binding template="ToastGeneric">`)
	if title != "" {
		b.WriteString(`<text><![CDATA[`)
		b.WriteString(cdataEscape(title))
		b.WriteString(`]]></text>`)
	}
	// Emit each body line as its own <text> node. A single multi-line CDATA
	// block is legal, but Windows Toast is more reliable with separate lines
	// (especially for CJK workspace paths).
	for _, line := range strings.Split(body, "\n") {
		if line == "" {
			continue
		}
		b.WriteString(`<text><![CDATA[`)
		b.WriteString(cdataEscape(line))
		b.WriteString(`]]></text>`)
	}
	b.WriteString(`</binding></visual>`)
	b.WriteString(`<audio src="ms-winsoundevent:Notification.Default" loop="false" />`)
	b.WriteString(`</toast>`)
	return b.String()
}

func cdataEscape(s string) string {
	// Split CDATA end marker so nested content cannot terminate the section early.
	return strings.ReplaceAll(s, "]]>", "]]]]><![CDATA[>")
}

func xmlAttrEscape(s string) string {
	replacer := strings.NewReplacer(
		`&`, `&amp;`,
		`"`, `&quot;`,
		`<`, `&lt;`,
		`>`, `&gt;`,
	)
	return replacer.Replace(s)
}

func powershellSingleQuote(s string) string {
	// Inside single-quoted PowerShell strings, escape ' as ''.
	return strings.ReplaceAll(s, "'", "''")
}
