//go:build cgo && ((windows && amd64) || (linux && (amd64 || arm64)) || (darwin && (amd64 || arm64)))

package service

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/duckdb/duckdb-go/v2"
)

func flattenBackupToCSV(ctx context.Context, inputPath, outputCSV string) error {
	db, err := sql.Open("duckdb", "")
	if err != nil {
		return fmt.Errorf("open duckdb client: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping duckdb client: %w", err)
	}

	if _, err := db.ExecContext(ctx, buildDuckDBSQL(duckDBPath(inputPath), duckDBPath(outputCSV))); err != nil {
		return fmt.Errorf("duckdb client import failed: %w", err)
	}
	return nil
}
