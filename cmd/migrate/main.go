package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	_ "github.com/joho/godotenv/autoload"
	"github.com/rs/zerolog/log"

	"waka-personal/internal/config"
	"waka-personal/internal/logging"
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

	var dir string
	var table string
	var verbose bool
	flag.StringVar(&dir, "dir", cfg.MigrationDir, "directory containing goose migrations")
	flag.StringVar(&table, "table", cfg.GooseTable, "goose version table name")
	flag.BoolVar(&verbose, "v", false, "enable verbose goose output")
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintln(out, "Usage: go run ./cmd/migrate [flags] <command> [args]")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Commands: up, up-by-one, up-to, down, down-to, redo, reset, status, version, create, fix, validate")
		fmt.Fprintln(out)
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		return fmt.Errorf("migration command is required")
	}

	command := args[0]
	commandArgs := args[1:]
	if err := store.RunMigrationCommand(context.Background(), cfg.DatabaseURL, dir, table, verbose, command, commandArgs...); err != nil {
		logger.Error().
			Str("command", command).
			Str("migration_dir", dir).
			Err(err).
			Msg("migration command failed")
		return err
	}

	return nil
}
