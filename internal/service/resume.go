package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"waka-personal/internal/domain"
)

const defaultResumeMonths = 12

func (s *QueryService) Resume(ctx context.Context, params domain.ResumeQueryParams) (domain.ResumeOverview, error) {
	settings, err := s.resolveQuerySettings(ctx, params.Timezone, nil, nil)
	if err != nil {
		return domain.ResumeOverview{}, fmt.Errorf("resolve resume settings: %w", err)
	}

	months := params.Months
	if months <= 0 {
		months = defaultResumeMonths
	}

	nowLocal := time.Now().In(settings.location)
	endLocal := startOfDay(nowLocal).AddDate(0, 0, 1)
	startLocal := endLocal.AddDate(0, -months, 0)

	heartbeats, err := s.store.ListHeartbeatsByRange(ctx, startLocal.UTC(), endLocal.UTC())
	if err != nil {
		return domain.ResumeOverview{}, fmt.Errorf("list resume heartbeats: %w", err)
	}

	filtered := filterHeartbeats(heartbeats, settings.writesOnly)
	intervals := buildHeartbeatIntervals(filtered, settings.timeout, limitForWindow(startLocal, endLocal, settings.location))
	totalSeconds := totalIntervalSeconds(intervals)

	categoryItems, _ := collectBucketData(intervals, totalSeconds, categoryBucketValue, false)
	categories := make([]domain.ResumeCategory, 0, len(categoryItems))
	for _, item := range categoryItems {
		name, _ := item["name"].(string)
		percent, _ := item["percent"].(float64)
		categories = append(categories, domain.ResumeCategory{Name: name, Percent: percent})
	}

	editorItems, _ := collectBucketData(intervals, totalSeconds, editorBucketValue, false)

	aiModelItems, aiModelAccs := collectBucketData(intervals, totalSeconds, aiModelBucketValue, true)
	applyModelPricing(aiModelItems, aiModelAccs, settings.pricingFn)
	applyModelDisplayNames(aiModelItems)
	models := make([]domain.ResumeModel, 0, len(aiModelItems))
	estCostUSD := 0.0
	for _, item := range aiModelItems {
		name, _ := item["name"].(string)
		seconds, _ := item["total_seconds"].(float64)
		if spendCents, ok := item["spend_cents"].(int64); ok {
			estCostUSD += float64(spendCents) / 100
		}
		models = append(models, domain.ResumeModel{Name: name, Hours: seconds / 3600})
	}

	aiAdditions, aiDeletions, humanAdditions, humanDeletions := sumLineChanges(intervals)
	aiInputTokens, aiOutputTokens := sumAITokens(filtered)

	totalAILines := aiAdditions + aiDeletions
	totalHumanLines := humanAdditions + humanDeletions
	aiLinePercent := 0.0
	if totalAILines+totalHumanLines > 0 {
		aiLinePercent = float64(totalAILines) / float64(totalAILines+totalHumanLines) * 100
	}

	totalTokens := aiInputTokens + aiOutputTokens
	linesPerMillionTokens := 0.0
	if totalTokens > 0 {
		linesPerMillionTokens = float64(totalAILines) / (float64(totalTokens) / 1_000_000)
	}

	agentSessions := countDistinctAISessions(filtered)
	longestTask := longestAITask(heartbeats, settings.timeout, endLocal.UTC())
	longestStreakDays := longestStreakFromIntervals(intervals, settings.location, startLocal, endLocal)
	monthly := monthlyLinePercents(intervals, settings.location, startLocal, endLocal)

	return domain.ResumeOverview{
		Timezone:   settings.timezone,
		RangeStart: startLocal.Format("2006-01-02"),
		RangeEnd:   endLocal.AddDate(0, 0, -1).Format("2006-01-02"),
		WorkType: domain.ResumeWorkType{
			Categories:        categories,
			EditorsCount:      len(editorItems),
			LongestStreakDays: longestStreakDays,
		},
		AIAssisted: domain.ResumeAIAssisted{
			AILinePercent:         aiLinePercent,
			AgentSessions:         agentSessions,
			LongestTaskSeconds:    longestTask.TotalSeconds,
			LinesPerMillionTokens: linesPerMillionTokens,
			Monthly:               monthly,
			Models:                models,
			Tokens: domain.ResumeTokens{
				InputTokens:  aiInputTokens,
				OutputTokens: aiOutputTokens,
				EstCostUSD:   estCostUSD,
			},
		},
		GeneratedAt: nowLocal.UTC().Format(time.RFC3339Nano),
	}, nil
}

func countDistinctAISessions(heartbeats []domain.HeartbeatRecord) int {
	sessions := map[string]struct{}{}
	for i := range heartbeats {
		session := strings.TrimSpace(heartbeats[i].AISession)
		if session == "" {
			continue
		}
		sessions[session] = struct{}{}
	}
	return len(sessions)
}

func longestStreakFromIntervals(intervals []heartbeatInterval, loc *time.Location, startLocal, endLocal time.Time) int {
	activeDays := map[string]bool{}
	for i := range intervals {
		activeDays[intervals[i].start.In(loc).Format("2006-01-02")] = true
	}

	longest, run := 0, 0
	for day := startLocal; day.Before(endLocal); day = day.AddDate(0, 0, 1) {
		if activeDays[day.Format("2006-01-02")] {
			run++
			if run > longest {
				longest = run
			}
			continue
		}
		run = 0
	}
	return longest
}

func monthlyLinePercents(intervals []heartbeatInterval, loc *time.Location, startLocal, endLocal time.Time) []domain.ResumeMonthlyLines {
	type monthTotals struct {
		aiLines    int
		humanLines int
	}
	totals := map[string]*monthTotals{}
	order := make([]string, 0)

	monthStart := time.Date(startLocal.Year(), startLocal.Month(), 1, 0, 0, 0, 0, loc)
	for month := monthStart; month.Before(endLocal); month = month.AddDate(0, 1, 0) {
		key := month.Format("2006-01")
		totals[key] = &monthTotals{}
		order = append(order, key)
	}

	for i := range intervals {
		key := intervals[i].start.In(loc).Format("2006-01")
		bucket, ok := totals[key]
		if !ok {
			continue
		}
		aiAdd, aiDel := splitLineChanges(intervals[i].record.AILineChanges)
		humanAdd, humanDel := splitLineChanges(intervals[i].record.HumanLineChanges)
		bucket.aiLines += aiAdd + aiDel
		bucket.humanLines += humanAdd + humanDel
	}

	out := make([]domain.ResumeMonthlyLines, 0, len(order))
	for _, key := range order {
		bucket := totals[key]
		total := bucket.aiLines + bucket.humanLines
		aiPercent, humanPercent := 0.0, 0.0
		if total > 0 {
			aiPercent = float64(bucket.aiLines) / float64(total) * 100
			humanPercent = 100 - aiPercent
		}
		monthLabel, _ := time.Parse("2006-01", key)
		out = append(out, domain.ResumeMonthlyLines{
			Month:        monthLabel.Format("Jan"),
			AIPercent:    aiPercent,
			HumanPercent: humanPercent,
		})
	}
	return out
}
