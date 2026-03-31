package main

import (
	"context"
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
		logger.Error().Msg("--file is required")
		os.Exit(1)
	}

	ctx := context.Background()
	if err := store.ApplyMigrations(ctx, cfg.DatabaseURL, cfg.MigrationDir, cfg.GooseTable); err != nil {
		logger.Error().Err(err).Msg("failed to apply migrations")
		os.Exit(1)
	}

	db, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error().Err(err).Msg("failed to connect to database")
		os.Exit(1)
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
		os.Exit(1)
	}

	logger.Info().
		Str("batch_id", result.BatchID).
		Int64("imported_rows", result.ImportedRows).
		Int64("skipped_rows", result.SkippedRows).
		Msg("backup import completed")
}
