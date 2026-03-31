package http

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"waka-personal/internal/config"
	"waka-personal/internal/domain"
	"waka-personal/internal/service"
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

func (s stubQuery) HeartbeatsForDate(ctx context.Context, day time.Time) ([]domain.HeartbeatRecord, time.Time, time.Time, string, error) {
	return nil, day.UTC(), day.Add(24 * time.Hour).UTC(), "UTC", nil
}

func (s stubQuery) DeleteHeartbeatsForDate(ctx context.Context, day time.Time, ids []string) (int64, error) {
	return int64(len(ids)), nil
}

func (s stubQuery) StatusbarToday(ctx context.Context, now time.Time) (domain.StatusbarTodayData, error) {
	var data domain.StatusbarTodayData
	data.GrandTotal.Text = "1 hr 2 mins"
	data.GrandTotal.TotalSeconds = 3720
	data.Categories = []domain.StatusbarCategory{{Name: "Coding", Text: "1 hr 2 mins", TotalSeconds: 3720}}
	data.HasTeamFeatures = true
	data.Timezone = "Asia/Bangkok"
	return data, nil
}

func (s stubQuery) FileExperts(ctx context.Context, entity, project string, projectRootCount *int, now time.Time) ([]map[string]any, error) {
	return s.fileExpertsPayload, nil
}

func TestNewApp_RejectsUnauthorizedRequests(t *testing.T) {
	app := NewApp(&config.Config{CORSAllowOrigins: []string{"*"}}, &Checker{}, Services{
		Auth:       service.NewAuthService("secret"),
		Heartbeats: stubHeartbeats{},
		Query:      stubQuery{},
	})

	req := httptest.NewRequest("GET", "/api/v1/users/current/statusbar/today?api_key=wrong", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestNewApp_FileExpertsAcceptsDoubleEncodedJSON(t *testing.T) {
	app := NewApp(&config.Config{CORSAllowOrigins: []string{"*"}}, &Checker{}, Services{
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
	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d with body %s", resp.StatusCode, string(bodyBytes))
	}
}

func TestNewApp_StatusbarTodayShape(t *testing.T) {
	app := NewApp(&config.Config{CORSAllowOrigins: []string{"*"}}, &Checker{}, Services{
		Auth:       stubAuth{},
		Heartbeats: stubHeartbeats{},
		Query:      stubQuery{},
	})

	req := httptest.NewRequest("GET", "/api/v1/users/current/statusbar/today", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
