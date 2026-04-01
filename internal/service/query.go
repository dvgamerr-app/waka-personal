package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"waka-personal/internal/domain"
	"waka-personal/internal/store"
)

type QueryService struct {
	store   *store.Store
	profile *ProfileService
}

func NewQueryService(dataStore *store.Store, profile *ProfileService) *QueryService {
	return &QueryService{store: dataStore, profile: profile}
}

func (s *QueryService) HeartbeatsForDate(ctx context.Context, day time.Time) (heartbeats []domain.HeartbeatRecord, start, end time.Time, timezone string, err error) {
	profile, err := s.profile.EffectiveSnapshot(ctx)
	if err != nil {
		return nil, time.Time{}, time.Time{}, "", err
	}

	loc, err := time.LoadLocation(storeDefault(profile.Timezone, "UTC"))
	if err != nil {
		return nil, time.Time{}, time.Time{}, "", fmt.Errorf("load timezone %q: %w", profile.Timezone, err)
	}

	localDay := day.In(loc)
	start = time.Date(localDay.Year(), localDay.Month(), localDay.Day(), 0, 0, 0, 0, loc)
	end = start.Add(24 * time.Hour)

	heartbeats, err = s.store.ListHeartbeatsByRange(ctx, start.UTC(), end.UTC())
	if err != nil {
		return nil, time.Time{}, time.Time{}, "", err
	}
	return heartbeats, start.UTC(), end.UTC(), loc.String(), nil
}

func (s *QueryService) DeleteHeartbeatsForDate(ctx context.Context, day time.Time, ids []string) (int64, error) {
	profile, err := s.profile.EffectiveSnapshot(ctx)
	if err != nil {
		return 0, err
	}

	loc, err := time.LoadLocation(storeDefault(profile.Timezone, "UTC"))
	if err != nil {
		return 0, fmt.Errorf("load timezone %q: %w", profile.Timezone, err)
	}

	localDay := day.In(loc)
	start := time.Date(localDay.Year(), localDay.Month(), localDay.Day(), 0, 0, 0, 0, loc)
	end := start.Add(24 * time.Hour)
	return s.store.DeleteHeartbeats(ctx, start.UTC(), end.UTC(), ids)
}

func (s *QueryService) StatusbarToday(ctx context.Context, now time.Time) (domain.StatusbarTodayData, error) {
	profile, err := s.profile.EffectiveSnapshot(ctx)
	if err != nil {
		return domain.StatusbarTodayData{}, err
	}

	loc, err := time.LoadLocation(storeDefault(profile.Timezone, "UTC"))
	if err != nil {
		return domain.StatusbarTodayData{}, fmt.Errorf("load timezone %q: %w", profile.Timezone, err)
	}

	start := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 0, 0, 0, 0, loc)
	end := start.Add(24 * time.Hour)

	heartbeats, err := s.store.ListHeartbeatsByRange(ctx, start.UTC(), end.UTC())
	if err != nil {
		return domain.StatusbarTodayData{}, err
	}

	timeout := 15
	if profile.TimeoutMinutes != nil && *profile.TimeoutMinutes > 0 {
		timeout = *profile.TimeoutMinutes
	}

	writesOnly := false
	if profile.WritesOnly != nil {
		writesOnly = *profile.WritesOnly
	}

	filtered := filterHeartbeats(heartbeats, writesOnly)
	categories, totalSeconds := summarizeHeartbeats(filtered, time.Duration(timeout)*time.Minute, end.UTC(), now.UTC())

	resp := domain.StatusbarTodayData{
		Categories:      categories,
		HasTeamFeatures: true,
		Timezone:        loc.String(),
	}
	resp.GrandTotal.Text = humanizeDuration(totalSeconds)
	resp.GrandTotal.TotalSeconds = totalSeconds
	return resp, nil
}

func (s *QueryService) FileExperts(ctx context.Context, entity, project string, projectRootCount *int, now time.Time) ([]map[string]any, error) {
	profile, err := s.profile.EffectiveSnapshot(ctx)
	if err != nil {
		return nil, err
	}

	heartbeats, err := s.store.ListHeartbeatsForEntity(ctx, entity, project, projectRootCount)
	if err != nil {
		return nil, err
	}

	timeout := 15
	if profile.TimeoutMinutes != nil && *profile.TimeoutMinutes > 0 {
		timeout = *profile.TimeoutMinutes
	}

	writesOnly := false
	if profile.WritesOnly != nil {
		writesOnly = *profile.WritesOnly
	}

	totalSeconds := totalDurationSeconds(filterHeartbeats(heartbeats, writesOnly), time.Duration(timeout)*time.Minute, now.UTC())
	user := map[string]any{
		"id":              storeDefault(profile.ExternalUserID, "singleton"),
		"username":        storeDefault(profile.Username, "local"),
		"display_name":    storeDefault(profile.DisplayName, storeDefault(profile.FullName, "Local User")),
		"full_name":       storeDefault(profile.FullName, storeDefault(profile.DisplayName, "Local User")),
		"email":           profile.Email,
		"photo":           profile.Photo,
		"profile_url":     profile.ProfileURL,
		"is_current_user": true,
		"name":            effectiveUserName(profile),
		"long_name":       effectiveLongName(profile),
	}

	return []map[string]any{
		{
			"user": user,
			"total": map[string]any{
				"text":          humanizeDuration(totalSeconds),
				"total_seconds": totalSeconds,
			},
		},
	}, nil
}

func filterHeartbeats(heartbeats []domain.HeartbeatRecord, writesOnly bool) []domain.HeartbeatRecord {
	if !writesOnly {
		return heartbeats
	}

	filtered := make([]domain.HeartbeatRecord, 0, len(heartbeats))
	for i := range heartbeats {
		heartbeat := &heartbeats[i]
		if heartbeat.IsWrite {
			filtered = append(filtered, *heartbeat)
		}
	}
	return filtered
}

func summarizeHeartbeats(heartbeats []domain.HeartbeatRecord, timeout time.Duration, windowEnd, now time.Time) (categories []domain.StatusbarCategory, total float64) {
	if len(heartbeats) == 0 {
		return []domain.StatusbarCategory{}, 0
	}

	sort.Slice(heartbeats, func(i, j int) bool {
		return heartbeats[i].Time.Before(heartbeats[j].Time)
	})

	limit := windowEnd
	if now.Before(limit) {
		limit = now
	}

	categoryTotals := map[string]float64{}
	for i := range heartbeats {
		heartbeat := &heartbeats[i]
		nextTime := heartbeat.Time.Add(timeout)
		if i+1 < len(heartbeats) && heartbeats[i+1].Time.Before(nextTime) {
			nextTime = heartbeats[i+1].Time
		}
		if nextTime.After(limit) {
			nextTime = limit
		}
		if !nextTime.After(heartbeat.Time) {
			continue
		}

		seconds := nextTime.Sub(heartbeat.Time).Seconds()
		category := displayCategoryName(heartbeat.Category)
		categoryTotals[category] += seconds
		total += seconds
	}

	categories = make([]domain.StatusbarCategory, 0, len(categoryTotals))
	for name, seconds := range categoryTotals {
		categories = append(categories, domain.StatusbarCategory{
			Name:         name,
			Text:         humanizeDuration(seconds),
			TotalSeconds: seconds,
		})
	}

	sort.Slice(categories, func(i, j int) bool {
		if categories[i].TotalSeconds == categories[j].TotalSeconds {
			return categories[i].Name < categories[j].Name
		}
		return categories[i].TotalSeconds > categories[j].TotalSeconds
	})

	return categories, total
}

func totalDurationSeconds(heartbeats []domain.HeartbeatRecord, timeout time.Duration, now time.Time) float64 {
	if len(heartbeats) == 0 {
		return 0
	}

	_, total := summarizeHeartbeats(heartbeats, timeout, now.Add(timeout), now)
	return total
}

func humanizeDuration(totalSeconds float64) string {
	seconds := int(totalSeconds + 0.5)
	if seconds <= 0 {
		return "0 secs"
	}

	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	remainingSeconds := seconds % 60
	parts := make([]string, 0, 2)
	if hours > 0 {
		if hours == 1 {
			parts = append(parts, "1 hr")
		} else {
			parts = append(parts, fmt.Sprintf("%d hrs", hours))
		}
	}
	if minutes > 0 {
		if minutes == 1 {
			parts = append(parts, "1 min")
		} else {
			parts = append(parts, fmt.Sprintf("%d mins", minutes))
		}
	}
	if len(parts) == 0 && remainingSeconds > 0 {
		if remainingSeconds == 1 {
			return "1 sec"
		}
		return fmt.Sprintf("%d secs", remainingSeconds)
	}
	if len(parts) == 0 {
		return "0 secs"
	}
	return strings.Join(parts, " ")
}

func displayCategoryName(category string) string {
	switch strings.TrimSpace(strings.ToLower(category)) {
	case "ai coding":
		return "AI Coding"
	case "code reviewing":
		return "Code Reviewing"
	case "writing docs":
		return "Writing Docs"
	case "running tests":
		return "Running Tests"
	case "writing tests":
		return "Writing Tests"
	case "manual testing":
		return "Manual Testing"
	default:
		words := strings.Fields(strings.TrimSpace(strings.ToLower(category)))
		for i, word := range words {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
		if len(words) == 0 {
			return "Coding"
		}
		return strings.Join(words, " ")
	}
}

func effectiveUserName(profile *domain.ProfileSnapshot) string {
	for _, candidate := range []string{profile.Username, profile.DisplayName, profile.FullName, "local"} {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}
	return "local"
}

func effectiveLongName(profile *domain.ProfileSnapshot) string {
	for _, candidate := range []string{profile.FullName, profile.DisplayName, profile.Username, "Local User"} {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}
	return "Local User"
}

func storeDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
