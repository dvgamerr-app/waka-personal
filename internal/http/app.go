package apihttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"

	"waka-personal/internal/config"
	"waka-personal/internal/domain"
)

type Authenticator interface {
	Validate(queryAPIKey, authorization string) error
}

type HeartbeatIngester interface {
	Ingest(ctx context.Context, body []byte, machineName string, importBatchID *string) ([]domain.HeartbeatRecord, error)
}

type QueryReader interface {
	HeartbeatsForDate(ctx context.Context, day time.Time) ([]domain.HeartbeatRecord, time.Time, time.Time, string, error)
	DeleteHeartbeatsForDate(ctx context.Context, day time.Time, ids []string) (int64, error)
	Durations(ctx context.Context, params domain.DurationQueryParams) ([]map[string]any, time.Time, time.Time, string, error)
	Summaries(ctx context.Context, params domain.SummaryQueryParams) ([]map[string]any, error)
	Stats(ctx context.Context, params domain.StatsQueryParams) (map[string]any, error)
	StatusbarToday(ctx context.Context, now time.Time) (map[string]any, error)
	FileExperts(ctx context.Context, entity, project string, projectRootCount *int, now time.Time) ([]map[string]any, error)
}

type Services struct {
	Auth       Authenticator
	Heartbeats HeartbeatIngester
	Query      QueryReader
}

func NewApp(cfg *config.Config, checker *Checker, services Services) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:               "waka-personal",
		DisableStartupMessage: true,
		ErrorHandler:          newAppErrorHandler(),
	})

	configureAppMiddleware(app, cfg)
	registerHealthRoutes(app, checker)
	registerUserRoutes(app, services)
	registerWebsiteRoutes(app, cfg)

	return app
}

func newAppErrorHandler() fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		if err == nil {
			err = errors.New("unknown error")
		}
		return c.Status(statusCodeForError(err)).JSON(fiber.Map{
			"error": err.Error(),
		})
	}
}

func configureAppMiddleware(app *fiber.App, cfg *config.Config) {
	app.Use(requestid.New())
	app.Use(securityHeadersMiddleware())
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Join(cfg.CORSAllowOrigins, ","),
		AllowHeaders:     "Authorization, Content-Type, X-Machine-Name",
		AllowMethods:     "GET,POST,DELETE,OPTIONS",
		AllowCredentials: false,
	}))
	app.Use("/api", limiter.New(limiter.Config{
		Max:        60,
		Expiration: time.Minute,
		KeyGenerator: func(_ *fiber.Ctx) string {
			return "global"
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "rate limit exceeded",
			})
		},
	}))
	app.Use("/api", apiDebugLogger())
}

func securityHeadersMiddleware() fiber.Handler {
	csp := strings.Join([]string{
		"default-src 'self'",
		"base-uri 'self'",
		"object-src 'none'",
		"frame-ancestors 'none'",
		"form-action 'self'",
		"img-src 'self' data: https:",
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
		"font-src 'self' data: https://fonts.gstatic.com",
		"script-src 'self' 'unsafe-inline'",
		"connect-src 'self' http: https:",
	}, "; ")

	permissionsPolicy := strings.Join([]string{
		"accelerometer=()",
		"camera=()",
		"geolocation=()",
		"gyroscope=()",
		"magnetometer=()",
		"microphone=()",
		"payment=()",
		"usb=()",
	}, ", ")

	return func(c *fiber.Ctx) error {
		c.Set("Content-Security-Policy", csp)
		c.Set("Permissions-Policy", permissionsPolicy)
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")

		if requestIsHTTPS(c) {
			c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		return c.Next()
	}
}

func registerHealthRoutes(app *fiber.App, checker *Checker) {
	app.Get("/healthz/live", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	app.Get("/healthz/ready", func(c *fiber.Ctx) error {
		if !checker.IsReady() {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "not ready"})
		}
		return c.JSON(fiber.Map{"status": "ready"})
	})
}

func registerUserRoutes(app *fiber.App, services Services) {
	api := app.Group("/api/v1/users/current", authenticateRequest(services.Auth))
	api.Use(cacheControlMiddleware())
	api.Post("/heartbeats", postHeartbeatHandler(services.Heartbeats))
	api.Post("/heartbeats.bulk", postBulkHeartbeatsHandler(services.Heartbeats))
	api.Get("/heartbeats", getHeartbeatsHandler(services.Query))
	api.Delete("/heartbeats.bulk", deleteHeartbeatsHandler(services.Query))
	api.Get("/durations", durationsHandler(services.Query))
	api.Get("/summaries", summariesHandler(services.Query))
	api.Get("/stats", statsHandler(services.Query))
	api.Get("/stats/:range", statsHandler(services.Query))
	api.Get("/statusbar/today", statusbarTodayHandler(services.Query))
	api.Get("/status_bar/today", statusbarTodayHandler(services.Query))
	api.Post("/file_experts", fileExpertsHandler(services.Query))
	api.Get("/dashboard", dashboardHandler(services.Query))
}

func registerWebsiteRoutes(app *fiber.App, cfg *config.Config) {
	distDir := filepath.Clean(strings.TrimSpace(cfg.WebsiteDistDir))
	if distDir == "" || distDir == "." {
		return
	}

	indexPath := filepath.Join(distDir, "index.html")
	info, err := os.Stat(indexPath)
	if err != nil || info.IsDir() {
		return
	}

	app.Static("/", distDir, fiber.Static{
		Browse:   false,
		Compress: true,
		Index:    "index.html",
	})

	indexHandler := websiteIndexHandler(indexPath)
	app.Get("/*", indexHandler)
	app.Head("/*", indexHandler)
}

func websiteIndexHandler(indexPath string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !shouldServeWebsiteIndex(c.Path()) {
			return fiber.ErrNotFound
		}
		return c.SendFile(indexPath)
	}
}

func shouldServeWebsiteIndex(path string) bool {
	normalizedPath := strings.TrimSpace(strings.ToLower(path))
	if normalizedPath == "" {
		return true
	}

	if normalizedPath == "/api" || strings.HasPrefix(normalizedPath, "/api/") {
		return false
	}
	if normalizedPath == "/healthz" || strings.HasPrefix(normalizedPath, "/healthz/") {
		return false
	}
	if normalizedPath == "/_astro" || strings.HasPrefix(normalizedPath, "/_astro/") {
		return false
	}

	return filepath.Ext(strings.TrimSuffix(normalizedPath, "/")) == ""
}

func requestIsHTTPS(c *fiber.Ctx) bool {
	if strings.EqualFold(c.Protocol(), "https") {
		return true
	}

	forwardedProto := strings.TrimSpace(strings.ToLower(c.Get("X-Forwarded-Proto")))
	return strings.HasPrefix(forwardedProto, "https")
}

func authenticateRequest(auth Authenticator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if err := auth.Validate(c.Query("api_key"), c.Get("Authorization")); err != nil {
			return err
		}
		return c.Next()
	}
}

func postHeartbeatHandler(ingester HeartbeatIngester) fiber.Handler {
	return func(c *fiber.Ctx) error {
		records, err := ingester.Ingest(c.Context(), c.Body(), c.Get("X-Machine-Name"), nil)
		if err != nil {
			return err
		}
		if len(records) == 0 {
			return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"data": fiber.Map{}})
		}

		record := records[0]
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
			"data": fiber.Map{
				"id":     record.ID,
				"entity": record.Entity,
				"type":   record.Type,
				"time":   float64(record.Time.UnixNano()) / float64(time.Second),
			},
		})
	}
}

func postBulkHeartbeatsHandler(ingester HeartbeatIngester) fiber.Handler {
	return func(c *fiber.Ctx) error {
		records, err := ingester.Ingest(c.Context(), c.Body(), c.Get("X-Machine-Name"), nil)
		if err != nil {
			return err
		}

		items := make([]fiber.Map, 0, len(records))
		for i := range records {
			record := &records[i]
			items = append(items, fiber.Map{
				"id":     record.ID,
				"entity": record.Entity,
				"type":   record.Type,
				"time":   float64(record.Time.UnixNano()) / float64(time.Second),
			})
		}
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
			"data": fiber.Map{
				"accepted":   len(records),
				"heartbeats": items,
			},
		})
	}
}

func getHeartbeatsHandler(query QueryReader) fiber.Handler {
	return func(c *fiber.Ctx) error {
		day, err := parseDayQuery(c.Query("date"))
		if err != nil {
			return err
		}

		records, start, end, timezone, err := query.HeartbeatsForDate(c.Context(), day)
		if err != nil {
			return err
		}

		items := make([]fiber.Map, 0, len(records))
		for i := range records {
			record := &records[i]
			items = append(items, fiber.Map{
				"id":                 record.ID,
				"entity":             record.Entity,
				"type":               record.Type,
				"category":           record.Category,
				"time":               float64(record.Time.UnixNano()) / float64(time.Second),
				"project":            stringOrNil(record.Project),
				"project_root_count": record.ProjectRootCount,
				"branch":             stringOrNil(record.Branch),
				"language":           stringOrNil(record.Language),
				"dependencies":       record.Dependencies,
				"machine_name_id":    stringOrNil(record.SourceMachineNameID),
				"ai_line_changes":    record.AILineChanges,
				"human_line_changes": record.HumanLineChanges,
				"lines":              record.Lines,
				"lineno":             record.Lineno,
				"cursorpos":          record.Cursorpos,
				"is_write":           record.IsWrite,
			})
		}

		return c.JSON(fiber.Map{
			"data":     items,
			"start":    start.Format(time.RFC3339),
			"end":      end.Format(time.RFC3339),
			"timezone": timezone,
		})
	}
}

func deleteHeartbeatsHandler(query QueryReader) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var payload struct {
			Date string   `json:"date"`
			IDs  []string `json:"ids"`
		}
		if err := decodeJSONBody(c.Body(), &payload); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		if payload.Date == "" {
			return fiber.NewError(fiber.StatusBadRequest, "date is required")
		}

		day, err := time.Parse("2006-01-02", payload.Date)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "date must use YYYY-MM-DD format")
		}

		deleted, err := query.DeleteHeartbeatsForDate(c.Context(), day, payload.IDs)
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{
			"data": fiber.Map{
				"deleted": deleted,
			},
		})
	}
}

func durationsHandler(query QueryReader) fiber.Handler {
	return func(c *fiber.Ctx) error {
		timeoutMinutes, err := parseOptionalIntQuery(c.Query("timeout"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		writesOnly, err := parseOptionalBoolQuery(c.Query("writes_only"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}

		params := domain.DurationQueryParams{
			Date:           c.Query("date"),
			Project:        c.Query("project"),
			Branches:       parseCSVQuery(c.Query("branches")),
			SliceBy:        c.Query("slice_by"),
			Timezone:       c.Query("timezone"),
			TimeoutMinutes: timeoutMinutes,
			WritesOnly:     writesOnly,
		}

		items, start, end, timezone, err := query.Durations(c.Context(), params)
		if err != nil {
			return err
		}

		return c.JSON(fiber.Map{
			"data":     items,
			"start":    start.Format(time.RFC3339),
			"end":      end.Format(time.RFC3339),
			"timezone": timezone,
		})
	}
}

func summariesHandler(query QueryReader) fiber.Handler {
	return func(c *fiber.Ctx) error {
		timeoutMinutes, err := parseOptionalIntQuery(c.Query("timeout"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		writesOnly, err := parseOptionalBoolQuery(c.Query("writes_only"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}

		params := domain.SummaryQueryParams{
			Start:          c.Query("start"),
			End:            c.Query("end"),
			Range:          c.Query("range"),
			Project:        c.Query("project"),
			Branches:       parseCSVQuery(c.Query("branches")),
			Timezone:       c.Query("timezone"),
			TimeoutMinutes: timeoutMinutes,
			WritesOnly:     writesOnly,
		}

		data, err := query.Summaries(c.Context(), params)
		if err != nil {
			return err
		}

		return c.JSON(fiber.Map{"data": data})
	}
}

func statsHandler(query QueryReader) fiber.Handler {
	return func(c *fiber.Ctx) error {
		timeoutMinutes, err := parseOptionalIntQuery(c.Query("timeout"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		writesOnly, err := parseOptionalBoolQuery(c.Query("writes_only"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}

		data, err := query.Stats(c.Context(), domain.StatsQueryParams{
			Range:          c.Params("range"),
			Timezone:       c.Query("timezone"),
			TimeoutMinutes: timeoutMinutes,
			WritesOnly:     writesOnly,
		})
		if err != nil {
			return err
		}

		return c.JSON(fiber.Map{"data": data})
	}
}

func statusbarTodayHandler(query QueryReader) fiber.Handler {
	return func(c *fiber.Ctx) error {
		data, err := query.StatusbarToday(c.Context(), time.Now().UTC())
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{
			"cached_at":         time.Now().UTC().Format(time.RFC3339),
			"data":              data,
			"has_team_features": true,
		})
	}
}

func fileExpertsHandler(query QueryReader) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var payload struct {
			Entity           string `json:"entity"`
			Project          string `json:"project"`
			ProjectRootCount *int   `json:"project_root_count"`
		}
		if err := decodeJSONBody(c.Body(), &payload); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		if strings.TrimSpace(payload.Entity) == "" {
			return fiber.NewError(fiber.StatusBadRequest, "entity is required")
		}

		data, err := query.FileExperts(c.Context(), payload.Entity, payload.Project, payload.ProjectRootCount, time.Now().UTC())
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{"data": data})
	}
}

func parseDayQuery(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fiber.NewError(fiber.StatusBadRequest, "date query parameter is required")
	}

	day, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fiber.NewError(fiber.StatusBadRequest, "date must use YYYY-MM-DD format")
	}
	return day, nil
}

func parseOptionalIntQuery(value string) (*int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	var parsed int
	if _, err := fmt.Sscanf(trimmed, "%d", &parsed); err != nil {
		return nil, fmt.Errorf("invalid integer query value %q", value)
	}
	return &parsed, nil
}

func parseOptionalBoolQuery(value string) (*bool, error) {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return nil, nil
	}
	switch trimmed {
	case "1", "true", "yes", "y", "on":
		result := true
		return &result, nil
	case "0", "false", "no", "n", "off":
		result := false
		return &result, nil
	default:
		return nil, fmt.Errorf("invalid boolean query value %q", value)
	}
}

func parseCSVQuery(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func dashboardHandler(query QueryReader) fiber.Handler {
	return func(c *fiber.Ctx) error {
		rangeParam := c.Query("range", "Last 7 Days")
		start := c.Query("start")
		end := c.Query("end")
		timezone := c.Query("timezone", "UTC")

		loc, err := time.LoadLocation(timezone)
		if err != nil {
			loc = time.UTC
		}
		statsRange := dashboardStatsRange(rangeParam, time.Now().In(loc), loc)
		todayDate := time.Now().In(loc).Format("2006-01-02")

		summaryParams := domain.SummaryQueryParams{Timezone: timezone}
		if start != "" && end != "" {
			summaryParams.Start = start
			summaryParams.End = end
		} else {
			summaryParams.Range = rangeParam
		}

		type statsResult struct {
			data map[string]any
			err  error
		}
		type listResult struct {
			data []map[string]any
			err  error
		}

		statsCh := make(chan statsResult, 1)
		summariesCh := make(chan listResult, 1)
		todayCh := make(chan statsResult, 1)
		projCh := make(chan listResult, 1)
		langCh := make(chan listResult, 1)

		go func() {
			v, e := query.Stats(c.Context(), domain.StatsQueryParams{Range: statsRange, Timezone: timezone})
			statsCh <- statsResult{v, e}
		}()
		go func() {
			v, e := query.Summaries(c.Context(), summaryParams)
			summariesCh <- listResult{v, e}
		}()
		go func() {
			v, e := query.StatusbarToday(c.Context(), time.Now().UTC())
			todayCh <- statsResult{v, e}
		}()
		go func() {
			items, _, _, _, e := query.Durations(c.Context(), domain.DurationQueryParams{Date: todayDate, SliceBy: "project", Timezone: timezone})
			projCh <- listResult{items, e}
		}()
		go func() {
			items, _, _, _, e := query.Durations(c.Context(), domain.DurationQueryParams{Date: todayDate, SliceBy: "language", Timezone: timezone})
			langCh <- listResult{items, e}
		}()

		statsRes := <-statsCh
		summariesRes := <-summariesCh
		todayRes := <-todayCh
		projRes := <-projCh
		langRes := <-langCh

		var apiErrors []string
		if statsRes.err != nil {
			apiErrors = append(apiErrors, statsRes.err.Error())
		}
		if summariesRes.err != nil {
			apiErrors = append(apiErrors, summariesRes.err.Error())
		}
		if todayRes.err != nil {
			apiErrors = append(apiErrors, todayRes.err.Error())
		}
		if projRes.err != nil {
			apiErrors = append(apiErrors, projRes.err.Error())
		}
		if langRes.err != nil {
			apiErrors = append(apiErrors, langRes.err.Error())
		}

		if statsRes.data == nil {
			statsRes.data = map[string]any{}
		}
		if todayRes.data == nil {
			todayRes.data = map[string]any{}
		}
		if summariesRes.data == nil {
			summariesRes.data = []map[string]any{}
		}
		if projRes.data == nil {
			projRes.data = []map[string]any{}
		}
		if langRes.data == nil {
			langRes.data = []map[string]any{}
		}

		return c.JSON(fiber.Map{
			"stats":              statsRes.data,
			"summaries":          summariesRes.data,
			"today":              todayRes.data,
			"project_durations":  projRes.data,
			"language_durations": langRes.data,
			"errors":             apiErrors,
		})
	}
}

func cacheControlMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if c.Method() != fiber.MethodGet {
			return c.Next()
		}
		if err := c.Next(); err != nil {
			return err
		}
		if requestIncludesToday(c) {
			c.Set(fiber.HeaderCacheControl, "no-store")
		} else {
			c.Set(fiber.HeaderCacheControl, "public, max-age=86400")
		}
		return nil
	}
}

func requestIncludesToday(c *fiber.Ctx) bool {
	today := time.Now().UTC().Format("2006-01-02")
	path := c.Path()

	if strings.HasSuffix(path, "/statusbar/today") ||
		strings.HasSuffix(path, "/status_bar/today") ||
		strings.HasSuffix(path, "/dashboard") {
		return true
	}

	if date := c.Query("date"); date != "" {
		return date >= today
	}

	if end := c.Query("end"); end != "" {
		return end >= today
	}

	if rangeParam := strings.TrimSpace(c.Query("range")); rangeParam != "" {
		return !isPastPeriod(rangeParam, today)
	}

	return true
}

func isPastPeriod(rangeParam, today string) bool {
	if len(rangeParam) == 7 {
		if _, err := time.Parse("2006-01", rangeParam); err == nil {
			return rangeParam < today[:7]
		}
	}
	if len(rangeParam) == 4 {
		if _, err := time.Parse("2006", rangeParam); err == nil {
			return rangeParam < today[:4]
		}
	}
	return false
}

func dashboardStatsRange(rangeParam string, now time.Time, loc *time.Location) string {
	rangeName := strings.ToLower(strings.TrimSpace(rangeParam))
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)

	switch rangeName {
	case "this month":
		return monthStart.Format("2006-01")
	case "last month":
		return monthStart.AddDate(0, -1, 0).Format("2006-01")
	case "last year":
		return fmt.Sprintf("%04d", monthStart.Year()-1)
	case "today", "yesterday", "last 7 days", "last 7 days from yesterday", "this week", "last week":
		return "last_7_days"
	case "last 14 days", "last 30 days":
		return "last_30_days"
	default:
		if _, err := time.ParseInLocation("2006-01", rangeName, loc); err == nil {
			return rangeName
		}
		if _, err := time.ParseInLocation("2006", rangeName, loc); err == nil {
			return rangeName
		}
		return "last_7_days"
	}
}

func Shutdown(ctx context.Context, app *fiber.App) error {
	done := make(chan error, 1)
	go func() {
		done <- app.Shutdown()
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("fiber shutdown timed out: %w", ctx.Err())
	case err := <-done:
		return err
	}
}

func decodeJSONBody(body []byte, dst any) error {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return errors.New("empty body")
	}
	for i := 0; i < 2; i++ {
		if strings.HasPrefix(trimmed, "\"") {
			var inner string
			if err := json.Unmarshal([]byte(trimmed), &inner); err != nil {
				return err
			}
			trimmed = strings.TrimSpace(inner)
			continue
		}
		break
	}
	return json.Unmarshal([]byte(trimmed), dst)
}

func stringOrNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
