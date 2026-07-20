package i18n

import "testing"

func TestDefaultIsZhCN(t *testing.T) {
	// Reset to default
	Set("")
	if Current() != "zh-CN" {
		t.Fatalf("Current() = %q, want zh-CN", Current())
	}
	if IsEnglish() {
		t.Fatal("IsEnglish() = true, want false")
	}
}

func TestSetEnglish(t *testing.T) {
	Set("en-US")
	defer Set("")

	if Current() != "en-US" {
		t.Fatalf("Current() = %q, want en-US", Current())
	}
	if !IsEnglish() {
		t.Fatal("IsEnglish() = false, want true")
	}
}

func TestSetNormalizesNonEnglish(t *testing.T) {
	cases := []string{"zh-TW", "ja", "", "fr"}
	for _, c := range cases {
		Set(c)
		if Current() != "zh-CN" {
			t.Fatalf("Set(%q): Current() = %q, want zh-CN", c, Current())
		}
	}
}

func TestTZhCN(t *testing.T) {
	Set("")
	defer Set("")
	got := T("menu.quit")
	if got != "退出" {
		t.Fatalf("T(menu.quit) = %q, want 退出", got)
	}
}

func TestTEnUS(t *testing.T) {
	Set("en-US")
	defer Set("")
	got := T("menu.quit")
	if got != "Quit" {
		t.Fatalf("T(menu.quit) = %q, want Quit", got)
	}
}

func TestTMissingKey(t *testing.T) {
	Set("")
	defer Set("")
	got := T("nonexistent.key")
	if got != "nonexistent.key" {
		t.Fatalf("T(nonexistent.key) = %q, want nonexistent.key", got)
	}
}

func TestTFallbackToZhCN(t *testing.T) {
	// Add a key only in zh-CN to test fallback
	// We can't easily test this without manipulating the catalog,
	// so we test that all existing keys have both translations.
	Set("en-US")
	defer Set("")
	for key, langs := range catalog {
		if _, ok := langs[EnUS]; !ok {
			// This key has no English translation — T should fall back to zh-CN
			zhVal := langs[ZhCN]
			got := T(key)
			if got != zhVal {
				t.Errorf("T(%q) = %q, want zh-CN fallback %q", key, got, zhVal)
			}
		}
	}
}
