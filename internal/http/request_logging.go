package http

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"

	"waka-personal/internal/service"
)

func apiDebugLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		startedAt := time.Now()
		method := c.Method()
		path := c.OriginalURL()
		ip := c.IP()
		bytesIn := len(c.Body())

		err := c.Next()
		status := c.Response().StatusCode()
		if err != nil {
			status = statusCodeForError(err)
		}

		event := log.Debug().
			Str("method", method).
			Str("path", path).
			Str("ip", ip).
			Int("status", status).
			Int("bytes_in", bytesIn).
			Dur("duration", time.Since(startedAt))

		if requestID := c.GetRespHeader(fiber.HeaderXRequestID); requestID != "" {
			event = event.Str("request_id", requestID)
		}
		if err != nil {
			event = event.Err(err)
		}

		event.Msg("api request")
		return err
	}
}

func statusCodeForError(err error) int {
	if err == nil {
		return fiber.StatusOK
		
	}

	status := fiber.StatusInternalServerError
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		status = fiberErr.Code
	}
	if errors.Is(err, service.ErrUnauthorized) {
		status = fiber.StatusUnauthorized
	}

	return status
}
