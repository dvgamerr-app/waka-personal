package main

import (
	"context"
	"errors"
	"flag"
	"os"

	_ "github.com/joho/godotenv/autoload"
	"github.com/rs/zerolog/log"

	"waka-personal/internal/config"
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

	var filePath string
	var format string
	var dryRun bool
	flag.StringVar(&filePath, "file", "", "absolute or relative path to backup json/json.gz")
	flag.StringVar(&format, "format", "backup-json", "import format")
	flag.BoolVar(&dryRun, "dry-run", false, "validate metadata without importing heartbeats")
	flag.Parse()

	if filePath == "" {
		err := errors.New("--file is required")
		logger.Error().Err(err).Msg("invalid importer arguments")
		return err
	}

	ctx := context.Background()
	if err := store.ApplyMigrations(ctx, cfg.DatabaseURL, cfg.MigrationDir, cfg.GooseTable); err != nil {
		logger.Error().Err(err).Msg("failed to apply migrations")
		return err
	}

	db, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error().Err(err).Msg("failed to connect to database")
		return err
	}
	defer db.Close()

	dataStore := store.New(db)
	importer := service.NewImportService(dataStore)
	result, err := importer.ImportBackupFile(ctx, service.ImportOptions{
		FilePath: filePath,
		Format:   format,
		DryRun:   dryRun,
	})
	if err != nil {
		logger.Error().Err(err).Msg("backup import failed")
		return err
	}

	logger.Info().
		Str("batch_id", result.BatchID).
		Int64("imported_rows", result.ImportedRows).
		Int64("skipped_rows", result.SkippedRows).
		Msg("backup import completed")
	return nil
}
