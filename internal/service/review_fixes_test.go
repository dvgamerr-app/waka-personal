package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"waka-personal/internal/domain"
)

func TestParseHeartbeatBodyRejectsEmptyAndNestedEmptyBodies(t *testing.T) {
	for _, body := range []string{"", "   ", `""`, `"  "`, `"\"\""`} {
		_, err := ParseHeartbeatBody([]byte(body))
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("ParseHeartbeatBody(%q) error = %v, want ErrInvalidInput", body, err)
		}
	}
}

func TestNormalizeHeartbeatValidationErrorsAreInvalidInput(t *testing.T) {
	payloads, err := ParseHeartbeatBody([]byte(`{"time":1710000000}`))
	if err != nil {
		t.Fatalf("ParseHeartbeatBody returned error: %v", err)
	}
	if _, err := NormalizeHeartbeat(&payloads[0], "machine", nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("missing entity error = %v, want ErrInvalidInput", err)
	}
}

func TestDaySpanCountsCalendarDaysAcrossDST(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	cases := []struct {
		name       string
		start, end time.Time
		want       int
	}{
		{"spring forward", time.Date(2026, time.March, 5, 0, 0, 0, 0, loc), time.Date(2026, time.March, 12, 0, 0, 0, 0, loc), 7},
		{"fall back", time.Date(2026, time.October, 30, 0, 0, 0, 0, loc), time.Date(2026, time.November, 4, 0, 0, 0, 0, loc), 5},
	}
	for _, tc := range cases {
		if got := daySpan(tc.start, tc.end); got != tc.want {
			t.Fatalf("%s: daySpan = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestSummariesWindowDaysAreUniqueAcrossFallBack(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	now := time.Date(2026, time.November, 3, 12, 0, 0, 0, loc)
	window, err := parseSummaryWindow(domain.SummaryQueryParams{Start: "2026-10-30", End: "2026-11-03"}, now, loc)
	if err != nil {
		t.Fatalf("parseSummaryWindow returned error: %v", err)
	}
	seen := map[string]bool{}
	for day := window.startLocal; day.Before(window.endLocal); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		if seen[key] {
			t.Fatalf("duplicate day %s", key)
		}
		seen[key] = true
	}
	if len(seen) != 5 {
		t.Fatalf("expected 5 days, got %d", len(seen))
	}
}

func TestStatsWindowMatchesSummaryWindowForPresets(t *testing.T) {
	loc := time.FixedZone("UTC+7", 7*60*60)
	now := time.Date(2026, time.October, 17, 9, 30, 0, 0, loc) // Saturday
	for _, preset := range []string{"Last 14 Days", "This Week", "Last 7 Days from Yesterday"} {
		stats, err := (&QueryService{}).parseStatsWindow(context.Background(), preset, "", "", now, loc)
		if err != nil {
			t.Fatalf("parseStatsWindow(%q) returned error: %v", preset, err)
		}
		summary, err := parseSummaryWindow(domain.SummaryQueryParams{Range: preset}, now, loc)
		if err != nil {
			t.Fatalf("parseSummaryWindow(%q) returned error: %v", preset, err)
		}
		if !stats.startLocal.Equal(summary.startLocal) || !stats.endLocal.Equal(summary.endLocal) {
			t.Fatalf("%s: stats window %s..%s differs from summary window %s..%s", preset, stats.startLocal, stats.endLocal, summary.startLocal, summary.endLocal)
		}
	}
}

func TestBuildHeartbeatIntervalsKeepsZeroLengthTelemetry(t *testing.T) {
	base := time.Date(2026, time.October, 17, 1, 0, 0, 0, time.UTC)
	tokens := func(in int64) *int64 { return &in }
	heartbeats := []domain.HeartbeatRecord{
		{Time: base, Entity: "a", AIInputTokens: tokens(1000)},
		{Time: base, Entity: "b", AIInputTokens: tokens(2000)},
		{Time: base.Add(time.Minute), Entity: "c", AIInputTokens: tokens(500)},
	}
	original := append([]domain.HeartbeatRecord(nil), heartbeats...)
	intervals := buildHeartbeatIntervals(heartbeats, 15*time.Minute, base.Add(time.Hour))
	if len(intervals) != 3 {
		t.Fatalf("expected 3 intervals including the zero-length one, got %d", len(intervals))
	}
	for i := range heartbeats {
		if heartbeats[i].Entity != original[i].Entity {
			t.Fatal("buildHeartbeatIntervals must not reorder the caller slice")
		}
	}
	if total := totalIntervalSeconds(intervals); total != 60+15*60 {
		t.Fatalf("unexpected total seconds %v", total)
	}
}

func TestTokenBucketsKeepOneBasisForTokensAndSpend(t *testing.T) {
	base := time.Date(2026, time.October, 17, 1, 0, 0, 0, time.UTC)
	tokens := func(in int64) *int64 { return &in }
	claude := "wakatime/2 claude-code/1 sonnet/5.5"
	heartbeats := []domain.HeartbeatRecord{
		{Time: base, Entity: "a", Plugin: claude, IsWrite: true, AIInputTokens: tokens(1_000_000)},
		{Time: base, Entity: "b", Plugin: claude, IsWrite: true, AIInputTokens: tokens(2_000_000)},
		{Time: base.Add(time.Minute), Entity: "c", Plugin: claude, IsWrite: false, AIInputTokens: tokens(4_000_000)},
		{Time: base.Add(2 * time.Minute), Entity: "d", AIInputTokens: tokens(1_000_000)}, // no model attribution
	}
	pricing := func(key string) (float64, float64) {
		if key == "claude-sonnet-5-5" {
			return 2, 10
		}
		return 3, 15
	}
	totals := accumulateTokenTotals(heartbeats, aiModelBucketValue, pricing)

	var inputTokens int64
	var cents float64
	for _, total := range totals {
		inputTokens += total.inputTokens
		cents += total.spendCents
	}
	if want, _ := sumAITokens(heartbeats); inputTokens != want {
		t.Fatalf("model buckets hold %d input tokens, heartbeats hold %d", inputTokens, want)
	}
	// 7M tokens at $2/M plus 1M unattributed at the $3/M default = $17.00.
	if cents != 1700 {
		t.Fatalf("unexpected spend %v cents, want 1700", cents)
	}
	if totals[unknownAIModel] == nil {
		t.Fatal("expected unattributed tokens to land in the Unknown bucket")
	}

	intervals := buildHeartbeatIntervals(filterHeartbeats(heartbeats, true), 15*time.Minute, base.Add(time.Hour))
	items, _ := collectAIBucketData(intervals, 1, aiModelBucketValue, totals)
	var itemTokens int64
	for _, item := range items {
		itemTokens += item["input_tokens"].(int64)
	}
	if itemTokens != inputTokens {
		t.Fatalf("ai_models items report %d input tokens, want %d regardless of writes_only", itemTokens, inputTokens)
	}
}

func TestDuckDBSQLMirrorsAPINormalization(t *testing.T) {
	sql := buildDuckDBSQL("input.json", "output.csv")
	for _, expected := range []string{
		"$.alternate_project",
		"$.alternate_language",
		"$.entity_type",
		"$.lines_in_file",
		"$.timestamp",
		"lower(COALESCE(NULLIF(json_extract_string(hb_json, '$.category')",
	} {
		if !strings.Contains(sql, expected) {
			t.Fatalf("expected importer SQL to contain %q", expected)
		}
	}
}

func TestAuthServiceRejectsEmptyAndWrongCredentials(t *testing.T) {
	auth := NewAuthService("secret")
	for _, header := range []string{"", "Basic", "Basic !!!", "Bearer secret", "Basic d3Jvbmc6"} {
		if err := auth.Validate("", header); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("Validate(%q) = %v, want ErrUnauthorized", header, err)
		}
	}
}
