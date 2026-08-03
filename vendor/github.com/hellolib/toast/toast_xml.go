package toast

import "strings"

// toastContent holds the platform-neutral fields needed to render a Windows
// ToastGeneric XML payload. Keeping it primitive (not the windows-only
// notification struct) makes buildToastXML unit-testable on any OS.
type toastContent struct {
	Title               string
	Message             string
	IconURI             string // file:/// URI for appLogoOverride, or "" for none
	ActivationType      string
	ActivationArguments string
	Audio               string // "" or "silent" => silent
	Loop                bool
	Duration            string // "short" | "long"
	Actions             []Action
}

// buildToastXML renders a ToastGeneric XML payload. Text goes in CDATA so CJK
// needs no entity escaping; attributes are XML-escaped. The payload is UTF-8
// and meant to be pushed as Base64 (never written to a .ps1 file).
func buildToastXML(c toastContent) string {
	activationType := c.ActivationType
	if activationType == "" {
		activationType = "protocol"
	}
	duration := c.Duration
	if duration == "" {
		duration = "short"
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>`)
	b.WriteString(`<toast activationType="`)
	b.WriteString(xmlAttrEscape(activationType))
	b.WriteString(`" launch="`)
	b.WriteString(xmlAttrEscape(c.ActivationArguments))
	b.WriteString(`" duration="`)
	b.WriteString(xmlAttrEscape(duration))
	b.WriteString(`">`)
	b.WriteString(`<visual><binding template="ToastGeneric">`)
	if c.IconURI != "" {
		b.WriteString(`<image placement="appLogoOverride" hint-crop="none" src="`)
		b.WriteString(xmlAttrEscape(c.IconURI))
		b.WriteString(`"/>`)
	}
	if c.Title != "" {
		b.WriteString(`<text><![CDATA[`)
		b.WriteString(cdataEscape(c.Title))
		b.WriteString(`]]></text>`)
	}
	// Each body line becomes its own <text> node — Windows Toast renders CJK
	// multi-line bodies more reliably this way than as one multi-line CDATA.
	for _, line := range strings.Split(c.Message, "\n") {
		if line == "" {
			continue
		}
		b.WriteString(`<text><![CDATA[`)
		b.WriteString(cdataEscape(line))
		b.WriteString(`]]></text>`)
	}
	b.WriteString(`</binding></visual>`)

	if c.Audio == "" || c.Audio == "silent" {
		b.WriteString(`<audio silent="true"/>`)
	} else {
		b.WriteString(`<audio src="`)
		b.WriteString(xmlAttrEscape(c.Audio))
		b.WriteString(`" loop="`)
		if c.Loop {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
		b.WriteString(`"/>`)
	}

	if len(c.Actions) > 0 {
		b.WriteString(`<actions>`)
		for _, a := range c.Actions {
			b.WriteString(`<action activationType="`)
			b.WriteString(xmlAttrEscape(a.Type))
			b.WriteString(`" content="`)
			b.WriteString(xmlAttrEscape(a.Label))
			b.WriteString(`" arguments="`)
			b.WriteString(xmlAttrEscape(a.Arguments))
			b.WriteString(`"/>`)
		}
		b.WriteString(`</actions>`)
	}

	b.WriteString(`</toast>`)
	return b.String()
}

// cdataEscape splits a CDATA end marker so embedded text cannot terminate the
// section early.
func cdataEscape(s string) string {
	return strings.ReplaceAll(s, "]]>", "]]]]><![CDATA[>")
}

func xmlAttrEscape(s string) string {
	return strings.NewReplacer(
		`&`, `&amp;`,
		`"`, `&quot;`,
		`<`, `&lt;`,
		`>`, `&gt;`,
	).Replace(s)
}

// powershellSingleQuote escapes a string for inclusion in a single-quoted
// PowerShell string (used for the AppID in the push command).
func powershellSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
