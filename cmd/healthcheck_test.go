package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthcheck(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "ready", status: http.StatusOK},
		{name: "not ready", status: http.StatusServiceUnavailable, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer server.Close()

			err := healthcheck(context.Background(), server.URL)
			if (err != nil) != tt.wantErr {
				t.Fatalf("healthcheck() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHealthcheckUnreachable(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL
	server.Close()

	if err := healthcheck(context.Background(), url); err == nil {
		t.Fatal("expected error for unreachable server")
	}
}
