package apihttp_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"waka-personal/internal/config"
	"waka-personal/internal/domain"
	apihttp "waka-personal/internal/http"
	"waka-personal/internal/service"
	"waka-personal/internal/store"
)

func TestHeartbeatsBulk_PersistsAndUpsertsRecords(t *testing.T) {
	dataStore := newTestStore(t)
	app := apihttp.NewApp(&config.Config{
		CORSAllowOrigins:        []string{"*"},
		AppTimezone:             "UTC",
		KeystrokeTimeoutMinutes: 15,
	}, &apihttp.Checker{}, apihttp.Services{
		Auth:       stubAuth{},
		Heartbeats: service.NewHeartbeatService(dataStore),
		Query:      stubQuery{},
	})

	initialPayload := `[{"id":"source-hb-1","entity":"E:/repo/main.go","type":"file","time":1710000000,"project":"waka-personal","language":"go","is_write":true,"plugin":"vscode"}]`
	resp := performBulkHeartbeatRequest(t, app, "machine-a", initialPayload)
	assertSuccessfulHeartbeatResponse(t, resp)

	records := listHeartbeatRecords(t, dataStore, time.Unix(1710000000, 0).UTC())
	if len(records) != 1 {
		t.Fatalf("expected 1 heartbeat after first ingest, got %d", len(records))
	}
	if records[0].Entity != "E:/repo/main.go" {
		t.Fatalf("expected initial entity to be persisted, got %q", records[0].Entity)
	}
	if records[0].MachineName != "machine-a" {
		t.Fatalf("expected machine name to be persisted, got %q", records[0].MachineName)
	}

	updatedPayload := `[{"id":"source-hb-1","entity":"E:/repo/renamed.go","type":"file","time":1710000000,"project":"waka-personal-renamed","language":"typescript","is_write":false,"plugin":"neovim"}]`
	resp = performBulkHeartbeatRequest(t, app, "machine-b", updatedPayload)
	assertSuccessfulHeartbeatResponse(t, resp)

	records = listHeartbeatRecords(t, dataStore, time.Unix(1710000000, 0).UTC())
	if len(records) != 1 {
		t.Fatalf("expected 1 heartbeat after upsert, got %d", len(records))
	}

	record := records[0]
	if record.Entity != "E:/repo/renamed.go" {
		t.Fatalf("expected entity to be updated, got %q", record.Entity)
	}
	if record.Project != "waka-personal-renamed" {
		t.Fatalf("expected project to be updated, got %q", record.Project)
	}
	if record.Language != "typescript" {
		t.Fatalf("expected language to be updated, got %q", record.Language)
	}
	if record.MachineName != "machine-b" {
		t.Fatalf("expected machine name to be updated, got %q", record.MachineName)
	}
	if record.Plugin != "neovim" {
		t.Fatalf("expected plugin to be updated, got %q", record.Plugin)
	}
	if record.IsWrite {
		t.Fatal("expected is_write to be updated to false")
	}
}

func performBulkHeartbeatRequest(t *testing.T, app *fiber.App, machineName, body string) *http.Response {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/current/heartbeats.bulk", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Machine-Name", machineName)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test returned error: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func assertSuccessfulHeartbeatResponse(t *testing.T, resp *http.Response) {
	t.Helper()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("expected 2xx response, got %d with body %s", resp.StatusCode, string(bodyBytes))
	}
}

func listHeartbeatRecords(t *testing.T, dataStore *store.Store, at time.Time) []domain.HeartbeatRecord {
	t.Helper()

	start := time.Date(at.UTC().Year(), at.UTC().Month(), at.UTC().Day(), 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	records, err := dataStore.ListHeartbeatsByRange(ctx, start, end)
	if err != nil {
		t.Fatalf("ListHeartbeatsByRange returned error: %v", err)
	}
	return records
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()

	baseURL := lookupTestDatabaseURL(t)
	adminPool := newTestDatabaseAdminPool(t, baseURL)

	dbName := "waka_personal_test_" + strings.ReplaceAll(uuid.NewString(), "-", "_")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := adminPool.Exec(ctx, fmt.Sprintf(`CREATE DATABASE "%s"`, dbName)); err != nil {
		t.Skipf("create test database %s: %v", dbName, err)
	}

	testDatabaseURL := replaceDatabaseName(t, baseURL, dbName)
	migrationDir := resolveMigrationDir(t)
	if err := store.ApplyMigrations(ctx, testDatabaseURL, migrationDir, "goose_db_version"); err != nil {
		terminateAndDropDatabase(t, adminPool, dbName)
		t.Fatalf("apply migrations: %v", err)
	}

	pool, err := store.Connect(ctx, testDatabaseURL)
	if err != nil {
		terminateAndDropDatabase(t, adminPool, dbName)
		t.Fatalf("connect to test database: %v", err)
	}

	dataStore := store.New(pool)
	t.Cleanup(func() {
		dataStore.Close()
		terminateAndDropDatabase(t, adminPool, dbName)
		adminPool.Close()
	})

	return dataStore
}

func lookupTestDatabaseURL(t *testing.T) string {
	t.Helper()

	for _, key := range []string{"WAKA_TEST_DATABASE_URL", "DATABASE_URL"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}

	envPath := filepath.Join(resolveRepoRoot(t), ".env")
	values, err := godotenv.Read(envPath)
	if err == nil {
		if value := strings.TrimSpace(values["DATABASE_URL"]); value != "" {
			return value
		}
	}

	t.Skip("DATABASE_URL is not configured for integration tests")
	return ""
}

func newTestDatabaseAdminPool(t *testing.T, databaseURL string) *pgxpool.Pool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	adminPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Skipf("connect to postgres for integration tests: %v", err)
	}

	return adminPool
}

func terminateAndDropDatabase(t *testing.T, adminPool *pgxpool.Pool, dbName string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := adminPool.Exec(ctx, `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()`, dbName); err != nil {
		t.Fatalf("terminate connections for %s: %v", dbName, err)
	}
	if _, err := adminPool.Exec(ctx, fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName)); err != nil {
		t.Fatalf("drop test database %s: %v", dbName, err)
	}
}

func replaceDatabaseName(t *testing.T, rawURL, dbName string) string {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse database url %q: %v", rawURL, err)
	}
	parsed.Path = "/" + dbName
	return parsed.String()
}

func resolveMigrationDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(resolveRepoRoot(t), "db", "migrations")
}

func resolveRepoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve repo root: runtime caller unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
