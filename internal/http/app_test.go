package apihttp_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
	mu           sync.Mutex
	statsCalls   []domain.StatsQueryParams
	summaryCalls []domain.SummaryQueryParams
}

func (s stubQuery) HeartbeatsForDate(ctx context.Context, day time.Time) (records []domain.HeartbeatRecord, start, end time.Time, timezone string, err error) {
	return nil, day.UTC(), day.Add(24 * time.Hour).UTC(), "UTC", nil
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
				"text":          "1 hr 2 mins",
				"total_seconds": 3720.0,
			},
		},
	}, nil
}

func (s stubQuery) Stats(ctx context.Context, params domain.StatsQueryParams) (map[string]any, error) {
	return map[string]any{
		"human_readable_total_including_other_language": "2 hrs 10 mins",
		"total_seconds_including_other_language":        7800.0,
	}, nil
}

func (s stubQuery) StatusbarToday(ctx context.Context, now time.Time) (map[string]any, error) {
	return map[string]any{
		"grand_total": map[string]any{
			"text":          "1 hr 2 mins",
			"total_seconds": 3720.0,
		},
		"categories": []map[string]any{
			{
				"name":          "Coding",
				"text":          "1 hr 2 mins",
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
						"text":          "10 mins",
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
	for _, expected := range []string{`"accepted":1`, `"id":"hb-1"`, `"entity":"/tmp/main.go"`} {
		if !strings.Contains(string(bodyBytes), expected) {
			t.Fatalf("expected response body to contain %s, got %s", expected, string(bodyBytes))
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
		"\"path\":\"/api/v1/users/current/statusbar/today?api_key=secret\"",
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
		Auth:       stubAuth{},
		Heartbeats: stubHeartbeats{},
		Query:      query,
	})

	req := httptest.NewRequest("GET", "/api/v1/users/current/dashboard?range=Last+Month&timezone=Asia%2FBangkok", http.NoBody)
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

func TestNewApp_DashboardMapsLastYearToCalendarStatsRange(t *testing.T) {
	query := &recordingQuery{}
	app := apihttp.NewApp(&config.Config{CORSAllowOrigins: []string{"*"}}, &apihttp.Checker{}, apihttp.Services{
		Auth:       stubAuth{},
		Heartbeats: stubHeartbeats{},
		Query:      query,
	})

	req := httptest.NewRequest("GET", "/api/v1/users/current/dashboard?range=Last+Year&timezone=Asia%2FBangkok", http.NoBody)
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
