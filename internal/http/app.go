package apihttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	Activity(ctx context.Context, params domain.ActivityQueryParams, now time.Time) (domain.ActivityOverview, error)
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
	registerDashboardRoutes(app, services)
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
}

func registerDashboardRoutes(app *fiber.App, services Services) {
	api := app.Group("/api/v2")
	api.Use(cacheControlMiddleware())
	api.Get("/dashboard", dashboardHandler(services.Query))
	api.Get("/project", projectDetailHandler(services.Query))
	api.Get("/live", liveDashboardHandler(services.Query))
	api.Get("/insights", insightsHandler(services.Query))
	api.Get("/wrapped", wrappedHandler(services.Query))
}

func projectDetailHandler(query QueryReader) fiber.Handler {
	return func(c *fiber.Ctx) error {
		project := strings.TrimSpace(c.Query("project"))
		if project == "" {
			return fiber.NewError(fiber.StatusBadRequest, "project is required")
		}

		timezone := c.Query("timezone", "UTC")
		params := domain.SummaryQueryParams{
			Start:    c.Query("start"),
			End:      c.Query("end"),
			Range:    c.Query("range", "Last 30 Days"),
			Project:  project,
			Timezone: timezone,
		}
		if params.Start != "" || params.End != "" {
			if params.Start == "" || params.End == "" {
				return fiber.NewError(fiber.StatusBadRequest, "start and end must be provided together")
			}
			params.Range = ""
		}

		summaries, err := query.Summaries(c.Context(), params)
		if err != nil {
			return err
		}
		if summaries == nil {
			summaries = []map[string]any{}
		}

		return c.JSON(fiber.Map{
			"project":      project,
			"timezone":     timezone,
			"summaries":    summaries,
			"generated_at": time.Now().UTC().Format(time.RFC3339),
		})
	}
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

	indexHandler := websiteIndexHandler(indexPath, cfg)
	app.Get("/", indexHandler)
	app.Head("/", indexHandler)

	app.Static("/", distDir, fiber.Static{
		Browse:   false,
		Compress: true,
		Index:    "index.html",
	})

	app.Get("/*", indexHandler)
	app.Head("/*", indexHandler)
}

func websiteIndexHandler(indexPath string, cfg *config.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !shouldServeWebsiteIndex(c.Path()) {
			return fiber.ErrNotFound
		}
		html, err := os.ReadFile(indexPath)
		if err != nil {
			return fmt.Errorf("read website index: %w", err)
		}
		c.Type("html", "utf-8")
		if c.Method() == fiber.MethodHead {
			return c.SendStatus(fiber.StatusOK)
		}
		return c.SendString(injectDashboardRuntimeConfig(string(html), cfg))
	}
}

func injectDashboardRuntimeConfig(html string, cfg *config.Config) string {
	payload, err := json.Marshal(map[string]string{
		"apiBase":  "",
		"timezone": cfg.AppTimezone,
	})
	if err != nil {
		return html
	}

	script := `<script>window.__WAKA_DASHBOARD_CONFIG__=` + string(payload) + `;</script>`
	if strings.Contains(html, "</head>") {
		return strings.Replace(html, "</head>", script+"</head>", 1)
	}
	return script + html
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

		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
			"data": heartbeatResponseData(&records[0]),
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
		responses := make([][]any, 0, len(records))
		for i := range records {
			record := &records[i]
			item := heartbeatResponseData(record)
			items = append(items, item)
			responses = append(responses, []any{
				fiber.Map{"data": item},
				fiber.StatusAccepted,
			})
		}
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
			"responses": responses,
			"data": fiber.Map{
				"accepted":   len(records),
				"heartbeats": items,
			},
		})
	}
}

func heartbeatResponseData(record *domain.HeartbeatRecord) fiber.Map {
	return fiber.Map{
		"id":     record.ID,
		"entity": record.Entity,
		"type":   record.Type,
		"time":   float64(record.Time.UnixNano()) / float64(time.Second),
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
				"id":                   record.ID,
				"entity":               record.Entity,
				"type":                 record.Type,
				"category":             record.Category,
				"time":                 float64(record.Time.UnixNano()) / float64(time.Second),
				"project":              stringOrNil(record.Project),
				"project_root_count":   record.ProjectRootCount,
				"branch":               stringOrNil(record.Branch),
				"language":             stringOrNil(record.Language),
				"dependencies":         record.Dependencies,
				"machine_name_id":      stringOrNil(record.SourceMachineNameID),
				"ai_line_changes":      record.AILineChanges,
				"human_line_changes":   record.HumanLineChanges,
				"ai_session":           stringOrNil(record.AISession),
				"ai_subscription_plan": stringOrNil(record.AISubscriptionPlan),
				"ai_input_tokens":      record.AIInputTokens,
				"ai_output_tokens":     record.AIOutputTokens,
				"ai_prompt_length":     record.AIPromptLength,
				"lines":                record.Lines,
				"lineno":               record.Lineno,
				"cursorpos":            record.Cursorpos,
				"is_write":             record.IsWrite,
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

// tokenMetrics calculates token usage from stats
func tokenMetrics(stats map[string]any) map[string]any {
	inputTokens := toInt64(stats["ai_input_tokens"])
	outputTokens := toInt64(stats["ai_output_tokens"])
	totalTokens := inputTokens + outputTokens
	return fiber.Map{
		"total_tokens":  totalTokens,
		"input_tokens":  inputTokens,
		"output_tokens": outputTokens,
	}
}

// spendMetrics sums per-model spend_cents from ai_models bucket (pricing from DB).
// Falls back to flat Claude Sonnet rate when no model data is present.
func spendMetrics(stats map[string]any) map[string]any {
	var totalCents, inputTokens, outputTokens int64
	if aiModels, ok := stats["ai_models"].([]map[string]any); ok {
		for _, m := range aiModels {
			totalCents += toInt64(m["spend_cents"])
			inputTokens += toInt64(m["input_tokens"])
			outputTokens += toInt64(m["output_tokens"])
		}
	}
	// ponytail: flat-rate fallback when no model data
	if totalCents == 0 {
		inputTokens = toInt64(stats["ai_input_tokens"])
		outputTokens = toInt64(stats["ai_output_tokens"])
		totalCents = int64(float64(inputTokens)*0.003/1000*100) + int64(float64(outputTokens)*0.015/1000*100)
	}
	return fiber.Map{
		"estimated_cents": totalCents,
		"token_count":     inputTokens + outputTokens,
		"input_tokens":    inputTokens,
		"output_tokens":   outputTokens,
	}
}

func toInt64(v any) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case float64:
		return int64(val)
	default:
		return 0
	}
}

type parallelQueryResult[T any] struct {
	data T
	err  error
}

func runParallel(tasks ...func()) {
	var waitGroup sync.WaitGroup
	waitGroup.Add(len(tasks))
	for _, task := range tasks {
		go func() {
			defer waitGroup.Done()
			task()
		}()
	}
	waitGroup.Wait()
}

func queryErrorMessages(errs ...error) []string {
	var messages []string
	for _, err := range errs {
		if err != nil {
			messages = append(messages, err.Error())
		}
	}
	return messages
}

func mapOrEmpty(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}

func listOrEmpty(value []map[string]any) []map[string]any {
	if value == nil {
		return []map[string]any{}
	}
	return value
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
		durationDate := dashboardDurationDate(rangeParam, start, end, time.Now().In(loc))

		summaryParams := domain.SummaryQueryParams{Timezone: timezone}
		if start != "" && end != "" {
			summaryParams.Start = start
			summaryParams.End = end
		} else {
			summaryParams.Range = rangeParam
		}

		var statsRes, todayRes parallelQueryResult[map[string]any]
		var summariesRes, projRes, langRes, editorRes parallelQueryResult[[]map[string]any]
		runParallel(
			func() {
				statsRes.data, statsRes.err = query.Stats(c.Context(), domain.StatsQueryParams{Range: statsRange, Start: start, End: end, Timezone: timezone})
			},
			func() {
				summariesRes.data, summariesRes.err = query.Summaries(c.Context(), summaryParams)
			},
			func() {
				todayRes.data, todayRes.err = query.StatusbarToday(c.Context(), time.Now().UTC())
			},
			func() {
				projRes.data, _, _, _, projRes.err = query.Durations(c.Context(), domain.DurationQueryParams{Date: durationDate, SliceBy: "project", Timezone: timezone})
			},
			func() {
				langRes.data, _, _, _, langRes.err = query.Durations(c.Context(), domain.DurationQueryParams{Date: durationDate, SliceBy: "language", Timezone: timezone})
			},
			func() {
				editorRes.data, _, _, _, editorRes.err = query.Durations(c.Context(), domain.DurationQueryParams{Date: durationDate, SliceBy: "editor", Timezone: timezone})
			},
		)

		apiErrors := queryErrorMessages(statsRes.err, summariesRes.err, todayRes.err, projRes.err, langRes.err, editorRes.err)
		statsRes.data = mapOrEmpty(statsRes.data)
		todayRes.data = mapOrEmpty(todayRes.data)
		summariesRes.data = listOrEmpty(summariesRes.data)
		projRes.data = listOrEmpty(projRes.data)
		langRes.data = listOrEmpty(langRes.data)
		editorRes.data = listOrEmpty(editorRes.data)

		tokens := tokenMetrics(statsRes.data)
		spend := spendMetrics(statsRes.data)

		return c.JSON(fiber.Map{
			"stats":              statsRes.data,
			"summaries":          summariesRes.data,
			"today":              todayRes.data,
			"project_durations":  projRes.data,
			"language_durations": langRes.data,
			"editor_durations":   editorRes.data,
			"token_metrics":      tokens,
			"spend_metrics":      spend,
			"errors":             apiErrors,
		})
	}
}

func liveDashboardHandler(query QueryReader) fiber.Handler {
	return func(c *fiber.Ctx) error {
		timezone := c.Query("timezone", "UTC")
		loc, err := time.LoadLocation(timezone)
		if err != nil {
			loc = time.UTC
		}

		now := time.Now().UTC()
		todayDate := now.In(loc).Format("2006-01-02")

		var todayRes parallelQueryResult[map[string]any]
		var projRes, langRes, editorRes parallelQueryResult[[]map[string]any]
		runParallel(
			func() {
				todayRes.data, todayRes.err = query.StatusbarToday(c.Context(), now)
			},
			func() {
				projRes.data, _, _, _, projRes.err = query.Durations(c.Context(), domain.DurationQueryParams{Date: todayDate, SliceBy: "project", Timezone: timezone})
			},
			func() {
				langRes.data, _, _, _, langRes.err = query.Durations(c.Context(), domain.DurationQueryParams{Date: todayDate, SliceBy: "language", Timezone: timezone})
			},
			func() {
				editorRes.data, _, _, _, editorRes.err = query.Durations(c.Context(), domain.DurationQueryParams{Date: todayDate, SliceBy: "editor", Timezone: timezone})
			},
		)

		apiErrors := queryErrorMessages(todayRes.err, projRes.err, langRes.err, editorRes.err)
		todayRes.data = mapOrEmpty(todayRes.data)
		projRes.data = listOrEmpty(projRes.data)
		langRes.data = listOrEmpty(langRes.data)
		editorRes.data = listOrEmpty(editorRes.data)

		return c.JSON(fiber.Map{
			"cached_at":          now.Format(time.RFC3339),
			"status":             "synchronized",
			"today":              todayRes.data,
			"project_durations":  projRes.data,
			"language_durations": langRes.data,
			"editor_durations":   editorRes.data,
			"errors":             apiErrors,
		})
	}
}

func insightsHandler(query QueryReader) fiber.Handler {
	return func(c *fiber.Ctx) error {
		timezone := c.Query("timezone", "UTC")
		rangeParam := c.Query("range", "Last 7 Days")

		loc, err := time.LoadLocation(timezone)
		if err != nil {
			loc = time.UTC
		}
		statsRange := dashboardStatsRange(rangeParam, time.Now().In(loc), loc)

		stats, err := query.Stats(c.Context(), domain.StatsQueryParams{
			Range:    statsRange,
			Timezone: timezone,
		})
		if err != nil {
			return err
		}
		if stats == nil {
			stats = map[string]any{}
		}

		summaries, err := query.Summaries(c.Context(), domain.SummaryQueryParams{
			Range:    rangeParam,
			Timezone: timezone,
		})
		if err != nil {
			return err
		}
		if summaries == nil {
			summaries = []map[string]any{}
		}

		tokens := tokenMetrics(stats)
		spend := spendMetrics(stats)

		return c.JSON(fiber.Map{
			"timezone":      timezone,
			"range":         rangeParam,
			"summaries":     summaries,
			"stats":         stats,
			"token_metrics": tokens,
			"spend_metrics": spend,
			"generated_at":  time.Now().UTC().Format(time.RFC3339),
		})
	}
}

func wrappedHandler(query QueryReader) fiber.Handler {
	return func(c *fiber.Ctx) error {
		timezone := c.Query("timezone", "UTC")
		year := strings.TrimSpace(c.Query("year"))
		if year == "" {
			loc, err := time.LoadLocation(timezone)
			if err != nil {
				loc = time.UTC
			}
			year = time.Now().In(loc).Format("2006")
		}
		parsedYear, err := time.Parse("2006", year)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "year must use YYYY format")
		}

		summaries, summaryErr := query.Summaries(c.Context(), domain.SummaryQueryParams{Range: year, Timezone: timezone})
		apiErrors := queryErrorMessages(summaryErr)
		summaries = listOrEmpty(summaries)
		activity, activityErr := query.Activity(c.Context(), domain.ActivityQueryParams{
			Timezone: timezone,
			Year:     parsedYear.Year(),
		}, time.Now().UTC())
		apiErrors = append(apiErrors, queryErrorMessages(activityErr)...)

		stats := wrappedStatsFromSummaries(summaries)
		if activityErr == nil {
			stats["ai_input_tokens"] = activity.TokenUsage.InputTokens
			stats["ai_output_tokens"] = activity.TokenUsage.OutputTokens
			stats["ai_total_tokens"] = activity.TokenUsage.TotalTokens
		}
		tokens := tokenMetrics(stats)
		spend := spendMetrics(stats)

		return c.JSON(fiber.Map{
			"timezone":      timezone,
			"year":          year,
			"stats":         stats,
			"summaries":     summaries,
			"token_metrics": tokens,
			"spend_metrics": spend,
			"activity":      activity,
			"total_days":    len(summaries),
			"generated_at":  time.Now().UTC().Format(time.RFC3339),
			"errors":        apiErrors,
		})
	}
}

func wrappedStatsFromSummaries(summaries []map[string]any) map[string]any {
	if len(summaries) == 0 {
		return map[string]any{}
	}

	totalSeconds := 0.0
	dailyAverage := 0.0
	bestDaySeconds := 0.0
	bestDayDate := ""
	bestDayText := "0s"
	activeDays := 0
	var aiAdditions, aiDeletions, humanAdditions, humanDeletions int64
	var aiInputTokens, aiOutputTokens int64

	for _, summary := range summaries {
		grandTotal := nestedMap(summary, "grand_total")
		seconds := toFloat64(grandTotal["total_seconds"])
		totalSeconds += seconds
		if seconds > 0 {
			activeDays++
		}

		aiAdditions += toInt64(grandTotal["ai_additions"])
		aiDeletions += toInt64(grandTotal["ai_deletions"])
		humanAdditions += toInt64(grandTotal["human_additions"])
		humanDeletions += toInt64(grandTotal["human_deletions"])
		aiInputTokens += toInt64(grandTotal["ai_input_tokens"])
		aiOutputTokens += toInt64(grandTotal["ai_output_tokens"])

		if seconds > bestDaySeconds {
			bestDaySeconds = seconds
			bestDayDate = nestedString(summary, "range", "date")
			bestDayText = strings.TrimSpace(toString(grandTotal["text"]))
			if bestDayText == "" {
				bestDayText = humanizeTotalSeconds(seconds)
			}
		}
	}

	dailyAverage = totalSeconds / float64(len(summaries))

	return map[string]any{
		"total_seconds_including_other_language":                totalSeconds,
		"human_readable_total_including_other_language":         humanizeTotalSeconds(totalSeconds),
		"daily_average_including_other_language":                dailyAverage,
		"human_readable_daily_average_including_other_language": humanizeTotalSeconds(dailyAverage),
		"ai_additions":            aiAdditions,
		"ai_deletions":            aiDeletions,
		"human_additions":         humanAdditions,
		"human_deletions":         humanDeletions,
		"ai_input_tokens":         aiInputTokens,
		"ai_output_tokens":        aiOutputTokens,
		"ai_total_tokens":         aiInputTokens + aiOutputTokens,
		"days_including_holidays": len(summaries),
		"days_minus_holidays":     activeDays,
		"best_day": map[string]any{
			"date":          bestDayDate,
			"text":          bestDayText,
			"total_seconds": bestDaySeconds,
		},
	}
}

func nestedMap(values map[string]any, key string) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	if nested, ok := values[key].(map[string]any); ok && nested != nil {
		return nested
	}
	return map[string]any{}
}

func nestedString(values map[string]any, firstKey, secondKey string) string {
	return toString(nestedMap(values, firstKey)[secondKey])
}

func toFloat64(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	default:
		return 0
	}
}

func toString(v any) string {
	if value, ok := v.(string); ok {
		return value
	}
	return ""
}

func humanizeTotalSeconds(totalSeconds float64) string {
	seconds := int(totalSeconds + 0.5)
	if seconds <= 0 {
		return "0s"
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	remaining := seconds % 60
	if hours > 0 {
		return fmt.Sprintf("%dh %02dm", hours, minutes)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%ds", remaining)
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
		strings.HasSuffix(path, "/dashboard") ||
		strings.HasSuffix(path, "/live") {
		return true
	}
	if strings.HasSuffix(path, "/wrapped") {
		year := strings.TrimSpace(c.Query("year"))
		return year == "" || year >= today[:4]
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
	case "today":
		return "today"
	case "yesterday":
		return "yesterday"
	case "last 7 days", "last 7 days from yesterday", "this week":
		return "last_7_days"
	case "last week":
		return "last_week"
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

func dashboardDurationDate(rangeParam, start, end string, now time.Time) string {
	if strings.TrimSpace(start) != "" && start == end {
		return start
	}

	switch strings.ToLower(strings.TrimSpace(rangeParam)) {
	case "yesterday":
		return now.AddDate(0, 0, -1).Format("2006-01-02")
	default:
		return now.Format("2006-01-02")
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
