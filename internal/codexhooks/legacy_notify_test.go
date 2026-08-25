package codexhooks

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseLegacyNotify(t *testing.T) {
	raw := `{
  "type":"agent-turn-complete",
  "thread-id":"019fa95b-1157-7a40-8c92-aeddffad385a",
  "turn-id":"turn-1",
  "cwd":"C:\\work\\demo",
  "client":"codex-vscode",
  "input-messages":["fix it"],
  "last-assistant-message":"cargo build succeeds"
}`

	msg, err := ParseLegacyNotify(raw)
	if err != nil {
		t.Fatalf("ParseLegacyNotify() error = %v", err)
	}
	if msg.Agent != "codex" || msg.Event != "run_completed" {
		t.Fatalf("message = %+v, want codex run_completed", msg)
	}
	if msg.SessionID != "019fa95b-1157-7a40-8c92-aeddffad385a" {
		t.Fatalf("SessionID = %q", msg.SessionID)
	}
	if msg.Workspace != `C:\work\demo` {
		t.Fatalf("Workspace = %q", msg.Workspace)
	}
	if !strings.Contains(msg.Body, "cargo build") {
		t.Fatalf("Body = %q", msg.Body)
	}
}

func TestLegacyNotifyMatchesStopHookForDedupe(t *testing.T) {
	legacy, err := ParseLegacyNotify(`{
  "type":"agent-turn-complete",
  "thread-id":"session-1",
  "cwd":"/work/demo",
  "last-assistant-message":"build succeeds"
}`)
	if err != nil {
		t.Fatal(err)
	}
	hook, err := ParseMessage(strings.NewReader(`{
  "hook_event_name":"Stop",
  "session_id":"session-1",
  "cwd":"/work/demo",
  "last_assistant_message":"build succeeds"
}`))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(legacy, hook) {
		t.Fatalf("legacy notify and Stop hook differ:\nlegacy = %+v\nhook   = %+v", legacy, hook)
	}
}

func TestParseLegacyNotifyFallsBackToDefaultBody(t *testing.T) {
	msg, err := ParseLegacyNotify(`{"type":"agent-turn-complete","thread-id":"s","cwd":"/tmp","last-assistant-message":null}`)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Body == "" {
		t.Fatal("Body should fall back to the default")
	}
}

func TestParseLegacyNotifyBuildsCodexDesktopActivationURI(t *testing.T) {
	msg, err := ParseLegacyNotify(`{
  "type":"agent-turn-complete",
  "thread-id":"019ff54e-d894-7d43-9dfc-fa3fb41479e7",
  "client":"Codex Desktop"
}`)
	if err != nil {
		t.Fatal(err)
	}
	if msg.ActivationURI != "codex://threads/019ff54e-d894-7d43-9dfc-fa3fb41479e7" {
		t.Fatalf("ActivationURI = %q", msg.ActivationURI)
	}
}

func TestParseLegacyNotifyOmitsDesktopActivationURIForOtherClients(t *testing.T) {
	for _, client := range []string{"codex-vscode", "codex-tui", ""} {
		raw := `{"type":"agent-turn-complete","thread-id":"thread-1","client":"` + client + `"}`
		msg, err := ParseLegacyNotify(raw)
		if err != nil {
			t.Fatal(err)
		}
		if msg.ActivationURI != "" {
			t.Fatalf("client %q ActivationURI = %q, want empty", client, msg.ActivationURI)
		}
	}
}

func TestParseLegacyNotifyRejectsUnsupportedAndInvalidPayloads(t *testing.T) {
	for _, raw := range []string{
		`not-json`,
		`{"type":"approval-requested"}`,
	} {
		if _, err := ParseLegacyNotify(raw); err == nil {
			t.Fatalf("ParseLegacyNotify(%q) error = nil", raw)
		}
	}
}
