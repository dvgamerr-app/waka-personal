package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"waka-personal/internal/domain"
)

type activityTaskAccumulator struct {
	session        string
	startedAt      time.Time
	endedAt        time.Time
	lastHeartbeat  time.Time
	totalSeconds   float64
	heartbeatCount int
	entities       map[string]struct{}
	agentName      string
	project        string
}

func (s *QueryService) Activity(ctx context.Context, params domain.ActivityQueryParams, now time.Time) (domain.ActivityOverview, error) {
	settings, err := s.resolveQuerySettings(ctx, params.Timezone, nil, nil)
	if err != nil {
		return domain.ActivityOverview{}, fmt.Errorf("resolve activity settings: %w", err)
	}

	nowLocal := now.In(settings.location)
	year := params.Year
	if year == 0 {
		year = nowLocal.Year()
	}
	if year < 1970 || year > nowLocal.Year() {
		return domain.ActivityOverview{}, fmt.Errorf("activity year must be between 1970 and %d", nowLocal.Year())
	}

	windowStartLocal := time.Date(year, time.January, 1, 0, 0, 0, 0, settings.location)
	calendarEndLocal := windowStartLocal.AddDate(1, 0, 0)
	windowEndLocal := calendarEndLocal
	if year == nowLocal.Year() {
		windowEndLocal = startOfDay(nowLocal).AddDate(0, 0, 1)
	}

	heartbeats, err := s.store.ListHeartbeatsByRange(ctx, windowStartLocal.UTC(), windowEndLocal.UTC())
	if err != nil {
		return domain.ActivityOverview{}, fmt.Errorf("list activity heartbeats: %w", err)
	}

	inputTokens, outputTokens, err := s.store.SumAITokensByRange(ctx, windowStartLocal.UTC(), windowEndLocal.UTC())
	if err != nil {
		return domain.ActivityOverview{}, fmt.Errorf("load yearly token usage: %w", err)
	}

	return buildActivityOverview(
		heartbeats,
		inputTokens,
		outputTokens,
		settings,
		year,
		windowStartLocal,
		windowEndLocal,
		daySpan(windowStartLocal, calendarEndLocal),
		nowLocal,
	), nil
}

func buildActivityOverview(
	heartbeats []domain.HeartbeatRecord,
	inputTokens int64,
	outputTokens int64,
	settings querySettings,
	year int,
	windowStartLocal time.Time,
	windowEndLocal time.Time,
	calendarDays int,
	nowLocal time.Time,
) domain.ActivityOverview {
	filtered := filterHeartbeats(heartbeats, settings.writesOnly)
	buckets := bucketHeartbeatsByLocalDate(filtered, settings.location)
	days := make([]domain.ActivityDay, 0, daySpan(windowStartLocal, windowEndLocal))

	var totalSeconds float64
	var peakDay domain.ActivityPeakDay
	activeDays := 0

	for day := windowStartLocal; day.Before(windowEndLocal); day = day.AddDate(0, 0, 1) {
		date := day.Format("2006-01-02")
		intervals := buildHeartbeatIntervals(
			buckets[date],
			settings.timeout,
			limitForDay(day, nowLocal).UTC(),
		)
		seconds := totalIntervalSeconds(intervals)
		days = append(days, domain.ActivityDay{Date: date, TotalSeconds: seconds})
		totalSeconds += seconds

		if seconds > 0 {
			activeDays++
		}
		if seconds > peakDay.TotalSeconds {
			peakDay = domain.ActivityPeakDay{Date: date, TotalSeconds: seconds}
		}
	}

	applyActivityIntensity(days, peakDay.TotalSeconds)
	longestStreak := longestActivityStreak(days)
	averageActiveDaySeconds := 0.0
	if activeDays > 0 {
		averageActiveDaySeconds = totalSeconds / float64(activeDays)
	}
	taskLimit := nowLocal
	if windowEndLocal.Before(taskLimit) {
		taskLimit = windowEndLocal
	}

	return domain.ActivityOverview{
		Timezone:                settings.timezone,
		Year:                    year,
		RangeStart:              windowStartLocal.Format("2006-01-02"),
		RangeEnd:                windowEndLocal.AddDate(0, 0, -1).Format("2006-01-02"),
		PeriodDays:              len(days),
		CalendarDays:            calendarDays,
		TimeoutMinutes:          settings.timeoutMinutes,
		WritesOnly:              settings.writesOnly,
		TotalSeconds:            totalSeconds,
		ActiveDays:              activeDays,
		AverageActiveDaySeconds: averageActiveDaySeconds,
		LongestStreakDays:       longestStreak.days,
		LongestStreakStart:      longestStreak.start,
		LongestStreakEnd:        longestStreak.end,
		PeakDay:                 peakDay,
		LongestTask:             longestAITask(heartbeats, settings.timeout, taskLimit.UTC()),
		TokenUsage: domain.ActivityTokenUsage{
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalTokens:  inputTokens + outputTokens,
		},
		Days:        days,
		GeneratedAt: nowLocal.UTC().Format(time.RFC3339Nano),
	}
}

func applyActivityIntensity(days []domain.ActivityDay, peakSeconds float64) {
	if peakSeconds <= 0 {
		return
	}
	for i := range days {
		if days[i].TotalSeconds <= 0 {
			continue
		}
		ratio := days[i].TotalSeconds / peakSeconds
		switch {
		case ratio <= 0.2:
			days[i].Intensity = 1
		case ratio <= 0.4:
			days[i].Intensity = 2
		case ratio <= 0.7:
			days[i].Intensity = 3
		default:
			days[i].Intensity = 4
		}
	}
}

type activityStreak struct {
	days  int
	start string
	end   string
}

func longestActivityStreak(days []domain.ActivityDay) activityStreak {
	longest := activityStreak{}
	run := 0
	runStart := ""
	for i := range days {
		if days[i].TotalSeconds > 0 {
			if run == 0 {
				runStart = days[i].Date
			}
			run++
			if run > longest.days {
				longest = activityStreak{days: run, start: runStart, end: days[i].Date}
			}
			continue
		}
		run = 0
		runStart = ""
	}
	return longest
}

func longestAITask(heartbeats []domain.HeartbeatRecord, timeout time.Duration, limit time.Time) domain.ActivityTask {
	aiHeartbeats := make([]domain.HeartbeatRecord, 0)
	for i := range heartbeats {
		if strings.TrimSpace(heartbeats[i].AISession) != "" {
			aiHeartbeats = append(aiHeartbeats, heartbeats[i])
		}
	}
	intervals := buildHeartbeatIntervals(aiHeartbeats, timeout, limit)

	var longest domain.ActivityTask
	var current *activityTaskAccumulator
	flush := func() {
		if current == nil || current.totalSeconds <= longest.TotalSeconds {
			return
		}
		longest = domain.ActivityTask{
			StartedAt:      current.startedAt.UTC().Format(time.RFC3339Nano),
			EndedAt:        current.endedAt.UTC().Format(time.RFC3339Nano),
			TotalSeconds:   current.totalSeconds,
			HeartbeatCount: current.heartbeatCount,
			EntityCount:    len(current.entities),
			AgentName:      current.agentName,
			Project:        current.project,
		}
	}

	for i := range intervals {
		interval := &intervals[i]
		session := strings.TrimSpace(interval.record.AISession)
		startsTask := current == nil ||
			current.session != session ||
			interval.start.Sub(current.lastHeartbeat) > timeout
		if startsTask {
			flush()
			current = &activityTaskAccumulator{
				session:       session,
				startedAt:     interval.start,
				lastHeartbeat: interval.start,
				entities:      map[string]struct{}{},
				agentName:     strings.TrimSpace(interval.record.AIAgentName),
				project:       strings.TrimSpace(interval.record.Project),
			}
		}

		current.endedAt = interval.end
		current.lastHeartbeat = interval.start
		current.totalSeconds += interval.seconds
		current.heartbeatCount++
		if entity := strings.TrimSpace(interval.record.Entity); entity != "" {
			current.entities[entity] = struct{}{}
		}
		if current.agentName == "" {
			current.agentName = strings.TrimSpace(interval.record.AIAgentName)
		}
		if current.project == "" {
			current.project = strings.TrimSpace(interval.record.Project)
		}
	}
	flush()
	return longest
}
