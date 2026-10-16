package main

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const healthcheckTimeout = 3 * time.Second

// healthcheck lets the container image probe itself without shipping curl.
func healthcheck(ctx context.Context, url string) error {
	ctx, cancel := context.WithTimeout(ctx, healthcheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("build healthcheck request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}
	return nil
}
