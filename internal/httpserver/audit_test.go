package httpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"timeview/internal/auditlog"
)

func TestMutationAuditLogAndList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.jsonl")
	operations, err := auditlog.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer operations.Close()
	s := New()
	s.SetAuditLog(operations)
	stateOf(t, request(s, "POST", "/timer/commands", `{"command":"start"}`, "start", ""))
	failed := request(s, "POST", "/timer/commands", `{"command":"unknown"}`, "failed", "")
	if failed.Code != 422 {
		t.Fatalf("status %d: %s", failed.Code, failed.Body.String())
	}
	stateOf(t, request(s, "PUT", "/timer/message", `{"text":"ログに残さない本文"}`, "message", ""))

	entries, err := operations.Read(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 || entries[0].Action != "timer:start" || entries[1].Result != "failure" || entries[1].ErrorCode != "INVALID_ARGUMENT" || entries[2].Action != "message:update" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "ログに残さない本文") {
		t.Fatal("message text was written to the operation log")
	}

	response := request(s, "GET", "/logs", "", "", "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"action":"message:update"`) {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
}
