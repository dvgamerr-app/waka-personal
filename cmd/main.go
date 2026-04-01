package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/rs/zerolog/log"

	"waka-personal/internal/config"
	apihttp "waka-personal/internal/http"
	"waka-personal/internal/logging"
	"waka-personal/internal/service"
	"waka-personal/internal/store"
)

func main() {
	if err := run(); err != nil {
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	logger := logging.New(cfg.LogLevel, cfg.LogFormat)
	log.Logger = logger

	ctx := context.Background()
	db, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error().Err(err).Msg("failed to connect to database")
		return err
	}
	defer db.Close()

	checker := &apihttp.Checker{}
	checker.SetReady(true)

	dataStore := store.New(db)
	profileService := service.NewProfileService(dataStore, cfg)
	app := apihttp.NewApp(cfg, checker, apihttp.Services{
		Auth:       service.NewAuthService(cfg.AppAPIKey),
		Heartbeats: service.NewHeartbeatService(dataStore),
		Query:      service.NewQueryService(dataStore, profileService),
	})

	logger.Info().Str("port", cfg.HTTPPort).Msg("Starting Gofiber")
	go func() {
		if err := app.Listen(":" + cfg.HTTPPort); err != nil && !strings.Contains(strings.ToLower(err.Error()), "server closed") {
			logger.Error().Err(err).Msg("fiber server exited")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info().Str("signal", sig.String()).Msg("shutdown signal received")

	checker.SetReady(false)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := apihttp.Shutdown(shutdownCtx, app); err != nil {
		logger.Error().Err(err).Msg("failed to shutdown fiber app")
		return err
	}
	logger.Info().Msg("shutdown complete")
	return nil
}
