package service

import (
	"context"
	"testing"
	"time"

	"waka-personal/internal/domain"
)

func TestParseSummaryWindowLastMonthUsesPreviousCalendarMonth(t *testing.T) {
	loc := time.FixedZone("UTC+7", 7*60*60)
	now := time.Date(2026, time.April, 2, 9, 30, 0, 0, loc)

	window, err := parseSummaryWindow(domain.SummaryQueryParams{Range: "Last Month"}, now, loc)
	if err != nil {
		t.Fatalf("parseSummaryWindow returned error: %v", err)
	}

	expectedStart := time.Date(2026, time.March, 1, 0, 0, 0, 0, loc)
	expectedEnd := time.Date(2026, time.April, 1, 0, 0, 0, 0, loc)

	if !window.startLocal.Equal(expectedStart) {
		t.Fatalf("expected start %s, got %s", expectedStart, window.startLocal)
	}
	if !window.endLocal.Equal(expectedEnd) {
		t.Fatalf("expected end %s, got %s", expectedEnd, window.endLocal)
	}
}

func TestParseSummaryWindowLastYearUsesPreviousCalendarYear(t *testing.T) {
	loc := time.FixedZone("UTC+7", 7*60*60)
	now := time.Date(2026, time.April, 2, 9, 30, 0, 0, loc)

	window, err := parseSummaryWindow(domain.SummaryQueryParams{Range: "Last Year"}, now, loc)
	if err != nil {
		t.Fatalf("parseSummaryWindow returned error: %v", err)
	}

	expectedStart := time.Date(2025, time.January, 1, 0, 0, 0, 0, loc)
	expectedEnd := time.Date(2026, time.January, 1, 0, 0, 0, 0, loc)

	if !window.startLocal.Equal(expectedStart) {
		t.Fatalf("expected start %s, got %s", expectedStart, window.startLocal)
	}
	if !window.endLocal.Equal(expectedEnd) {
		t.Fatalf("expected end %s, got %s", expectedEnd, window.endLocal)
	}
}

func TestParseSummaryWindowCalendarYearRange(t *testing.T) {
	loc := time.FixedZone("UTC+7", 7*60*60)
	now := time.Date(2026, time.June, 21, 9, 30, 0, 0, loc)

	window, err := parseSummaryWindow(domain.SummaryQueryParams{Range: "2026"}, now, loc)
	if err != nil {
		t.Fatalf("parseSummaryWindow returned error: %v", err)
	}

	expectedStart := time.Date(2026, time.January, 1, 0, 0, 0, 0, loc)
	expectedEnd := time.Date(2026, time.June, 22, 0, 0, 0, 0, loc)

	if !window.startLocal.Equal(expectedStart) {
		t.Fatalf("expected start %s, got %s", expectedStart, window.startLocal)
	}
	if !window.endLocal.Equal(expectedEnd) {
		t.Fatalf("expected end %s, got %s", expectedEnd, window.endLocal)
	}
}

func TestParseSummaryWindowCalendarMonthRange(t *testing.T) {
	loc := time.FixedZone("UTC+7", 7*60*60)
	now := time.Date(2026, time.June, 21, 9, 30, 0, 0, loc)

	window, err := parseSummaryWindow(domain.SummaryQueryParams{Range: "2026-05"}, now, loc)
	if err != nil {
		t.Fatalf("parseSummaryWindow returned error: %v", err)
	}

	expectedStart := time.Date(2026, time.May, 1, 0, 0, 0, 0, loc)
	expectedEnd := time.Date(2026, time.June, 1, 0, 0, 0, 0, loc)

	if !window.startLocal.Equal(expectedStart) {
		t.Fatalf("expected start %s, got %s", expectedStart, window.startLocal)
	}
	if !window.endLocal.Equal(expectedEnd) {
		t.Fatalf("expected end %s, got %s", expectedEnd, window.endLocal)
	}
}

func TestParseStatsWindowLastYearUsesPreviousCalendarYear(t *testing.T) {
	loc := time.FixedZone("UTC+7", 7*60*60)
	now := time.Date(2026, time.April, 2, 9, 30, 0, 0, loc)

	window, err := (&QueryService{}).parseStatsWindow(context.Background(), "last_year", "", "", now, loc)
	if err != nil {
		t.Fatalf("parseStatsWindow returned error: %v", err)
	}

	expectedStart := time.Date(2025, time.January, 1, 0, 0, 0, 0, loc)
	expectedEnd := time.Date(2026, time.January, 1, 0, 0, 0, 0, loc)

	if !window.startLocal.Equal(expectedStart) {
		t.Fatalf("expected start %s, got %s", expectedStart, window.startLocal)
	}
	if !window.endLocal.Equal(expectedEnd) {
		t.Fatalf("expected end %s, got %s", expectedEnd, window.endLocal)
	}
}

func TestAIPromptSessionMetrics(t *testing.T) {
	promptLengthA := 301
	promptLengthB := 99
	heartbeats := []domain.HeartbeatRecord{
		{AISession: "session-a", AIPromptLength: &promptLengthA},
		{AISession: "session-a", AIPromptLength: &promptLengthB},
		{AISession: "session-b"},
		{AISession: "  "},
	}

	promptCount, promptChars, sessionCount := aiPromptSessionMetrics(heartbeats)
	if promptCount != 2 {
		t.Fatalf("expected 2 prompts, got %d", promptCount)
	}
	if promptChars != 400 {
		t.Fatalf("expected 400 prompt chars, got %d", promptChars)
	}
	if sessionCount != 2 {
		t.Fatalf("expected 2 sessions, got %d", sessionCount)
	}
}

func TestInferSpecificAIModelParsesLatestModels(t *testing.T) {
	cases := []struct {
		plugin      string
		wantKey     string
		wantDisplay string
	}{
		{"claude-code/3.0.0 opus/5.5", "claude-opus-5-5", "Claude Opus 5.5"},
		{"claude-code/3.0.0 fable/5.1", "claude-fable-5-1", "Claude Fable 5.1"},
		{"claude-code/3.0.0 sonnet/5.5", "claude-sonnet-5-5", "Claude Sonnet 5.5"},
		{"codex-cli/1.0.0 gpt/6.1-sol-high", "gpt-6.1-sol", "GPT-6.1 Sol"},
		{"codex-cli/1.0.0 gpt/6-astra", "gpt-6-astra", "GPT-6 Astra"},
		{"codex-cli/1.0.0 gpt/5.6-luna-xhigh", "gpt-5.6-luna", "GPT-5.6 Luna"},
	}

	for _, tc := range cases {
		key, ok := inferSpecificAIModel(domain.HeartbeatRecord{Plugin: tc.plugin})
		if !ok || key != tc.wantKey {
			t.Fatalf("inferSpecificAIModel(%q) = %q, %v; want %q", tc.plugin, key, ok, tc.wantKey)
		}
		if got := modelDisplayName(key); got != tc.wantDisplay {
			t.Fatalf("modelDisplayName(%q) = %q; want %q", key, got, tc.wantDisplay)
		}
	}
}
