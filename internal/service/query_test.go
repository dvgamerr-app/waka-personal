package service

import (
	"testing"
	"time"

	"waka-personal/internal/domain"
)

func TestSummarizeHeartbeats(t *testing.T) {
	base := time.Date(2026, 3, 31, 9, 0, 0, 0, time.UTC)
	heartbeats := []domain.HeartbeatRecord{
		{Time: base, Category: "coding"},
		{Time: base.Add(2 * time.Minute), Category: "coding"},
		{Time: base.Add(20 * time.Minute), Category: "building"},
	}

	categories, total := summarizeHeartbeats(heartbeats, 15*time.Minute, base.Add(40*time.Minute), base.Add(25*time.Minute))
	if total != 1320 {
		t.Fatalf("expected total 1320 seconds, got %.0f", total)
	}
	if len(categories) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(categories))
	}
	if categories[0].Name != "Coding" || categories[0].TotalSeconds != 1020 {
		t.Fatalf("unexpected coding category: %#v", categories[0])
	}
	if categories[1].Name != "Building" || categories[1].TotalSeconds != 300 {
		t.Fatalf("unexpected building category: %#v", categories[1])
	}
}

func TestHumanizeDuration(t *testing.T) {
	if got := humanizeDuration(3720); got != "1 hr 2 mins" {
		t.Fatalf("unexpected duration text: %s", got)
	}
}
