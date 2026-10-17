package apihttp_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"waka-personal/internal/config"
	"waka-personal/internal/domain"
	apihttp "waka-personal/internal/http"
	"waka-personal/internal/service"
)

type failingHeartbeats struct{ err error }

func (f failingHeartbeats) Ingest(context.Context, []byte, string, *string) ([]domain.HeartbeatRecord, error) {
	return nil, f.err
}

type deleteRecordingQuery struct {
	stubQuery
	calls int
}

func (q *deleteRecordingQuery) DeleteHeartbeatsForDate(ctx context.Context, day time.Time, timezone string, ids []string) (int64, error) {
	q.calls++
	return q.stubQuery.DeleteHeartbeatsForDate(ctx, day, timezone, ids)
}

type testApp interface {
	Test(req *http.Request, msTimeout ...int) (*http.Response, error)
}

func doRequest(t *testing.T, app testApp, req *http.Request) (status int, body string, headers http.Header) {
	t.Helper()
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, string(raw), resp.Header
}

func TestIngestInvalidInputIsBadRequest(t *testing.T) {
	app := apihttp.NewApp(&config.Config{}, &apihttp.Checker{}, apihttp.Services{
		Auth:       stubAuth{},
		Heartbeats: failingHeartbeats{err: fmt.Errorf("%w: heartbeat entity is required", service.ErrInvalidInput)},
		Query:      stubQuery{},
	})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/users/current/heartbeats.bulk", strings.NewReader(`[{"time":1}]`))
	status, body, _ := doRequest(t, app, req)
	if status != http.StatusBadRequest || !strings.Contains(body, "heartbeat entity is required") {
		t.Fatalf("status=%d body=%s, want 400 with the validation message", status, body)
	}
}

func TestInternalErrorsDoNotLeakDetails(t *testing.T) {
	app := apihttp.NewApp(&config.Config{}, &apihttp.Checker{}, apihttp.Services{
		Auth:       stubAuth{},
		Heartbeats: failingHeartbeats{err: fmt.Errorf("upsert heartbeat: password=hunter2 rejected")},
		Query:      stubQuery{},
	})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/users/current/heartbeats", strings.NewReader(`{"entity":"x","time":1}`))
	status, body, _ := doRequest(t, app, req)
	if status != http.StatusInternalServerError {
		t.Fatalf("status=%d, want 500", status)
	}
	if strings.Contains(body, "hunter2") || !strings.Contains(body, "internal server error") {
		t.Fatalf("500 body leaked internals: %s", body)
	}
}

func TestDeleteHeartbeatsRequiresIDsOrExplicitAll(t *testing.T) {
	query := &deleteRecordingQuery{}
	app := apihttp.NewApp(&config.Config{}, &apihttp.Checker{}, apihttp.Services{
		Auth:       stubAuth{},
		Heartbeats: stubHeartbeats{},
		Query:      query,
	})

	for _, body := range []string{`{"date":"2026-10-01"}`, `{"date":"2026-10-01","ids":[]}`} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/users/current/heartbeats.bulk", strings.NewReader(body))
		if status, resp, _ := doRequest(t, app, req); status != http.StatusBadRequest {
			t.Fatalf("body %s: status=%d resp=%s, want 400", body, status, resp)
		}
	}
	if query.calls != 0 {
		t.Fatalf("delete must not run without ids or all, ran %d times", query.calls)
	}

	for _, body := range []string{`{"date":"2026-10-01","ids":["a"]}`, `{"date":"2026-10-01","all":true}`} {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/api/v1/users/current/heartbeats.bulk", strings.NewReader(body))
		if status, resp, _ := doRequest(t, app, req); status != http.StatusOK {
			t.Fatalf("body %s: status=%d resp=%s, want 200", body, status, resp)
		}
	}
	if query.calls != 2 {
		t.Fatalf("expected 2 delete calls, got %d", query.calls)
	}
}

func TestCORSIsOptIn(t *testing.T) {
	closed := apihttp.NewApp(&config.Config{}, &apihttp.Checker{}, apihttp.Services{Auth: stubAuth{}, Heartbeats: stubHeartbeats{}, Query: stubQuery{}})
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v2/dashboard?range=Today", http.NoBody)
	req.Header.Set("Origin", "https://evil.example")
	if _, _, headers := doRequest(t, closed, req); headers.Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("no CORS headers expected by default, got %q", headers.Get("Access-Control-Allow-Origin"))
	}

	open := apihttp.NewApp(&config.Config{CORSAllowOrigins: []string{"https://app.example"}}, &apihttp.Checker{}, apihttp.Services{Auth: stubAuth{}, Heartbeats: stubHeartbeats{}, Query: stubQuery{}})
	allowed := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v2/dashboard?range=Today", http.NoBody)
	allowed.Header.Set("Origin", "https://app.example")
	if _, _, headers := doRequest(t, open, allowed); headers.Get("Access-Control-Allow-Origin") != "https://app.example" {
		t.Fatalf("configured origin should be allowed, got %q", headers.Get("Access-Control-Allow-Origin"))
	}
	denied := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v2/dashboard?range=Today", http.NoBody)
	denied.Header.Set("Origin", "https://evil.example")
	if _, _, headers := doRequest(t, open, denied); headers.Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("unconfigured origin must be rejected, got %q", headers.Get("Access-Control-Allow-Origin"))
	}
}

func TestDashboardPresetStatsMatchSummaries(t *testing.T) {
	for _, preset := range []string{"Last 14 Days", "This Week", "Last 7 Days from Yesterday"} {
		query := &recordingQuery{}
		app := apihttp.NewApp(&config.Config{}, &apihttp.Checker{}, apihttp.Services{Auth: stubAuth{}, Heartbeats: stubHeartbeats{}, Query: query})
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v2/dashboard?range="+strings.ReplaceAll(preset, " ", "+")+"&timezone=UTC", http.NoBody)
		if status, body, _ := doRequest(t, app, req); status != http.StatusOK {
			t.Fatalf("%s: status=%d body=%s", preset, status, body)
		}
		query.mu.Lock()
		got := append([]domain.StatsQueryParams(nil), query.statsCalls...)
		query.mu.Unlock()
		if len(got) != 1 || got[0].Range != preset {
			t.Fatalf("%s: stats calls = %#v, want the preset itself", preset, got)
		}
	}
}
