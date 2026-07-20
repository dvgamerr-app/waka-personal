package apihttp_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"waka-personal/internal/config"
	"waka-personal/internal/domain"
	apihttp "waka-personal/internal/http"
	"waka-personal/internal/service"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type stubAuth struct {
	err error
}

func (s stubAuth) Validate(queryAPIKey, authorization string) error {
	return s.err
}

type stubHeartbeats struct{}

func (s stubHeartbeats) Ingest(ctx context.Context, body []byte, machineName string, importBatchID *string) ([]domain.HeartbeatRecord, error) {
	return []domain.HeartbeatRecord{
		{
			ID:     "hb-1",
			Entity: "/tmp/main.go",
			Type:   "file",
			Time:   time.Unix(1710000000, 0).UTC(),
		},
	}, nil
}

type stubQuery struct {
	fileExpertsPayload []map[string]any
}

type recordingQuery struct {
	stubQuery
	mu            sync.Mutex
	statsCalls    []domain.StatsQueryParams
	summaryCalls  []domain.SummaryQueryParams
	durationCalls []domain.DurationQueryParams
}

func (s stubQuery) HeartbeatsForDate(ctx context.Context, day time.Time) (records []domain.HeartbeatRecord, start, end time.Time, timezone string, err error) {
	aiInputTokens := int64(1200)
	aiOutputTokens := int64(800)
	aiPromptLength := 64
	return []domain.HeartbeatRecord{
		{
			ID:                 "hb-1",
			Entity:             "/tmp/main.go",
			Type:               "file",
			Category:           "coding",
			Time:               time.Unix(1710000000, 0).UTC(),
			AISession:          "sess-123",
			AISubscriptionPlan: "pro",
			AIInputTokens:      &aiInputTokens,
			AIOutputTokens:     &aiOutputTokens,
			AIPromptLength:     &aiPromptLength,
		},
	}, day.UTC(), day.Add(24 * time.Hour).UTC(), "UTC", nil
}

func (s stubQuery) DeleteHeartbeatsForDate(ctx context.Context, day time.Time, ids []string) (int64, error) {
	return int64(len(ids)), nil
}

func (s stubQuery) Durations(ctx context.Context, params domain.DurationQueryParams) ([]map[string]any, time.Time, time.Time, string, error) {
	return []map[string]any{
		{
			"project":  "waka-personal",
			"time":     float64(time.Unix(1710000000, 0).Unix()),
			"duration": 300.0,
		},
	}, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC), "UTC", nil
}

func (s stubQuery) Summaries(ctx context.Context, params domain.SummaryQueryParams) ([]map[string]any, error) {
	return []map[string]any{
		{
			"grand_total": map[string]any{
				"text":          "1h 02m",
				"total_seconds": 3720.0,
			},
		},
	}, nil
}

func (s stubQuery) Stats(ctx context.Context, params domain.StatsQueryParams) (map[string]any, error) {
	return map[string]any{
		"human_readable_total_including_other_language": "2h 10m",
		"total_seconds_including_other_language":        7800.0,
	}, nil
}

func (s stubQuery) StatusbarToday(ctx context.Context, now time.Time) (map[string]any, error) {
	return map[string]any{
		"grand_total": map[string]any{
			"text":          "1h 02m",
			"total_seconds": 3720.0,
		},
		"categories": []map[string]any{
			{
				"name":          "Coding",
				"text":          "1h 02m",
				"total_seconds": 3720.0,
			},
		},
		"range": map[string]any{
			"text":     "Today",
			"timezone": "Asia/Bangkok",
		},
	}, nil
}

func (s stubQuery) FileExperts(ctx context.Context, entity, project string, projectRootCount *int, now time.Time) ([]map[string]any, error) {
	return s.fileExpertsPayload, nil
}

func (q *recordingQuery) Summaries(ctx context.Context, params domain.SummaryQueryParams) ([]map[string]any, error) {
	q.mu.Lock()
	q.summaryCalls = append(q.summaryCalls, params)
	q.mu.Unlock()
	return q.stubQuery.Summaries(ctx, params)
}

func (q *recordingQuery) Stats(ctx context.Context, params domain.StatsQueryParams) (map[string]any, error) {
	q.mu.Lock()
	q.statsCalls = append(q.statsCalls, params)
	q.mu.Unlock()
	return q.stubQuery.Stats(ctx, params)
}

func (q *recordingQuery) Durations(ctx context.Context, params domain.DurationQueryParams) ([]map[string]any, time.Time, time.Time, string, error) {
	q.mu.Lock()
	q.durationCalls = append(q.durationCalls, params)
	q.mu.Unlock()
	return q.stubQuery.Durations(ctx, params)
}

func TestNewApp_RejectsUnauthorizedRequests(t *testing.T) {
	app := apihttp.NewApp(&config.Config{CORSAllowOrigins: []string{"*"}}, &apihttp.Checker{}, apihttp.Services{
		Auth:       service.NewAuthService("secret"),
		Heartbeats: stubHeartbeats{},
		Query:      stubQuery{},
	})

	req := httptest.NewRequest("GET", "/api/v1/users/current/statusbar/today?api_key=wrong", http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestNewApp_FileExpertsAcceptsDoubleEncodedJSON(t *testing.T) {
	app := apihttp.NewApp(&config.Config{CORSAllowOrigins: []string{"*"}}, &apihttp.Checker{}, apihttp.Services{
		Auth:       stubAuth{},
		Heartbeats: stubHeartbeats{},
		Query: stubQuery{
			fileExpertsPayload: []map[string]any{
				{
					"user": map[string]any{"name": "dvgamerr", "is_current_user": true},
					"total": map[string]any{
						"text":          "10m",
						"total_seconds": 600,
					},
				},
			},
		},
	})

	body := "\"{\\\"entity\\\":\\\"/tmp/main.go\\\",\\\"project\\\":\\\"waka-personal\\\"}\""
	req := httptest.NewRequest("POST", "/api/v1/users/current/file_experts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d with body %s", resp.StatusCode, string(bodyBytes))
	}
}

func TestNewApp_HeartbeatsBulkExecutesHandler(t *testing.T) {
	app := apihttp.NewApp(&config.Config{CORSAllowOrigins: []string{"*"}}, &apihttp.Checker{}, apihttp.Services{
		Auth:       stubAuth{},
		Heartbeats: stubHeartbeats{},
		Query:      stubQuery{},
	})

	req := httptest.NewRequest("POST", "/api/v1/users/current/heartbeats.bulk", strings.NewReader(`[{"entity":"/tmp/main.go","time":1710000000}]`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d with body %s", resp.StatusCode, string(bodyBytes))
	}
	for _, expected := range []string{`"accepted":1`, `"id":"hb-1"`, `"entity":"/tmp/main.go"`, `"responses":[`, `202`} {
		if !strings.Contains(string(bodyBytes), expected) {
			t.Fatalf("expected response body to contain %s, got %s", expected, string(bodyBytes))
		}
	}
}

func TestNewApp_GetHeartbeatsIncludesAITelemetry(t *testing.T) {
	app := apihttp.NewApp(&config.Config{CORSAllowOrigins: []string{"*"}}, &apihttp.Checker{}, apihttp.Services{
		Auth:       stubAuth{},
		Heartbeats: stubHeartbeats{},
		Query:      stubQuery{},
	})

	req := httptest.NewRequest("GET", "/api/v1/users/current/heartbeats?date=2024-03-09", http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", resp.StatusCode, string(bodyBytes))
	}

	body := string(bodyBytes)
	for _, expected := range []string{
		`"ai_session":"sess-123"`,
		`"ai_subscription_plan":"pro"`,
		`"ai_input_tokens":1200`,
		`"ai_output_tokens":800`,
		`"ai_prompt_length":64`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected response body to contain %s, got %s", expected, body)
		}
	}
}

func TestNewApp_StatusbarTodayShape(t *testing.T) {
	app := apihttp.NewApp(&config.Config{CORSAllowOrigins: []string{"*"}}, &apihttp.Checker{}, apihttp.Services{
		Auth:       stubAuth{},
		Heartbeats: stubHeartbeats{},
		Query:      stubQuery{},
	})

	req := httptest.NewRequest("GET", "/api/v1/users/current/statusbar/today", http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestNewApp_ServesWebsiteFromDist(t *testing.T) {
	distDir := newWebsiteDist(t, map[string]string{
		"index.html":     "<!doctype html><html><body>website</body></html>",
		"assets/app.js":  "console.log('website')",
		"favicon.svg":    "<svg></svg>",
		"_astro/app.css": "body{color:black}",
	})

	app := apihttp.NewApp(&config.Config{
		CORSAllowOrigins: []string{"*"},
		WebsiteDistDir:   distDir,
		AppAPIKey:        "runtime-key",
		AppTimezone:      "Asia/Bangkok",
	}, &apihttp.Checker{}, apihttp.Services{
		Auth:       stubAuth{},
		Heartbeats: stubHeartbeats{},
		Query:      stubQuery{},
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := apihttp.Shutdown(ctx, app); err != nil {
			t.Fatalf("shutdown app: %v", err)
		}
	})

	rootReq := httptest.NewRequest("GET", "/", http.NoBody)
	rootResp, err := app.Test(rootReq)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	t.Cleanup(func() { _ = rootResp.Body.Close() })
	if rootResp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", rootResp.StatusCode)
	}

	rootBody, err := io.ReadAll(rootResp.Body)
	if err != nil {
		t.Fatalf("read root response body: %v", err)
	}
	if !strings.Contains(string(rootBody), "website") {
		t.Fatalf("expected root response to contain website markup, got %s", string(rootBody))
	}
	for _, expected := range []string{
		"window.__WAKA_DASHBOARD_CONFIG__",
		`"timezone":"Asia/Bangkok"`,
	} {
		if !strings.Contains(string(rootBody), expected) {
			t.Fatalf("expected root response to contain %s, got %s", expected, string(rootBody))
		}
	}
	if strings.Contains(string(rootBody), "runtime-key") {
		t.Fatalf("expected root response to avoid exposing api key, got %s", string(rootBody))
	}

	assetReq := httptest.NewRequest("GET", "/assets/app.js", http.NoBody)
	assetResp, err := app.Test(assetReq)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	t.Cleanup(func() { _ = assetResp.Body.Close() })
	if assetResp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", assetResp.StatusCode)
	}

	assetBody, err := io.ReadAll(assetResp.Body)
	if err != nil {
		t.Fatalf("read asset response body: %v", err)
	}
	if !strings.Contains(string(assetBody), "console.log('website')") {
		t.Fatalf("expected asset response to contain built asset content, got %s", string(assetBody))
	}
}

func TestNewApp_FallsBackToIndexForWebsiteRoutes(t *testing.T) {
	distDir := newWebsiteDist(t, map[string]string{
		"index.html": "<!doctype html><html><body>dashboard shell</body></html>",
	})

	app := apihttp.NewApp(&config.Config{
		CORSAllowOrigins: []string{"*"},
		WebsiteDistDir:   distDir,
	}, &apihttp.Checker{}, apihttp.Services{
		Auth:       stubAuth{},
		Heartbeats: stubHeartbeats{},
		Query:      stubQuery{},
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := apihttp.Shutdown(ctx, app); err != nil {
			t.Fatalf("shutdown app: %v", err)
		}
	})

	req := httptest.NewRequest("GET", "/dashboard", http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(body), "dashboard shell") {
		t.Fatalf("expected website fallback to return index markup, got %s", string(body))
	}
}

func TestNewApp_DoesNotFallbackForAPIRoutesOrMissingAssets(t *testing.T) {
	distDir := newWebsiteDist(t, map[string]string{
		"index.html": "<!doctype html><html><body>dashboard shell</body></html>",
	})

	app := apihttp.NewApp(&config.Config{
		CORSAllowOrigins: []string{"*"},
		WebsiteDistDir:   distDir,
	}, &apihttp.Checker{}, apihttp.Services{
		Auth:       stubAuth{},
		Heartbeats: stubHeartbeats{},
		Query:      stubQuery{},
	})
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := apihttp.Shutdown(ctx, app); err != nil {
			t.Fatalf("shutdown app: %v", err)
		}
	})

	for _, target := range []string{"/api/missing", "/missing.js"} {
		req := httptest.NewRequest("GET", target, http.NoBody)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test returned error for %s: %v", target, err)
		}
		t.Cleanup(func() { _ = resp.Body.Close() })
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404 for %s, got %d", target, resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("read response body for %s: %v", target, err)
		}
		if strings.Contains(string(body), "dashboard shell") {
			t.Fatalf("expected %s to avoid website fallback, got %s", target, string(body))
		}
	}
}

func TestNewApp_SetsSecurityHeaders(t *testing.T) {
	app := apihttp.NewApp(&config.Config{CORSAllowOrigins: []string{"*"}}, &apihttp.Checker{}, apihttp.Services{
		Auth:       stubAuth{},
		Heartbeats: stubHeartbeats{},
		Query:      stubQuery{},
	})

	req := httptest.NewRequest("GET", "/healthz/live", http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}
	for header, expected := range headers {
		if got := resp.Header.Get(header); got != expected {
			t.Fatalf("expected %s to be %q, got %q", header, expected, got)
		}
	}

	csp := resp.Header.Get("Content-Security-Policy")
	for _, expected := range []string{
		"default-src 'self'",
		"object-src 'none'",
		"frame-ancestors 'none'",
		"script-src 'self' 'unsafe-inline'",
	} {
		if !strings.Contains(csp, expected) {
			t.Fatalf("expected CSP to contain %q, got %q", expected, csp)
		}
	}

	permissionsPolicy := resp.Header.Get("Permissions-Policy")
	for _, expected := range []string{"camera=()", "microphone=()", "payment=()"} {
		if !strings.Contains(permissionsPolicy, expected) {
			t.Fatalf("expected Permissions-Policy to contain %q, got %q", expected, permissionsPolicy)
		}
	}
}

func TestNewApp_LogsAPIRequestsAtDebugLevel(t *testing.T) {
	var buffer bytes.Buffer
	previousLogger := log.Logger
	previousLevel := zerolog.GlobalLevel()
	log.Logger = zerolog.New(&buffer)
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	t.Cleanup(func() {
		log.Logger = previousLogger
		zerolog.SetGlobalLevel(previousLevel)
	})

	app := apihttp.NewApp(&config.Config{CORSAllowOrigins: []string{"*"}}, &apihttp.Checker{}, apihttp.Services{
		Auth:       stubAuth{},
		Heartbeats: stubHeartbeats{},
		Query:      stubQuery{},
	})

	req := httptest.NewRequest("GET", "/api/v1/users/current/statusbar/today?api_key=secret", http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	output := buffer.String()
	for _, expected := range []string{
		"\"message\":\"api request\"",
		"\"method\":\"GET\"",
		"\"path\":\"/api/v1/users/current/statusbar/today\"",
		"\"status\":200",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected log output to contain %s, got %s", expected, output)
		}
	}
}

func TestNewApp_DashboardMapsLastMonthToCalendarStatsRange(t *testing.T) {
	query := &recordingQuery{}
	app := apihttp.NewApp(&config.Config{CORSAllowOrigins: []string{"*"}}, &apihttp.Checker{}, apihttp.Services{
		Auth:       service.NewAuthService("secret"),
		Heartbeats: stubHeartbeats{},
		Query:      query,
	})

	req := httptest.NewRequest("GET", "/api/v2/dashboard?range=Last+Month&timezone=Asia%2FBangkok", http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	now := time.Now().In(loc)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	expectedStatsRange := monthStart.AddDate(0, -1, 0).Format("2006-01")

	query.mu.Lock()
	defer query.mu.Unlock()

	if len(query.statsCalls) != 1 {
		t.Fatalf("expected 1 stats call, got %d", len(query.statsCalls))
	}
	if query.statsCalls[0].Range != expectedStatsRange {
		t.Fatalf("expected stats range %q, got %q", expectedStatsRange, query.statsCalls[0].Range)
	}
	if len(query.summaryCalls) != 1 {
		t.Fatalf("expected 1 summaries call, got %d", len(query.summaryCalls))
	}
	if query.summaryCalls[0].Range != "Last Month" {
		t.Fatalf("expected summaries range %q, got %q", "Last Month", query.summaryCalls[0].Range)
	}
}

func TestNewApp_LiveDashboardBypassesAuth(t *testing.T) {
	app := apihttp.NewApp(&config.Config{CORSAllowOrigins: []string{"*"}}, &apihttp.Checker{}, apihttp.Services{
		Auth:       service.NewAuthService("secret"),
		Heartbeats: stubHeartbeats{},
		Query:      stubQuery{},
	})

	req := httptest.NewRequest("GET", "/api/v2/live?timezone=Asia%2FBangkok", http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", resp.StatusCode, string(bodyBytes))
	}

	body := string(bodyBytes)
	for _, expected := range []string{
		`"status":"synchronized"`,
		`"today":`,
		`"project_durations":`,
		`"language_durations":`,
		`"cached_at":`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected response body to contain %s, got %s", expected, body)
		}
	}
}

func TestNewApp_DashboardSecondaryPagesBypassAuth(t *testing.T) {
	app := apihttp.NewApp(&config.Config{CORSAllowOrigins: []string{"*"}}, &apihttp.Checker{}, apihttp.Services{
		Auth:       service.NewAuthService("secret"),
		Heartbeats: stubHeartbeats{},
		Query:      stubQuery{},
	})

	for _, path := range []string{
		"/api/v2/insights?timezone=Asia%2FBangkok",
		"/api/v2/wrapped?year=2026&timezone=Asia%2FBangkok",
	} {
		req := httptest.NewRequest("GET", path, http.NoBody)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("app.Test returned error for %s: %v", path, err)
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("read response body for %s: %v", path, err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d with body %s", path, resp.StatusCode, string(bodyBytes))
		}

		body := string(bodyBytes)
		for _, expected := range []string{`"summaries":`, `"timezone":"Asia/Bangkok"`} {
			if !strings.Contains(body, expected) {
				t.Fatalf("expected response body for %s to contain %s, got %s", path, expected, body)
			}
		}
	}
}

func TestNewApp_ProjectDetailFiltersSummariesAndBypassesAuth(t *testing.T) {
	query := &recordingQuery{}
	app := apihttp.NewApp(&config.Config{CORSAllowOrigins: []string{"*"}}, &apihttp.Checker{}, apihttp.Services{
		Auth:       service.NewAuthService("secret"),
		Heartbeats: stubHeartbeats{},
		Query:      query,
	})

	req := httptest.NewRequest("GET", "/api/v2/project?project=waka-personal&range=Last+30+Days&timezone=Asia%2FBangkok", http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d with body %s", resp.StatusCode, string(bodyBytes))
	}

	query.mu.Lock()
	defer query.mu.Unlock()
	if len(query.summaryCalls) != 1 {
		t.Fatalf("expected 1 summaries call, got %d", len(query.summaryCalls))
	}
	call := query.summaryCalls[0]
	if call.Project != "waka-personal" {
		t.Fatalf("expected project %q, got %q", "waka-personal", call.Project)
	}
	if call.Range != "Last 30 Days" {
		t.Fatalf("expected range %q, got %q", "Last 30 Days", call.Range)
	}
	if call.Timezone != "Asia/Bangkok" {
		t.Fatalf("expected timezone %q, got %q", "Asia/Bangkok", call.Timezone)
	}
}

func TestNewApp_WrappedUsesSingleSummaryScan(t *testing.T) {
	query := &recordingQuery{}
	app := apihttp.NewApp(&config.Config{CORSAllowOrigins: []string{"*"}}, &apihttp.Checker{}, apihttp.Services{
		Auth:       service.NewAuthService("secret"),
		Heartbeats: stubHeartbeats{},
		Query:      query,
	})

	req := httptest.NewRequest("GET", "/api/v2/wrapped?year=2026&timezone=Asia%2FBangkok", http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d with body %s", resp.StatusCode, string(bodyBytes))
	}

	if len(query.summaryCalls) != 1 {
		t.Fatalf("expected 1 summaries call, got %d", len(query.summaryCalls))
	}
	if query.summaryCalls[0].Range != "2026" {
		t.Fatalf("expected summaries range %q, got %q", "2026", query.summaryCalls[0].Range)
	}
	if len(query.statsCalls) != 0 {
		t.Fatalf("expected wrapped endpoint to avoid stats scan, got %d calls", len(query.statsCalls))
	}
}

func TestNewApp_DashboardUsesSelectedSingleDayForDurations(t *testing.T) {
	query := &recordingQuery{}
	app := apihttp.NewApp(&config.Config{CORSAllowOrigins: []string{"*"}}, &apihttp.Checker{}, apihttp.Services{
		Auth:       service.NewAuthService("secret"),
		Heartbeats: stubHeartbeats{},
		Query:      query,
	})

	req := httptest.NewRequest("GET", "/api/v2/dashboard?start=2026-06-15&end=2026-06-15&timezone=Asia%2FBangkok", http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d with body %s", resp.StatusCode, string(bodyBytes))
	}

	query.mu.Lock()
	defer query.mu.Unlock()
	if len(query.durationCalls) != 3 {
		t.Fatalf("expected 3 durations calls, got %d", len(query.durationCalls))
	}
	wantSliceBy := map[string]bool{"project": true, "language": true, "editor": true}
	for _, call := range query.durationCalls {
		if call.Date != "2026-06-15" {
			t.Fatalf("expected duration date %q, got %q", "2026-06-15", call.Date)
		}
		if !wantSliceBy[call.SliceBy] {
			t.Fatalf("unexpected duration slice_by %q", call.SliceBy)
		}
		delete(wantSliceBy, call.SliceBy)
	}
	if len(wantSliceBy) != 0 {
		t.Fatalf("missing duration slice_by calls: %v", wantSliceBy)
	}
}

func TestNewApp_DashboardMapsLastYearToCalendarStatsRange(t *testing.T) {
	query := &recordingQuery{}
	app := apihttp.NewApp(&config.Config{CORSAllowOrigins: []string{"*"}}, &apihttp.Checker{}, apihttp.Services{
		Auth:       service.NewAuthService("secret"),
		Heartbeats: stubHeartbeats{},
		Query:      query,
	})

	req := httptest.NewRequest("GET", "/api/v2/dashboard?range=Last+Year&timezone=Asia%2FBangkok", http.NoBody)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	loc, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	expectedStatsRange := fmt.Sprintf("%04d", time.Now().In(loc).Year()-1)

	query.mu.Lock()
	defer query.mu.Unlock()

	if len(query.statsCalls) != 1 {
		t.Fatalf("expected 1 stats call, got %d", len(query.statsCalls))
	}
	if query.statsCalls[0].Range != expectedStatsRange {
		t.Fatalf("expected stats range %q, got %q", expectedStatsRange, query.statsCalls[0].Range)
	}
	if len(query.summaryCalls) != 1 {
		t.Fatalf("expected 1 summaries call, got %d", len(query.summaryCalls))
	}
	if query.summaryCalls[0].Range != "Last Year" {
		t.Fatalf("expected summaries range %q, got %q", "Last Year", query.summaryCalls[0].Range)
	}
}

func newWebsiteDist(t *testing.T, files map[string]string) string {
	t.Helper()

	distDir, err := os.MkdirTemp("", "waka-personal-http-*")
	if err != nil {
		t.Fatalf("create website dist temp dir: %v", err)
	}
	t.Cleanup(func() {
		var removeErr error
		for range 10 {
			removeErr = os.RemoveAll(distDir)
			if removeErr == nil || os.IsNotExist(removeErr) {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Logf("cleanup website dist temp dir %s: %v", distDir, removeErr)
	})

	for relativePath, contents := range files {
		fullPath := filepath.Join(distDir, relativePath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("create website dist dir for %s: %v", relativePath, err)
		}
		if err := os.WriteFile(fullPath, []byte(contents), 0o644); err != nil {
			t.Fatalf("write website dist file %s: %v", relativePath, err)
		}
	}

	return distDir
}
