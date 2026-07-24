package winfocus

import "testing"

func TestEncodeDecode(t *testing.T) {
	encoded := Encode(0x120a3e, "proj – pwsh")
	if encoded == "" {
		t.Fatal("Encode returned empty string")
	}

	hwnd, title, ok := Decode(encoded)
	if !ok {
		t.Fatalf("Decode(%q) ok=false", encoded)
	}
	if hwnd != 0x120a3e || title != "proj – pwsh" {
		t.Fatalf("Decode() = (%#x, %q), want (0x120a3e, %q)", hwnd, title, "proj – pwsh")
	}
}

func TestDecodeRejectsInvalidCapture(t *testing.T) {
	cases := []string{
		"",
		"not-json",
		`{"hwnd":"","title":"x"}`,
		`{"hwnd":"0","title":"x"}`,
		`{"hwnd":"not-hex","title":"x"}`,
	}
	for _, c := range cases {
		if hwnd, title, ok := Decode(c); ok {
			t.Fatalf("Decode(%q) = (%#x,%q,true), want ok=false", c, hwnd, title)
		}
	}
}
