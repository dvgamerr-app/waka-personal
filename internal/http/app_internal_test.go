package apihttp

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"waka-personal/internal/service"
)

func TestDashboardStatsRangeMatchesSummaryPresets(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, time.October, 17, 9, 0, 0, 0, loc)
	cases := map[string]string{
		"Last 7 Days":                "last_7_days",
		"Last 30 Days":               "last_30_days",
		"Last 14 Days":               "Last 14 Days",
		"This Week":                  "This Week",
		"Last 7 Days from Yesterday": "Last 7 Days from Yesterday",
		"Last Month":                 "2026-09",
		"Last Year":                  "2025",
		"Today":                      "today",
		"Yesterday":                  "yesterday",
	}
	for rangeParam, want := range cases {
		if got := dashboardStatsRange(rangeParam, now, loc); got != want {
			t.Fatalf("dashboardStatsRange(%q) = %q, want %q", rangeParam, got, want)
		}
	}
}

func TestSpendMetricsOnlyEstimatesWithoutModelData(t *testing.T) {
	withEmptyModels := map[string]any{
		"ai_input_tokens": int64(1_000_000),
		"ai_models":       []map[string]any{},
	}
	if got := spendMetrics(withEmptyModels)["estimated_cents"]; got != int64(0) {
		t.Fatalf("empty model list must not fall back to a flat estimate, got %v", got)
	}

	free := map[string]any{
		"ai_models": []map[string]any{{"input_tokens": int64(5000), "output_tokens": int64(100), "spend_cents": int64(0)}},
	}
	if got := spendMetrics(free)["estimated_cents"]; got != int64(0) {
		t.Fatalf("free models must stay at zero cost, got %v", got)
	}

	noModels := map[string]any{"ai_input_tokens": int64(1_000_000), "ai_output_tokens": int64(0)}
	if got := spendMetrics(noModels)["estimated_cents"]; got != int64(300) {
		t.Fatalf("flat fallback for 1M input tokens should be 300 cents, got %v", got)
	}
}

func TestWrappedStatsAggregatePerModelSpend(t *testing.T) {
	day := func(model string, in, out, cents int64) map[string]any {
		return map[string]any{
			"grand_total": map[string]any{"total_seconds": 3600.0, "ai_input_tokens": in, "ai_output_tokens": out},
			"range":       map[string]any{"date": "2026-10-01"},
			"ai_models": []map[string]any{
				{"name": model, "input_tokens": in, "output_tokens": out, "spend_cents": cents},
			},
		}
	}
	stats := wrappedStatsFromSummaries([]map[string]any{
		day("Claude Sonnet 5.5", 1000, 100, 7),
		day("Claude Sonnet 5.5", 2000, 200, 13),
		day("GPT-5.5", 500, 50, 4),
	})
	spend := spendMetrics(stats)
	if spend["estimated_cents"] != int64(24) {
		t.Fatalf("wrapped spend = %v, want 24 cents from the pricing table", spend["estimated_cents"])
	}
	if spend["input_tokens"] != int64(3500) {
		t.Fatalf("wrapped spend tokens = %v, want 3500", spend["input_tokens"])
	}
}

func TestParseOptionalIntQueryRejectsTrailingGarbage(t *testing.T) {
	if _, err := parseOptionalIntQuery("15abc"); err == nil {
		t.Fatal("expected 15abc to be rejected")
	}
	if value, err := parseOptionalIntQuery(" 20 "); err != nil || value == nil || *value != 20 {
		t.Fatalf("expected 20, got %v (%v)", value, err)
	}
}

func TestStatusCodeForInvalidInput(t *testing.T) {
	err := fmt.Errorf("decode: %w", service.ErrInvalidInput)
	if got := statusCodeForError(err); got != 400 {
		t.Fatalf("status = %d, want 400", got)
	}
	if got := statusCodeForError(errors.New("boom")); got != 500 {
		t.Fatalf("status = %d, want 500", got)
	}
}
