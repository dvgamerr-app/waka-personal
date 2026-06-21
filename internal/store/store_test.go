package store

import (
	"strings"
	"testing"
)

func TestParseOptionalTimestamp(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "rfc3339",
			value: "2026-03-31T15:04:05Z",
			want:  "2026-03-31T15:04:05Z",
		},
		{
			name:  "legacy without timezone",
			value: "2016-08-06 06:21:41",
			want:  "2016-08-06T06:21:41Z",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseOptionalTimestamp(tc.value)
			if err != nil {
				t.Fatalf("parseOptionalTimestamp returned error: %v", err)
			}
			if got == nil {
				t.Fatal("expected parsed timestamp")
			}
			if got.Format("2006-01-02T15:04:05Z") != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got.Format("2006-01-02T15:04:05Z"))
			}
		})
	}
}

func TestInsertTempImportRowsUpdatesAITelemetryOnConflict(t *testing.T) {
	for _, expected := range []string{
		"SELECT DISTINCT ON (dedupe_hash)",
		"ON CONFLICT (dedupe_hash) DO UPDATE",
		"ai_line_changes = EXCLUDED.ai_line_changes",
		"human_line_changes = EXCLUDED.human_line_changes",
		"ai_session = EXCLUDED.ai_session",
		"ai_input_tokens = EXCLUDED.ai_input_tokens",
		"ai_output_tokens = EXCLUDED.ai_output_tokens",
		"ai_prompt_length = EXCLUDED.ai_prompt_length",
	} {
		if !strings.Contains(insertTempImportRowsQuery, expected) {
			t.Fatalf("expected import query to contain %q", expected)
		}
	}
}

func TestUpsertTempSourceUserAgentsTrustsExportedID(t *testing.T) {
	for _, expected := range []string{
		"INSERT INTO source_user_agents",
		"'local-' || md5(plugin)",
		"UPDATE import_heartbeats_tmp",
		"source_user_agent_id",
		"wakatime-export",
		"ON CONFLICT (id) DO UPDATE",
		"ai_agent_name",
	} {
		if !strings.Contains(upsertTempSourceUserAgentsQuery, expected) {
			t.Fatalf("expected source user agent import query to contain %q", expected)
		}
	}
}
