package apihttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"waka-personal/internal/config"
	apihttp "waka-personal/internal/http"
	"waka-personal/internal/service"
)

func newE2EApp(t *testing.T) *testAppHandle {
	t.Helper()
	dataStore := newTestStore(t)
	cfg := &config.Config{AppTimezone: "UTC", KeystrokeTimeoutMinutes: 15, WritesOnly: true}
	app := apihttp.NewApp(cfg, &apihttp.Checker{}, apihttp.Services{
		Auth:       stubAuth{},
		Heartbeats: service.NewHeartbeatService(dataStore),
		Query:      service.NewQueryService(dataStore, service.NewProfileService(dataStore, cfg)),
	})
	return &testAppHandle{app: app}
}

type testAppHandle struct{ app testApp }

func (h *testAppHandle) json(t *testing.T, method, path, body string) (status int, decoded map[string]any) {
	t.Helper()
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequestWithContext(context.Background(), method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	status, raw, _ := doRequest(t, h.app, req)
	decoded = map[string]any{}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
			t.Fatalf("%s %s returned non-JSON %q: %v", method, path, raw, err)
		}
	}
	return status, decoded
}

func num(t *testing.T, values map[string]any, path ...string) float64 {
	t.Helper()
	var current any = values
	for _, key := range path {
		next, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("path %v: %v is not an object", path, current)
		}
		current = next[key]
	}
	value, ok := current.(float64)
	if !ok {
		t.Fatalf("path %v is %v, not a number", path, current)
	}
	return value
}

func sumItems(t *testing.T, summaries any, bucket, field string) float64 {
	t.Helper()
	var total float64
	days, _ := summaries.([]any)
	for _, day := range days {
		items, _ := day.(map[string]any)[bucket].([]any)
		for _, item := range items {
			value, _ := item.(map[string]any)[field].(float64)
			total += value
		}
	}
	return total
}

// TestReadPathUsesOneTokenBasis drives the real services against PostgreSQL:
// a duplicate-timestamp heartbeat, a non-write heartbeat and an unattributed
// heartbeat must all be counted, and every page must agree on tokens and spend.
func TestReadPathUsesOneTokenBasis(t *testing.T) {
	h := newE2EApp(t)

	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	base := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 10, 0, 0, 0, time.UTC)
	day := base.Format("2006-01-02")
	agent := "wakatime/2.0 (Windows-amd64) go1.26 claude-code/1.0 sonnet/5.5"
	heartbeat := func(entity string, at time.Time, write bool, userAgent string, inputTokens int) string {
		ua := ""
		if userAgent != "" {
			ua = fmt.Sprintf(`,"user_agent":%q`, userAgent)
		}
		return fmt.Sprintf(`{"entity":%q,"time":%d,"project":"p","branch":"main","language":"Go","is_write":%t,"ai_session":"s1","ai_input_tokens":%d,"ai_output_tokens":0,"ai_line_changes":%d%s}`,
			entity, at.Unix(), write, inputTokens, 10, ua)
	}
	body := "[" + strings.Join([]string{
		heartbeat("/a.go", base, true, agent, 1_000_000),
		heartbeat("/b.go", base, true, agent, 2_000_000),                   // same timestamp: zero-length interval
		heartbeat("/c.go", base.Add(time.Minute), false, agent, 4_000_000), // not a write
		heartbeat("/d.go", base.Add(2*time.Minute), true, "", 1_000_000),   // no model attribution
	}, ",") + "]"
	if status, resp := h.json(t, http.MethodPost, "/api/v1/users/current/heartbeats.bulk", body); status != http.StatusAccepted {
		t.Fatalf("seed ingest status=%d resp=%v", status, resp)
	}

	const wantInput = 8_000_000.0
	// 7M tokens on sonnet 5.5 at $2/M plus 1M unattributed tokens at the $3/M default.
	const wantCents = 1700.0

	_, dash := h.json(t, http.MethodGet, "/api/v2/dashboard?range=Yesterday&timezone=UTC", "")
	if errs, _ := dash["errors"].([]any); len(errs) != 0 {
		t.Fatalf("dashboard reported errors: %v", errs)
	}
	if got := num(t, dash, "token_metrics", "input_tokens"); got != wantInput {
		t.Fatalf("dashboard token KPI = %v, want %v even with writes_only", got, wantInput)
	}
	if got := num(t, dash, "spend_metrics", "input_tokens"); got != wantInput {
		t.Fatalf("spend basis = %v tokens, want %v", got, wantInput)
	}
	if got := num(t, dash, "spend_metrics", "estimated_cents"); got != wantCents {
		t.Fatalf("spend = %v cents, want %v", got, wantCents)
	}
	if got := sumItems(t, dash["summaries"], "projects", "spend_cents"); got != wantCents {
		t.Fatalf("project spend = %v, want %v", got, wantCents)
	}
	if got := sumItems(t, dash["summaries"], "ai_models", "spend_cents"); got != wantCents {
		t.Fatalf("model spend = %v, want %v", got, wantCents)
	}
	if got := num(t, dash, "stats", "days_including_holidays"); got != 1 {
		t.Fatalf("stats window = %v days, want 1", got)
	}

	for _, preset := range []string{"Last 14 Days", "This Week"} {
		_, resp := h.json(t, http.MethodGet, "/api/v2/dashboard?range="+strings.ReplaceAll(preset, " ", "+")+"&timezone=UTC", "")
		summaries, _ := resp["summaries"].([]any)
		if got := num(t, resp, "stats", "days_including_holidays"); got != float64(len(summaries)) {
			t.Fatalf("%s: stats cover %v days but summaries cover %d", preset, got, len(summaries))
		}
	}

	_, wrapped := h.json(t, http.MethodGet, fmt.Sprintf("/api/v2/wrapped?year=%d&timezone=UTC", base.Year()), "")
	if got := num(t, wrapped, "spend_metrics", "estimated_cents"); got != wantCents {
		t.Fatalf("wrapped spend = %v, want the dashboard value %v", got, wantCents)
	}
	if got := num(t, wrapped, "activity", "token_usage", "input_tokens"); got != wantInput {
		t.Fatalf("wrapped activity tokens = %v, want %v", got, wantInput)
	}

	_, project := h.json(t, http.MethodGet, "/api/v2/project?project=p&range=Yesterday&timezone=UTC", "")
	if got := sumItems(t, project["summaries"], "ai_models", "spend_cents"); got != wantCents {
		t.Fatalf("project page spend = %v, want %v", got, wantCents)
	}

	_, insights := h.json(t, http.MethodGet, "/api/v2/insights?range=Last+Week&timezone=UTC", "")
	if _, ok := insights["spend_metrics"]; !ok {
		t.Fatalf("insights response is missing spend_metrics: %v", insights)
	}

	_, resume := h.json(t, http.MethodGet, "/api/v2/resume?months=1&timezone=UTC", "")
	if got := num(t, resume, "aiAssisted", "tokens", "inputTokens"); got != wantInput {
		t.Fatalf("resume tokens = %v, want %v", got, wantInput)
	}

	// WakaTime-compatible read endpoints.
	for _, path := range []string{
		"/api/v1/users/current/durations?date=" + day + "&timezone=UTC",
		"/api/v1/users/current/summaries?start=" + day + "&end=" + day + "&timezone=UTC",
		"/api/v1/users/current/stats/yesterday?timezone=UTC",
		"/api/v1/users/current/statusbar/today?timezone=UTC",
		"/api/v1/users/current/heartbeats?date=" + day + "&timezone=UTC",
	} {
		if status, resp := h.json(t, http.MethodGet, path, ""); status != http.StatusOK {
			t.Fatalf("GET %s status=%d resp=%v", path, status, resp)
		}
	}
	if status, resp := h.json(t, http.MethodPost, "/api/v1/users/current/file_experts", `{"entity":"/a.go","project":"p"}`); status != http.StatusOK {
		t.Fatalf("file_experts status=%d resp=%v", status, resp)
	}

	// Deleting a day needs an explicit ids list or "all".
	deletePath := "/api/v1/users/current/heartbeats.bulk"
	if status, _ := h.json(t, http.MethodDelete, deletePath, fmt.Sprintf(`{"date":%q,"timezone":"UTC"}`, day)); status != http.StatusBadRequest {
		t.Fatalf("delete without ids/all status=%d, want 400", status)
	}
	status, resp := h.json(t, http.MethodDelete, deletePath, fmt.Sprintf(`{"date":%q,"timezone":"UTC","all":true}`, day))
	if status != http.StatusOK || num(t, resp, "data", "deleted") != 4 {
		t.Fatalf("delete all status=%d resp=%v, want 4 deleted", status, resp)
	}
}
