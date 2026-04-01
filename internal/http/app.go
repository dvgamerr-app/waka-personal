package apihttp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
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
	StatusbarToday(ctx context.Context, now time.Time) (domain.StatusbarTodayData, error)
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
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Join(cfg.CORSAllowOrigins, ","),
		AllowHeaders:     "Authorization, Content-Type, X-Machine-Name",
		AllowMethods:     "GET,POST,DELETE,OPTIONS",
		AllowCredentials: false,
	}))
	app.Use("/api", apiDebugLogger())
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
	api.Post("/heartbeats", postHeartbeatHandler(services.Heartbeats))
	api.Post("/heartbeats.bulk", postBulkHeartbeatsHandler(services.Heartbeats))
	api.Get("/heartbeats", getHeartbeatsHandler(services.Query))
	api.Delete("/heartbeats.bulk", deleteHeartbeatsHandler(services.Query))
	api.Get("/statusbar/today", statusbarTodayHandler(services.Query))
	api.Post("/file_experts", fileExpertsHandler(services.Query))
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

func statusbarTodayHandler(query QueryReader) fiber.Handler {
	return func(c *fiber.Ctx) error {
		data, err := query.StatusbarToday(c.Context(), time.Now().UTC())
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{
			"data":              data,
			"has_team_features": data.HasTeamFeatures,
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
