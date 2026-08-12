package service

import (
	"testing"
	"time"

	"waka-personal/internal/domain"
)

func TestBuildActivityOverviewCalculatesBackendMetrics(t *testing.T) {
	loc := time.UTC
	windowStart := time.Date(2026, time.January, 1, 0, 0, 0, 0, loc)
	windowEnd := time.Date(2026, time.January, 6, 0, 0, 0, 0, loc)
	now := time.Date(2026, time.January, 5, 12, 0, 0, 0, loc)

	record := func(day int, hour, minute int, entity string, isWrite bool) domain.HeartbeatRecord {
		return domain.HeartbeatRecord{
			Time:    time.Date(2026, time.January, day, hour, minute, 0, 0, loc),
			Entity:  entity,
			IsWrite: isWrite,
		}
	}

	heartbeats := []domain.HeartbeatRecord{
		record(1, 9, 0, "day-1-a.go", true),
		record(1, 9, 5, "day-1-b.go", true),
		record(2, 9, 0, "ignored-non-write.go", false),
		record(3, 10, 0, "day-3.go", true),
		record(4, 10, 0, "day-4.go", true),
		record(5, 10, 0, "day-5.go", true),
	}

	aiTaskA := record(4, 11, 0, "agent-a.go", false)
	aiTaskA.AISession = "session-a"
	aiTaskA.AIAgentName = "Claude Code"
	aiTaskA.Project = "waka-personal"
	aiTaskB := record(4, 11, 10, "agent-b.go", false)
	aiTaskB.AISession = "session-a"
	aiTaskB.AIAgentName = "Claude Code"
	aiTaskB.Project = "waka-personal"
	heartbeats = append(heartbeats, aiTaskA, aiTaskB)

	overview := buildActivityOverview(
		heartbeats,
		100,
		50,
		querySettings{
			location:       loc,
			timezone:       "UTC",
			timeout:        15 * time.Minute,
			timeoutMinutes: 15,
			writesOnly:     true,
		},
		2026,
		windowStart,
		windowEnd,
		365,
		now,
	)

	if len(overview.Days) != 5 {
		t.Fatalf("expected 5 activity days, got %d", len(overview.Days))
	}
	if overview.TotalSeconds != 3900 {
		t.Fatalf("expected 3900 total seconds, got %v", overview.TotalSeconds)
	}
	if overview.ActiveDays != 4 {
		t.Fatalf("expected 4 active days, got %d", overview.ActiveDays)
	}
	if overview.AverageActiveDaySeconds != 975 {
		t.Fatalf("expected 975 average active-day seconds, got %v", overview.AverageActiveDaySeconds)
	}
	if overview.PeakDay.Date != "2026-01-01" || overview.PeakDay.TotalSeconds != 1200 {
		t.Fatalf("unexpected peak day: %#v", overview.PeakDay)
	}
	if overview.LongestStreakDays != 3 || overview.LongestStreakStart != "2026-01-03" || overview.LongestStreakEnd != "2026-01-05" {
		t.Fatalf("unexpected longest streak: %#v", overview)
	}
	if overview.TokenUsage.TotalTokens != 150 {
		t.Fatalf("expected 150 yearly tokens, got %d", overview.TokenUsage.TotalTokens)
	}
	if overview.Year != 2026 || overview.CalendarDays != 365 {
		t.Fatalf("unexpected annual metadata: %#v", overview)
	}
	if overview.LongestTask.TotalSeconds != 1500 {
		t.Fatalf("expected 1500 second AI task, got %v", overview.LongestTask.TotalSeconds)
	}
	if overview.LongestTask.HeartbeatCount != 2 || overview.LongestTask.EntityCount != 2 {
		t.Fatalf("unexpected longest task counts: %#v", overview.LongestTask)
	}
	if overview.LongestTask.AgentName != "Claude Code" || overview.LongestTask.Project != "waka-personal" {
		t.Fatalf("unexpected longest task labels: %#v", overview.LongestTask)
	}
	if overview.Days[0].Intensity != 4 || overview.Days[1].Intensity != 0 {
		t.Fatalf("unexpected activity intensities: %#v", overview.Days)
	}
}

func TestLongestActivityStreakIncludesRange(t *testing.T) {
	days := []domain.ActivityDay{
		{Date: "2026-01-01", TotalSeconds: 60},
		{Date: "2026-01-02", TotalSeconds: 60},
		{Date: "2026-01-03", TotalSeconds: 0},
		{Date: "2026-01-04", TotalSeconds: 60},
		{Date: "2026-01-05", TotalSeconds: 0},
	}

	longest := longestActivityStreak(days)
	if longest.days != 2 || longest.start != "2026-01-01" || longest.end != "2026-01-02" {
		t.Fatalf("unexpected longest streak: %#v", longest)
	}
}
