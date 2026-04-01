package http

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
	"waka-personal/internal/service"
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
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			status := fiber.StatusInternalServerError
			if fiberErr, ok := err.(*fiber.Error); ok {
				status = fiberErr.Code
			}
			if errors.Is(err, service.ErrUnauthorized) {
				status = fiber.StatusUnauthorized
			}
			if err == nil {
				err = errors.New("unknown error")
			}
			return c.Status(status).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	app.Use(requestid.New())
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     strings.Join(cfg.CORSAllowOrigins, ","),
		AllowHeaders:     "Authorization, Content-Type, X-Machine-Name",
		AllowMethods:     "GET,POST,DELETE,OPTIONS",
		AllowCredentials: false,
	}))
	app.Use("/api", apiDebugLogger())

	app.Get("/healthz/live", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	app.Get("/healthz/ready", func(c *fiber.Ctx) error {
		if !checker.IsReady() {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "not ready"})
		}
		return c.JSON(fiber.Map{"status": "ready"})
	})

	api := app.Group("/api/v1/users/current", func(c *fiber.Ctx) error {
		return services.Auth.Validate(c.Query("api_key"), c.Get("Authorization"))
	})

	api.Post("/heartbeats", func(c *fiber.Ctx) error {
		records, err := services.Heartbeats.Ingest(c.Context(), c.Body(), c.Get("X-Machine-Name"), nil)
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
	})

	api.Post("/heartbeats.bulk", func(c *fiber.Ctx) error {
		records, err := services.Heartbeats.Ingest(c.Context(), c.Body(), c.Get("X-Machine-Name"), nil)
		if err != nil {
			return err
		}
		items := make([]fiber.Map, 0, len(records))
		for _, record := range records {
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
	})

	api.Get("/heartbeats", func(c *fiber.Ctx) error {
		dateValue := c.Query("date")
		if dateValue == "" {
			return fiber.NewError(fiber.StatusBadRequest, "date query parameter is required")
		}
		day, err := time.Parse("2006-01-02", dateValue)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "date must use YYYY-MM-DD format")
		}

		records, start, end, timezone, err := services.Query.HeartbeatsForDate(c.Context(), day)
		if err != nil {
			return err
		}

		items := make([]fiber.Map, 0, len(records))
		for _, record := range records {
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
	})

	api.Delete("/heartbeats.bulk", func(c *fiber.Ctx) error {
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

		deleted, err := services.Query.DeleteHeartbeatsForDate(c.Context(), day, payload.IDs)
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{
			"data": fiber.Map{
				"deleted": deleted,
			},
		})
	})

	api.Get("/statusbar/today", func(c *fiber.Ctx) error {
		data, err := services.Query.StatusbarToday(c.Context(), time.Now().UTC())
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{
			"data":              data,
			"has_team_features": data.HasTeamFeatures,
		})
	})

	api.Post("/file_experts", func(c *fiber.Ctx) error {
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

		data, err := services.Query.FileExperts(c.Context(), payload.Entity, payload.Project, payload.ProjectRootCount, time.Now().UTC())
		if err != nil {
			return err
		}
		return c.JSON(fiber.Map{"data": data})
	})

	return app
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
