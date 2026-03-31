package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

const defaultGooseTable = "goose_db_version"

func ApplyMigrations(ctx context.Context, databaseURL, dir, tableName string) error {
	return RunMigrationCommand(ctx, databaseURL, dir, tableName, false, "up")
}

func RunMigrationCommand(
	ctx context.Context,
	databaseURL, dir, tableName string,
	verbose bool,
	command string,
	args ...string,
) error {
	dir = filepath.Clean(dir)

	switch command {
	case "create":
		if len(args) != 2 {
			return fmt.Errorf("goose create requires NAME and TYPE arguments")
		}
		if err := goose.Create(nil, dir, args[0], args[1]); err != nil {
			return fmt.Errorf("run goose create: %w", err)
		}
		return nil
	case "fix":
		if err := goose.Fix(dir); err != nil {
			return fmt.Errorf("run goose fix: %w", err)
		}
		return nil
	}

	db, err := openMigrationDB(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	goose.SetVerbose(verbose)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	if strings.TrimSpace(tableName) == "" {
		tableName = defaultGooseTable
	}
	goose.SetTableName(tableName)

	if err := goose.RunContext(ctx, command, db, dir, args...); err != nil {
		return fmt.Errorf("run goose %s: %w", command, err)
	}
	return nil
}

func openMigrationDB(ctx context.Context, databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open migration database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping migration database: %w", err)
	}
	return db, nil
}
