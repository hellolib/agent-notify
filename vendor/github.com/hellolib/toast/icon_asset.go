package toast

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
)

// embeddedToastIcon is the default "terminal" logo shown as the Windows toast
// appLogoOverride image when a caller does not supply WithIcon. Regenerate with:
//
//	go run assets/_generate/main.go
//
//go:embed assets/toast-icon.png
var embeddedToastIcon []byte

// defaultToastIconPath is a stable per-user path owned by this library (not the
// consumer's private dir). It lives under the user cache dir (%LOCALAPPDATA% on
// Windows) rather than TempDir so a registered AppUserModelID IconUri keeps
// resolving after temp cleanups.
func defaultToastIconPath() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "hellolib-toast", "toast-icon.png")
}

// writeIconIfAbsent writes data to path only when the file does not already
// exist (idempotent); parent dirs are created.
func writeIconIfAbsent(path string, data []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// materializeDefaultIcon writes the embedded default icon to its stable temp
// path (once) and returns that path; "" on any failure so callers can omit the
// image and still show a logo-less toast.
func materializeDefaultIcon() string {
	p := defaultToastIconPath()
	if err := writeIconIfAbsent(p, embeddedToastIcon); err != nil {
		return ""
	}
	return p
}

// fileURI converts an absolute filesystem path to a file:/// URI. Backslashes
// become forward slashes; every byte except URL-unreserved chars, "/" and ":"
// is percent-encoded (spaces and CJK escaped, the drive-letter colon kept). A
// leading slash is ensured so both "C:/x" and "/x" yield three leading slashes.
func fileURI(path string) string {
	slashed := strings.ReplaceAll(path, `\`, "/")
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	b.WriteString("file://")
	for i := 0; i < len(slashed); i++ {
		c := slashed[i]
		if isUnreservedURIByte(c) || c == '/' || c == ':' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&0x0F])
	}
	return b.String()
}

func isUnreservedURIByte(c byte) bool {
	switch {
	case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		return true
	case c == '-', c == '.', c == '_', c == '~':
		return true
	}
	return false
}
