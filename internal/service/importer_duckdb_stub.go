//go:build !(cgo && ((windows && amd64) || (linux && (amd64 || arm64)) || (darwin && (amd64 || arm64))))

package service

import (
	"context"
	"fmt"
	"runtime"
)

func flattenBackupToCSV(_ context.Context, _, _ string) error {
	return fmt.Errorf(
		"duckdb go client is unavailable for %s/%s with CGO disabled; enable CGO and install a supported C toolchain to run importer",
		runtime.GOOS,
		runtime.GOARCH,
	)
}
