// Package testutil provides shared helpers for unit tests.
package testutil

import (
	"path/filepath"
	"runtime"
	"testing"
)

// IsolateHome redirects the process home directory used by os.UserHomeDir
// (and therefore config.DefaultPath / agent settings paths) into a temp dir.
//
// On Windows, UserHomeDir prefers USERPROFILE over HOME; setting only HOME
// leaves tests writing into the real user profile (e.g. polluting
// %USERPROFILE%\.agent-notify\config.yaml).
func IsolateHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
		// Prevent HOMEDRIVE+HOMEPATH fallback if USERPROFILE is cleared elsewhere.
		t.Setenv("HOMEDRIVE", filepath.VolumeName(dir))
		t.Setenv("HOMEPATH", dir[len(filepath.VolumeName(dir)):])
	}
	return dir
}
