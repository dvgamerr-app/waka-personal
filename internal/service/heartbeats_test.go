package service

import (
	"testing"
)

func TestParseHeartbeatBody_DoubleEncodedArray(t *testing.T) {
	body := []byte("\"[{\\\"entity\\\":\\\"/tmp/main.go\\\",\\\"time\\\":1710000000,\\\"is_write\\\":true}]\"")
	payloads, err := ParseHeartbeatBody(body)
	if err != nil {
		t.Fatalf("ParseHeartbeatBody returned error: %v", err)
	}
	if len(payloads) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(payloads))
	}
	if payloads[0].Entity != "/tmp/main.go" {
		t.Fatalf("unexpected entity: %s", payloads[0].Entity)
	}
}

func TestNormalizeHeartbeat_UsesAliases(t *testing.T) {
	payloads, err := ParseHeartbeatBody([]byte(`{
		"entity": "/tmp/main.go",
		"time": 1710000000,
		"alternate_project": "waka-personal",
		"lines_in_file": 42,
		"dependencies": "fiber,pgx"
	}`))
	if err != nil {
		t.Fatalf("ParseHeartbeatBody returned error: %v", err)
	}

	record, err := NormalizeHeartbeat(payloads[0], "machine-a", nil)
	if err != nil {
		t.Fatalf("NormalizeHeartbeat returned error: %v", err)
	}
	if record.Project != "waka-personal" {
		t.Fatalf("expected project alias to be used, got %q", record.Project)
	}
	if record.Lines == nil || *record.Lines != 42 {
		t.Fatalf("expected lines alias to be used, got %#v", record.Lines)
	}
	if len(record.Dependencies) != 2 {
		t.Fatalf("expected 2 dependencies, got %d", len(record.Dependencies))
	}
}
