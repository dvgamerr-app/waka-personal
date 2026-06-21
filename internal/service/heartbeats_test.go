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

	record, err := NormalizeHeartbeat(&payloads[0], "machine-a", nil)
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

func TestNormalizeHeartbeat_AITelemetry(t *testing.T) {
	payloads, err := ParseHeartbeatBody([]byte(`{
		"entity": "/tmp/main.go",
		"time": 1710000000,
		"ai_session": "sess-123",
		"ai_subscription_plan": "pro",
		"ai_input_tokens": 1200,
		"ai_output_tokens": 800,
		"ai_prompt_length": 64
	}`))
	if err != nil {
		t.Fatalf("ParseHeartbeatBody returned error: %v", err)
	}

	record, err := NormalizeHeartbeat(&payloads[0], "machine-a", nil)
	if err != nil {
		t.Fatalf("NormalizeHeartbeat returned error: %v", err)
	}
	if record.AISession != "sess-123" || record.AISubscriptionPlan != "pro" {
		t.Fatalf("unexpected ai strings: %q %q", record.AISession, record.AISubscriptionPlan)
	}
	if record.AIInputTokens == nil || *record.AIInputTokens != 1200 {
		t.Fatalf("expected ai_input_tokens 1200, got %#v", record.AIInputTokens)
	}
	if record.AIOutputTokens == nil || *record.AIOutputTokens != 800 {
		t.Fatalf("expected ai_output_tokens 800, got %#v", record.AIOutputTokens)
	}
	if record.AIPromptLength == nil || *record.AIPromptLength != 64 {
		t.Fatalf("expected ai_prompt_length 64, got %#v", record.AIPromptLength)
	}
}

func TestNormalizeHeartbeat_WakaTimeCLIAISyncAliases(t *testing.T) {
	payloads, err := ParseHeartbeatBody([]byte(`[
		{
			"entity": "/tmp/main.go",
			"entity_type": "file",
			"timestamp": 1710000000,
			"alternate_project": "fallback-project",
			"alternate_language": "Go",
			"category": "ai coding",
			"ai_line_changes": -1,
			"ai_session": "session",
			"ai_input_tokens": 0,
			"ai_output_tokens": 7,
			"ai_prompt_length": 53,
			"user_agent": "wakatime/2.19.0 (Windows-amd64) go1.26.0 vscode/1.101.0 vscode-wakatime/24.13.0"
		}
	]`))
	if err != nil {
		t.Fatalf("ParseHeartbeatBody returned error: %v", err)
	}

	record, err := NormalizeHeartbeat(&payloads[0], "machine-a", nil)
	if err != nil {
		t.Fatalf("NormalizeHeartbeat returned error: %v", err)
	}
	if record.Type != "file" {
		t.Fatalf("expected entity_type alias to be used, got %q", record.Type)
	}
	if record.Time.Unix() != 1710000000 {
		t.Fatalf("expected timestamp alias to be used, got %s", record.Time)
	}
	if record.Project != "fallback-project" {
		t.Fatalf("expected alternate project to be used, got %q", record.Project)
	}
	if record.Language != "Go" {
		t.Fatalf("expected alternate language to be used, got %q", record.Language)
	}
	if len(record.Plugin) < 8 || record.Plugin[:8] != "wakatime" {
		t.Fatalf("expected user_agent to be stored as plugin, got %q", record.Plugin)
	}
	if record.AILineChanges == nil || *record.AILineChanges != -1 {
		t.Fatalf("expected ai_line_changes -1, got %#v", record.AILineChanges)
	}
	if record.AIOutputTokens == nil || *record.AIOutputTokens != 7 {
		t.Fatalf("expected ai_output_tokens 7, got %#v", record.AIOutputTokens)
	}
}
