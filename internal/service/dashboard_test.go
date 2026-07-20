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
