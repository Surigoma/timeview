package auditlog

import (
	"path/filepath"
	"testing"
)

func TestWriteAndReadLatestEntries(t *testing.T) {
	log, err := Open(filepath.Join(t.TempDir(), "operations.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	for _, action := range []string{"start", "pause", "reset"} {
		if err := log.Write(Entry{Type: "operation", Action: action, Result: "success"}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := log.Read(2)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Action != "pause" || entries[1].Action != "reset" || entries[0].Time.IsZero() {
		t.Fatalf("unexpected entries: %#v", entries)
	}
}
