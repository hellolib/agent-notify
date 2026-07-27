package common

import (
	"encoding/json"
	"testing"
)

func TestLenientObject(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantNil bool
	}{
		{"object", `{"error":"x"}`, false},
		{"string", `"failed"`, true},
		{"array", `[{"type":"text"}]`, true},
		{"number", `42`, true},
		{"empty", ``, true},
		{"invalid json", `{broken`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := LenientObject(json.RawMessage(tc.raw))
			if (got == nil) != tc.wantNil {
				t.Fatalf("LenientObject(%s) = %v, wantNil=%v", tc.raw, got, tc.wantNil)
			}
		})
	}
}

func TestLenientBool(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{`true`, true},
		{`false`, false},
		{`"true"`, true},
		{`"false"`, false},
		{``, false},
		{`1`, false},
		{`{bad`, false},
	}
	for _, tc := range cases {
		if got := LenientBool(json.RawMessage(tc.raw)); got != tc.want {
			t.Fatalf("LenientBool(%s) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}
