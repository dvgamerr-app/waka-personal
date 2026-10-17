package store

import (
	"encoding/csv"
	"errors"
	"strings"
	"testing"
	"time"
)

var csvHeader = []string{
	"source_heartbeat_id", "time", "source_created_at", "entity", "type", "category",
	"project", "branch", "language", "project_root_count", "project_folder", "lineno",
	"cursorpos", "lines", "is_write", "is_unsaved_entity", "ai_line_changes",
	"human_line_changes", "machine_name", "source_machine_name_id", "plugin",
	"source_user_agent_id", "dependencies_json", "origin_payload_json",
	"ai_session", "ai_subscription_plan", "ai_input_tokens", "ai_output_tokens",
	"ai_prompt_length",
}

func csvRow(overrides map[string]string) []string {
	row := make([]string, len(csvHeader))
	defaults := map[string]string{
		"source_heartbeat_id": "src-1", "time": "1760000100.5", "entity": "/a.go",
		"type": "file", "category": "coding", "project": "p", "language": "Go",
		"is_write": "true", "ai_input_tokens": "100", "ai_output_tokens": "10",
		"ai_session": "s1", "plugin": "wakatime/2 claude-code/1",
	}
	for i, column := range csvHeader {
		if value, ok := overrides[column]; ok {
			row[i] = value
		} else {
			row[i] = defaults[column]
		}
	}
	return row
}

func TestParseHeartbeatCSVRecordMapsEveryColumn(t *testing.T) {
	id, hash, parsed, err := parseHeartbeatCSVRecord(csvRow(map[string]string{
		"lineno": "7", "ai_line_changes": "-3", "machine_name": "box", "dependencies_json": `["x"]`,
	}), "batch-1")
	if err != nil {
		t.Fatalf("parseHeartbeatCSVRecord returned error: %v", err)
	}
	if id == "" || hash == "" {
		t.Fatalf("expected generated id and hash, got %q %q", id, hash)
	}
	if parsed.Entity != "/a.go" || parsed.Type != "file" || parsed.Category != "coding" || parsed.Project != "p" {
		t.Fatalf("unexpected parsed fields: %#v", parsed)
	}
	if !parsed.IsWrite || parsed.Lineno == nil || *parsed.Lineno != 7 {
		t.Fatalf("unexpected write/lineno: %#v", parsed)
	}
	if parsed.AILineChanges == nil || *parsed.AILineChanges != -3 {
		t.Fatalf("negative line changes must survive, got %#v", parsed.AILineChanges)
	}
	if parsed.AIInputTokens == nil || *parsed.AIInputTokens != 100 || parsed.ImportBatchID != "batch-1" {
		t.Fatalf("unexpected tokens/batch: %#v", parsed)
	}
	if string(parsed.Dependencies) != `["x"]` || string(parsed.OriginPayload) != "{}" {
		t.Fatalf("unexpected json defaults: %s %s", parsed.Dependencies, parsed.OriginPayload)
	}
	wantTime := time.Unix(1760000100, 500_000_000).UTC()
	if !parsed.Time.Equal(wantTime) {
		t.Fatalf("time = %s, want %s", parsed.Time, wantTime)
	}
}

func TestParseHeartbeatCSVRecordAppliesDefaultsAndRejectsBadInput(t *testing.T) {
	_, _, parsed, err := parseHeartbeatCSVRecord(csvRow(map[string]string{"type": "", "category": "", "dependencies_json": ""}), "b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Type != "file" || parsed.Category != "coding" || string(parsed.Dependencies) != "[]" {
		t.Fatalf("defaults not applied: %#v", parsed)
	}

	for name, row := range map[string][]string{
		"short row":  csvRow(nil)[:10],
		"bad time":   csvRow(map[string]string{"time": "yesterday"}),
		"bad bool":   csvRow(map[string]string{"is_write": "maybe"}),
		"bad int":    csvRow(map[string]string{"lineno": "x"}),
		"bad int64":  csvRow(map[string]string{"ai_input_tokens": "1.5"}),
		"bad create": csvRow(map[string]string{"source_created_at": "soon"}),
	} {
		if _, _, _, err := parseHeartbeatCSVRecord(row, "b"); err == nil {
			t.Fatalf("%s: expected an error", name)
		}
	}
}

func TestCSVScalarParsers(t *testing.T) {
	if got, err := parseOptionalInt(""); err != nil || got != nil {
		t.Fatalf("empty int = %v, %v", got, err)
	}
	if got, err := parseOptionalInt("42"); err != nil || got == nil || *got != 42 {
		t.Fatalf("int = %v, %v", got, err)
	}
	if got, err := parseOptionalInt64("9000000000"); err != nil || got == nil || *got != 9_000_000_000 {
		t.Fatalf("int64 = %v, %v", got, err)
	}
	if got, err := parseOptionalInt64(""); err != nil || got != nil {
		t.Fatalf("empty int64 = %v, %v", got, err)
	}
	for in, want := range map[string]bool{"": false, "TRUE": true, "false": false, "1": true} {
		if got, err := parseOptionalBool(in, false); err != nil || got != want {
			t.Fatalf("parseOptionalBool(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if defaultIfEmpty("", "x") != "x" || defaultIfEmpty("y", "x") != "y" {
		t.Fatal("defaultIfEmpty misbehaves")
	}
	if got, err := parseHeartbeatCSVTime("1760000100"); err != nil || got.Unix() != 1760000100 {
		t.Fatalf("time = %v, %v", got, err)
	}
	if _, err := parseHeartbeatCSVTime("nope"); err == nil {
		t.Fatal("expected a time parse error")
	}
}

func TestHeartbeatCSVSourceStreamsRowsAndReportsErrors(t *testing.T) {
	build := func(rows ...[]string) *csv.Reader {
		var builder strings.Builder
		writer := csv.NewWriter(&builder)
		_ = writer.Write(csvHeader)
		for _, row := range rows {
			_ = writer.Write(row)
		}
		writer.Flush()
		return csv.NewReader(strings.NewReader(builder.String()))
	}

	source, err := newHeartbeatCSVSource(build(csvRow(nil), csvRow(map[string]string{"source_heartbeat_id": "src-2", "entity": "/b.go"})), "batch")
	if err != nil {
		t.Fatalf("newHeartbeatCSVSource returned error: %v", err)
	}
	count := 0
	for source.Next() {
		values, err := source.Values()
		if err != nil || len(values) != 32 {
			t.Fatalf("row %d: %d values, err %v; want the 32 temp-table columns", count, len(values), err)
		}
		count++
	}
	if count != 2 || source.Err() != nil {
		t.Fatalf("streamed %d rows, err %v", count, source.Err())
	}

	bad, err := newHeartbeatCSVSource(build(csvRow(map[string]string{"time": "x"})), "batch")
	if err != nil {
		t.Fatalf("header should be valid: %v", err)
	}
	if bad.Next() || bad.Err() == nil || !strings.Contains(bad.Err().Error(), "row 1") {
		t.Fatalf("expected a row-numbered parse error, got %v", bad.Err())
	}

	if _, err := newHeartbeatCSVSource(csv.NewReader(strings.NewReader("a,b\n1,2\n")), "b"); err == nil {
		t.Fatal("expected a header mismatch error")
	}
	if _, err := newHeartbeatCSVSource(csv.NewReader(strings.NewReader("")), "b"); err == nil {
		t.Fatal("expected an error for an empty file")
	}
}

func TestIsUniqueViolationIgnoresOtherErrors(t *testing.T) {
	if IsUniqueViolation(errors.New("boom")) || IsUniqueViolation(nil) {
		t.Fatal("non-postgres errors are not unique violations")
	}
}
