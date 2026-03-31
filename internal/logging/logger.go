package logging

import (
	"os"
	"strings"

	"github.com/rs/zerolog"
)

func New(level, format string) zerolog.Logger {
	var parsedLevel zerolog.Level
	switch strings.ToLower(level) {
	case "debug":
		parsedLevel = zerolog.DebugLevel
	case "warn", "warning":
		parsedLevel = zerolog.WarnLevel
	case "error":
		parsedLevel = zerolog.ErrorLevel
	default:
		parsedLevel = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(parsedLevel)
	if strings.EqualFold(format, "text") {
		return zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout}).With().Timestamp().Logger()
	}
	return zerolog.New(os.Stdout).With().Timestamp().Logger()
}
