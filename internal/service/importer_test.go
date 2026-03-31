package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadBackupMetadata(t *testing.T) {
	path := filepath.Join("testdata", "backup-sample.json")
	rawUser, fileRange, err := readBackupMetadata(path)
	if err != nil {
		t.Fatalf("readBackupMetadata returned error: %v", err)
	}
	if len(rawUser) == 0 {
		t.Fatal("expected raw user metadata")
	}
	if fileRange.Start == 0 || fileRange.End == 0 {
		t.Fatalf("expected range values, got %#v", fileRange)
	}
}

func TestMapBackupUserToSnapshot(t *testing.T) {
	path := filepath.Join("testdata", "backup-sample.json")
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}

	var payload struct {
		User json.RawMessage `json:"user"`
	}
	if err := json.Unmarshal(contents, &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	snapshot, err := mapBackupUserToSnapshot(payload.User)
	if err != nil {
		t.Fatalf("mapBackupUserToSnapshot returned error: %v", err)
	}
	if snapshot.Username != "dvgamerr" {
		t.Fatalf("expected username dvgamerr, got %q", snapshot.Username)
	}
	if snapshot.TimeoutMinutes == nil || *snapshot.TimeoutMinutes != 15 {
		t.Fatalf("expected timeout 15, got %#v", snapshot.TimeoutMinutes)
	}
	if snapshot.WritesOnly == nil || !*snapshot.WritesOnly {
		t.Fatalf("expected writes_only=true, got %#v", snapshot.WritesOnly)
	}
}

func TestDuckDBPath(t *testing.T) {
	got := duckDBPath(`E:\tmp\waka's\backup.json.gz`)
	want := "E:/tmp/waka''s/backup.json.gz"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestBuildDuckDBSQLIncludesMaximumObjectSize(t *testing.T) {
	sql := buildDuckDBSQL("input.json", "output.csv")
	expected := "maximum_object_size = 1073741824"
	if !strings.Contains(sql, expected) {
		t.Fatalf("expected SQL to contain %q, got %q", expected, sql)
	}
}
