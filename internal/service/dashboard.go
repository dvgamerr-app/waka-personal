package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"waka-personal/internal/domain"
)

type querySettings struct {
	profile        *domain.ProfileSnapshot
	location       *time.Location
	timezone       string
	timeout        time.Duration
	timeoutMinutes int
	writesOnly     bool
}

type heartbeatInterval struct {
	record  domain.HeartbeatRecord
	start   time.Time
	end     time.Time
	seconds float64
}

type durationSegment struct {
	key            string
	label          string
	machineNameID  string
	start          time.Time
	end            time.Time
	seconds        float64
	aiAdditions    int
	aiDeletions    int
	humanAdditions int
	humanDeletions int
}

type bucketAccumulator struct {
	seconds        float64
	machineNameID  string
	aiAdditions    int
	aiDeletions    int
	humanAdditions int
	humanDeletions int
}

type rangeWindow struct {
	name       string
	humanName  string
	startLocal time.Time
	endLocal   time.Time
}

func (s *QueryService) Durations(ctx context.Context, params domain.DurationQueryParams) ([]map[string]any, time.Time, time.Time, string, error) {
	settings, err := s.resolveQuerySettings(ctx, params.Timezone, params.TimeoutMinutes, params.WritesOnly)
	if err != nil {
		return nil, time.Time{}, time.Time{}, "", err
	}

	dayStartLocal, err := parseDayInLocation(params.Date, settings.location)
	if err != nil {
		return nil, time.Time{}, time.Time{}, "", err
	}
	dayEndLocal := dayStartLocal.Add(24 * time.Hour)

	heartbeats, err := s.store.ListHeartbeatsByRange(ctx, dayStartLocal.UTC(), dayEndLocal.UTC())
	if err != nil {
		return nil, time.Time{}, time.Time{}, "", err
	}

	filtered := filterHeartbeatsForProjectBranches(filterHeartbeats(heartbeats, settings.writesOnly), params.Project, params.Branches)
	intervals := buildHeartbeatIntervals(filtered, settings.timeout, limitForWindow(dayStartLocal, dayEndLocal, settings.location))
	segments := buildDurationSegments(intervals, normalizeSliceBy(params.SliceBy))

	items := make([]map[string]any, 0, len(segments))
	for i := range segments {
		items = append(items, durationSegmentMap(&segments[i]))
	}

	return items, dayStartLocal.UTC(), dayEndLocal.UTC(), settings.timezone, nil
}

func (s *QueryService) Summaries(ctx context.Context, params domain.SummaryQueryParams) ([]map[string]any, error) {
	settings, err := s.resolveQuerySettings(ctx, params.Timezone, params.TimeoutMinutes, params.WritesOnly)
	if err != nil {
		return nil, err
	}

	window, err := parseSummaryWindow(params, time.Now().In(settings.location), settings.location)
	if err != nil {
		return nil, err
	}

	heartbeats, err := s.store.ListHeartbeatsByRange(ctx, window.startLocal.UTC(), window.endLocal.UTC())
	if err != nil {
		return nil, err
	}

	filtered := filterHeartbeatsForProjectBranches(filterHeartbeats(heartbeats, settings.writesOnly), params.Project, params.Branches)
	buckets := bucketHeartbeatsByLocalDate(filtered, settings.location)

	items := make([]map[string]any, 0)
	nowLocal := time.Now().In(settings.location)
	for day := window.startLocal; day.Before(window.endLocal); day = day.Add(24 * time.Hour) {
		items = append(items, buildDailySummaryMap(
			buckets[day.Format("2006-01-02")],
			day,
			nowLocal,
			settings,
			params.Project != "",
		))
	}

	return items, nil
}

func (s *QueryService) Stats(ctx context.Context, params domain.StatsQueryParams) (map[string]any, error) {
	settings, err := s.resolveQuerySettings(ctx, params.Timezone, params.TimeoutMinutes, params.WritesOnly)
	if err != nil {
		return nil, err
	}

	nowLocal := time.Now().In(settings.location)
	window, err := s.parseStatsWindow(ctx, params.Range, nowLocal, settings.location)
	if err != nil {
		return nil, err
	}

	heartbeats, err := s.store.ListHeartbeatsByRange(ctx, window.startLocal.UTC(), window.endLocal.UTC())
	if err != nil {
		return nil, err
	}

	filtered := filterHeartbeats(heartbeats, settings.writesOnly)
	intervals := buildHeartbeatIntervals(filtered, settings.timeout, limitForWindow(window.startLocal, window.endLocal, settings.location))
	totalSeconds, totalSecondsIncludingOther := totalsByLanguage(intervals)
	days := buildDayTotals(intervals, settings.location)
	bestDayDate, bestDaySeconds := bestDay(days)

	categories, _ := collectBucketData(intervals, totalSecondsIncludingOther, categoryBucketValue, false)
	projects, _ := collectBucketData(intervals, totalSecondsIncludingOther, projectBucketValue, false)
	languages, _ := collectBucketData(intervals, totalSecondsIncludingOther, languageBucketValue, false)
	editors, _ := collectBucketData(intervals, totalSecondsIncludingOther, editorBucketValue, false)
	operatingSystems, _ := collectBucketData(intervals, totalSecondsIncludingOther, operatingSystemBucketValue, false)
	dependencies, _ := collectBucketData(intervals, totalSecondsIncludingOther, dependencyBucketValue, false)
	machines, _ := collectMachineBucketData(intervals, totalSecondsIncludingOther)

	dayCount := daySpan(window.startLocal, window.endLocal)
	activeDayCount := activeDays(days)
	dailyAverage := 0.0
	dailyAverageIncludingOther := 0.0
	if dayCount > 0 {
		dailyAverage = totalSeconds / float64(dayCount)
		dailyAverageIncludingOther = totalSecondsIncludingOther / float64(dayCount)
	}

	aiAdditions, aiDeletions, humanAdditions, humanDeletions := sumLineChanges(intervals)
	nowUTC := time.Now().UTC().Format(time.RFC3339)

	return map[string]any{
		"total_seconds":                                         totalSeconds,
		"total_seconds_including_other_language":                totalSecondsIncludingOther,
		"human_readable_total":                                  humanizeDuration(totalSeconds),
		"human_readable_total_including_other_language":         humanizeDuration(totalSecondsIncludingOther),
		"daily_average":                                         dailyAverage,
		"daily_average_including_other_language":                dailyAverageIncludingOther,
		"human_readable_daily_average":                          humanizeDuration(dailyAverage),
		"human_readable_daily_average_including_other_language": humanizeDuration(dailyAverageIncludingOther),
		"ai_additions":                                          aiAdditions,
		"ai_deletions":                                          aiDeletions,
		"human_additions":                                       humanAdditions,
		"human_deletions":                                       humanDeletions,
		"categories":                                            categories,
		"projects":                                              projects,
		"languages":                                             languages,
		"editors":                                               editors,
		"operating_systems":                                     operatingSystems,
		"dependencies":                                          dependencies,
		"machines":                                              machines,
		"best_day": map[string]any{
			"date":          bestDayDate,
			"text":          humanizeDuration(bestDaySeconds),
			"total_seconds": bestDaySeconds,
		},
		"range":                      window.name,
		"human_readable_range":       window.humanName,
		"holidays":                   dayCount - activeDayCount,
		"days_including_holidays":    dayCount,
		"days_minus_holidays":        activeDayCount,
		"status":                     "ok",
		"percent_calculated":         100,
		"is_already_updating":        false,
		"is_coding_activity_visible": true,
		"is_language_usage_visible":  true,
		"is_editor_usage_visible":    true,
		"is_category_usage_visible":  true,
		"is_os_usage_visible":        true,
		"is_stuck":                   false,
		"is_including_today":         window.endLocal.After(startOfDay(nowLocal)),
		"is_up_to_date":              true,
		"start":                      window.startLocal.UTC().Format(time.RFC3339),
		"end":                        window.endLocal.UTC().Format(time.RFC3339),
		"timezone":                   settings.timezone,
		"timeout":                    settings.timeoutMinutes,
		"writes_only":                settings.writesOnly,
		"user_id":                    storeDefault(settings.profile.ExternalUserID, "singleton"),
		"username":                   effectiveUserName(settings.profile),
		"created_at":                 nowUTC,
		"modified_at":                nowUTC,
	}, nil
}

func (s *QueryService) resolveQuerySettings(ctx context.Context, timezone string, timeoutMinutes *int, writesOnly *bool) (querySettings, error) {
	profile, err := s.profile.EffectiveSnapshot(ctx)
	if err != nil {
		return querySettings{}, err
	}

	effectiveTimezone := strings.TrimSpace(timezone)
	if effectiveTimezone == "" {
		effectiveTimezone = storeDefault(profile.Timezone, "UTC")
	}

	loc, err := time.LoadLocation(effectiveTimezone)
	if err != nil {
		return querySettings{}, fmt.Errorf("load timezone %q: %w", effectiveTimezone, err)
	}

	effectiveTimeout := 15
	if profile.TimeoutMinutes != nil && *profile.TimeoutMinutes > 0 {
		effectiveTimeout = *profile.TimeoutMinutes
	}
	if timeoutMinutes != nil && *timeoutMinutes > 0 {
		effectiveTimeout = *timeoutMinutes
	}

	effectiveWritesOnly := false
	if profile.WritesOnly != nil {
		effectiveWritesOnly = *profile.WritesOnly
	}
	if writesOnly != nil {
		effectiveWritesOnly = *writesOnly
	}

	return querySettings{
		profile:        profile,
		location:       loc,
		timezone:       loc.String(),
		timeout:        time.Duration(effectiveTimeout) * time.Minute,
		timeoutMinutes: effectiveTimeout,
		writesOnly:     effectiveWritesOnly,
	}, nil
}

func (s *QueryService) parseStatsWindow(ctx context.Context, value string, now time.Time, loc *time.Location) (rangeWindow, error) {
	rangeName := strings.TrimSpace(strings.ToLower(value))
	if rangeName == "" {
		rangeName = "last_7_days"
	}

	today := startOfDay(now)
	tomorrow := today.Add(24 * time.Hour)

	switch rangeName {
	case "last_7_days":
		return rangeWindow{name: "last_7_days", humanName: "Last 7 Days", startLocal: today.AddDate(0, 0, -6), endLocal: tomorrow}, nil
	case "last_30_days":
		return rangeWindow{name: "last_30_days", humanName: "Last 30 Days", startLocal: today.AddDate(0, 0, -29), endLocal: tomorrow}, nil
	case "last_6_months":
		return rangeWindow{name: "last_6_months", humanName: "Last 6 Months", startLocal: today.AddDate(0, -6, 1), endLocal: tomorrow}, nil
	case "last_year":
		lastYearStart := time.Date(today.Year()-1, time.January, 1, 0, 0, 0, 0, loc)
		lastYearEnd := time.Date(today.Year()-1, time.December, 31, 0, 0, 0, 0, loc).Add(24 * time.Hour)
		return rangeWindow{name: "last_year", humanName: "Last Year", startLocal: lastYearStart, endLocal: lastYearEnd}, nil
	case "all_time":
		start, _, err := s.store.GetHeartbeatBounds(ctx)
		if err != nil {
			return rangeWindow{}, err
		}
		if start == nil {
			return rangeWindow{name: "all_time", humanName: "All Time", startLocal: today, endLocal: tomorrow}, nil
		}
		startLocal := start.In(loc)
		startLocal = time.Date(startLocal.Year(), startLocal.Month(), startLocal.Day(), 0, 0, 0, 0, loc)
		return rangeWindow{name: "all_time", humanName: "All Time", startLocal: startLocal, endLocal: tomorrow}, nil
	default:
		if parsed, err := time.ParseInLocation("2006-01", rangeName, loc); err == nil {
			endLocal := parsed.AddDate(0, 1, 0)
			if parsed.Year() == now.Year() && parsed.Month() == now.Month() {
				endLocal = tomorrow
			}
			return rangeWindow{name: rangeName, humanName: parsed.Format("January 2006"), startLocal: parsed, endLocal: endLocal}, nil
		}
		if parsed, err := time.ParseInLocation("2006", rangeName, loc); err == nil {
			endLocal := parsed.AddDate(1, 0, 0)
			if parsed.Year() == now.Year() {
				endLocal = tomorrow
			}
			return rangeWindow{name: rangeName, humanName: parsed.Format("2006"), startLocal: parsed, endLocal: endLocal}, nil
		}
		return rangeWindow{}, fmt.Errorf("unsupported stats range %q", value)
	}
}

func parseSummaryWindow(params domain.SummaryQueryParams, now time.Time, loc *time.Location) (rangeWindow, error) {
	today := startOfDay(now)
	tomorrow := today.Add(24 * time.Hour)

	switch strings.TrimSpace(strings.ToLower(params.Range)) {
	case "":
	case "today":
		return rangeWindow{name: "Today", humanName: "Today", startLocal: today, endLocal: tomorrow}, nil
	case "yesterday":
		start := today.AddDate(0, 0, -1)
		return rangeWindow{name: "Yesterday", humanName: "Yesterday", startLocal: start, endLocal: today}, nil
	case "last 7 days":
		return rangeWindow{name: "Last 7 Days", humanName: "Last 7 Days", startLocal: today.AddDate(0, 0, -6), endLocal: tomorrow}, nil
	case "last 7 days from yesterday":
		return rangeWindow{name: "Last 7 Days from Yesterday", humanName: "Last 7 Days from Yesterday", startLocal: today.AddDate(0, 0, -7), endLocal: today}, nil
	case "last 14 days":
		return rangeWindow{name: "Last 14 Days", humanName: "Last 14 Days", startLocal: today.AddDate(0, 0, -13), endLocal: tomorrow}, nil
	case "last 30 days":
		return rangeWindow{name: "Last 30 Days", humanName: "Last 30 Days", startLocal: today.AddDate(0, 0, -29), endLocal: tomorrow}, nil
	case "this week":
		start := beginningOfWeek(today)
		return rangeWindow{name: "This Week", humanName: "This Week", startLocal: start, endLocal: tomorrow}, nil
	case "last week":
		end := beginningOfWeek(today)
		start := end.AddDate(0, 0, -7)
		return rangeWindow{name: "Last Week", humanName: "Last Week", startLocal: start, endLocal: end}, nil
	case "this month":
		start := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, loc)
		return rangeWindow{name: "This Month", humanName: "This Month", startLocal: start, endLocal: tomorrow}, nil
	case "last month":
		end := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, loc)
		start := end.AddDate(0, -1, 0)
		return rangeWindow{name: "Last Month", humanName: "Last Month", startLocal: start, endLocal: end}, nil
	case "last year":
		lastYearStart := time.Date(today.Year()-1, time.January, 1, 0, 0, 0, 0, loc)
		lastYearEnd := time.Date(today.Year()-1, time.December, 31, 0, 0, 0, 0, loc).Add(24 * time.Hour)
		return rangeWindow{name: "Last Year", humanName: "Last Year", startLocal: lastYearStart, endLocal: lastYearEnd}, nil
	}

	if strings.TrimSpace(params.Start) == "" || strings.TrimSpace(params.End) == "" {
		return rangeWindow{}, fmt.Errorf("start and end are required when range is empty")
	}

	startLocal, err := parseDayInLocation(params.Start, loc)
	if err != nil {
		return rangeWindow{}, fmt.Errorf("parse start: %w", err)
	}
	endLocal, err := parseDayInLocation(params.End, loc)
	if err != nil {
		return rangeWindow{}, fmt.Errorf("parse end: %w", err)
	}

	return rangeWindow{
		name:       fmt.Sprintf("%s..%s", params.Start, params.End),
		humanName:  fmt.Sprintf("%s to %s", startLocal.Format("Jan 2, 2006"), endLocal.Format("Jan 2, 2006")),
		startLocal: startLocal,
		endLocal:   endLocal.Add(24 * time.Hour),
	}, nil
}

func buildDailySummaryMap(heartbeats []domain.HeartbeatRecord, dayStartLocal, nowLocal time.Time, settings querySettings, includeProjectDetails bool) map[string]any {
	dayEndLocal := dayStartLocal.Add(24 * time.Hour)
	intervals := buildHeartbeatIntervals(heartbeats, settings.timeout, limitForDay(dayStartLocal, nowLocal).UTC())
	totalSecondsIncludingOther := totalIntervalSeconds(intervals)
	aiAdditions, aiDeletions, humanAdditions, humanDeletions := sumLineChanges(intervals)

	categories, _ := collectBucketData(intervals, totalSecondsIncludingOther, categoryBucketValue, false)
	projects, _ := collectBucketData(intervals, totalSecondsIncludingOther, projectBucketValue, true)
	languages, _ := collectBucketData(intervals, totalSecondsIncludingOther, languageBucketValue, false)
	editors, _ := collectBucketData(intervals, totalSecondsIncludingOther, editorBucketValue, false)
	operatingSystems, _ := collectBucketData(intervals, totalSecondsIncludingOther, operatingSystemBucketValue, false)
	dependencies, _ := collectBucketData(intervals, totalSecondsIncludingOther, dependencyBucketValue, false)
	machines, _ := collectMachineBucketData(intervals, totalSecondsIncludingOther)

	summary := map[string]any{
		"grand_total": mergeMaps(timeFieldsMap(totalSecondsIncludingOther), map[string]any{
			"total_seconds":   totalSecondsIncludingOther,
			"ai_additions":    aiAdditions,
			"ai_deletions":    aiDeletions,
			"human_additions": humanAdditions,
			"human_deletions": humanDeletions,
		}),
		"categories":        categories,
		"projects":          projects,
		"languages":         languages,
		"editors":           editors,
		"operating_systems": operatingSystems,
		"dependencies":      dependencies,
		"machines":          machines,
		"range": map[string]any{
			"date":     dayStartLocal.Format("2006-01-02"),
			"start":    dayStartLocal.UTC().Format(time.RFC3339),
			"end":      dayEndLocal.UTC().Format(time.RFC3339),
			"text":     humanReadableDayRangeText(dayStartLocal, nowLocal),
			"timezone": settings.timezone,
		},
	}

	if includeProjectDetails {
		branches, _ := collectBucketData(intervals, totalSecondsIncludingOther, branchBucketValue, false)
		entities, _ := collectBucketData(intervals, totalSecondsIncludingOther, entityBucketValue, true)
		summary["branches"] = branches
		summary["entities"] = entities
	}

	return summary
}

func buildHeartbeatIntervals(heartbeats []domain.HeartbeatRecord, timeout time.Duration, limit time.Time) []heartbeatInterval {
	if len(heartbeats) == 0 {
		return []heartbeatInterval{}
	}

	sort.Slice(heartbeats, func(i, j int) bool {
		return heartbeats[i].Time.Before(heartbeats[j].Time)
	})

	intervals := make([]heartbeatInterval, 0, len(heartbeats))
	for i := range heartbeats {
		record := heartbeats[i]
		end := record.Time.Add(timeout)
		if i+1 < len(heartbeats) && heartbeats[i+1].Time.Before(end) {
			end = heartbeats[i+1].Time
		}
		if end.After(limit) {
			end = limit
		}
		if !end.After(record.Time) {
			continue
		}

		intervals = append(intervals, heartbeatInterval{
			record:  record,
			start:   record.Time,
			end:     end,
			seconds: end.Sub(record.Time).Seconds(),
		})
	}

	return intervals
}

func buildDurationSegments(intervals []heartbeatInterval, sliceBy string) []durationSegment {
	segments := make([]durationSegment, 0, len(intervals))

	for i := range intervals {
		interval := intervals[i]
		key, label, machineNameID := sliceKeyForInterval(interval.record, sliceBy)
		aiAdditions, aiDeletions := splitLineChanges(interval.record.AILineChanges)
		humanAdditions, humanDeletions := splitLineChanges(interval.record.HumanLineChanges)

		segment := durationSegment{
			key:            key,
			label:          label,
			machineNameID:  machineNameID,
			start:          interval.start,
			end:            interval.end,
			seconds:        interval.seconds,
			aiAdditions:    aiAdditions,
			aiDeletions:    aiDeletions,
			humanAdditions: humanAdditions,
			humanDeletions: humanDeletions,
		}

		if len(segments) == 0 {
			segments = append(segments, segment)
			continue
		}

		last := &segments[len(segments)-1]
		if last.key == segment.key && last.label == segment.label && last.end.Equal(segment.start) {
			last.end = segment.end
			last.seconds += segment.seconds
			last.aiAdditions += segment.aiAdditions
			last.aiDeletions += segment.aiDeletions
			last.humanAdditions += segment.humanAdditions
			last.humanDeletions += segment.humanDeletions
			if last.machineNameID == "" {
				last.machineNameID = segment.machineNameID
			}
			continue
		}

		segments = append(segments, segment)
	}

	return segments
}

func durationSegmentMap(segment *durationSegment) map[string]any {
	item := map[string]any{
		segment.key:       segment.label,
		"time":            float64(segment.start.UnixNano()) / float64(time.Second),
		"duration":        segment.seconds,
		"ai_additions":    segment.aiAdditions,
		"ai_deletions":    segment.aiDeletions,
		"human_additions": segment.humanAdditions,
		"human_deletions": segment.humanDeletions,
	}
	if segment.key == "machine" && strings.TrimSpace(segment.machineNameID) != "" {
		item["machine_name_id"] = segment.machineNameID
	}
	return item
}

func bucketHeartbeatsByLocalDate(heartbeats []domain.HeartbeatRecord, loc *time.Location) map[string][]domain.HeartbeatRecord {
	out := make(map[string][]domain.HeartbeatRecord)
	for i := range heartbeats {
		record := heartbeats[i]
		key := record.Time.In(loc).Format("2006-01-02")
		out[key] = append(out[key], record)
	}
	return out
}

func collectBucketData(intervals []heartbeatInterval, totalSeconds float64, labeler func(domain.HeartbeatRecord) (string, bool), includeLineChanges bool) ([]map[string]any, map[string]*bucketAccumulator) {
	accumulators := map[string]*bucketAccumulator{}
	for i := range intervals {
		record := intervals[i].record
		name, ok := labeler(record)
		if !ok {
			continue
		}

		bucket := accumulators[name]
		if bucket == nil {
			bucket = &bucketAccumulator{}
			accumulators[name] = bucket
		}

		bucket.seconds += intervals[i].seconds
		if includeLineChanges {
			aiAdditions, aiDeletions := splitLineChanges(record.AILineChanges)
			humanAdditions, humanDeletions := splitLineChanges(record.HumanLineChanges)
			bucket.aiAdditions += aiAdditions
			bucket.aiDeletions += aiDeletions
			bucket.humanAdditions += humanAdditions
			bucket.humanDeletions += humanDeletions
		}
	}

	keys := sortedBucketKeys(accumulators)
	items := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		acc := accumulators[key]
		item := mergeMaps(timeFieldsMap(acc.seconds), map[string]any{
			"name":          key,
			"total_seconds": acc.seconds,
			"percent":       percentOf(acc.seconds, totalSeconds),
		})
		if includeLineChanges {
			item["ai_additions"] = acc.aiAdditions
			item["ai_deletions"] = acc.aiDeletions
			item["human_additions"] = acc.humanAdditions
			item["human_deletions"] = acc.humanDeletions
		}
		items = append(items, item)
	}

	return items, accumulators
}

func collectMachineBucketData(intervals []heartbeatInterval, totalSeconds float64) ([]map[string]any, map[string]*bucketAccumulator) {
	accumulators := map[string]*bucketAccumulator{}
	for i := range intervals {
		record := intervals[i].record
		name, ok := machineBucketValue(record)
		if !ok {
			continue
		}

		bucket := accumulators[name]
		if bucket == nil {
			bucket = &bucketAccumulator{machineNameID: strings.TrimSpace(record.SourceMachineNameID)}
			accumulators[name] = bucket
		}
		bucket.seconds += intervals[i].seconds
		if bucket.machineNameID == "" {
			bucket.machineNameID = strings.TrimSpace(record.SourceMachineNameID)
		}
	}

	keys := sortedBucketKeys(accumulators)
	items := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		acc := accumulators[key]
		items = append(items, mergeMaps(timeFieldsMap(acc.seconds), map[string]any{
			"name":            key,
			"machine_name_id": acc.machineNameID,
			"total_seconds":   acc.seconds,
			"percent":         percentOf(acc.seconds, totalSeconds),
		}))
	}

	return items, accumulators
}

func totalsByLanguage(intervals []heartbeatInterval) (float64, float64) {
	total := 0.0
	totalIncludingOther := 0.0
	for i := range intervals {
		totalIncludingOther += intervals[i].seconds
		if language, ok := languageBucketValue(intervals[i].record); ok && language != "Other" {
			total += intervals[i].seconds
		}
	}
	return total, totalIncludingOther
}

func totalIntervalSeconds(intervals []heartbeatInterval) float64 {
	total := 0.0
	for i := range intervals {
		total += intervals[i].seconds
	}
	return total
}

func buildDayTotals(intervals []heartbeatInterval, loc *time.Location) map[string]float64 {
	out := map[string]float64{}
	for i := range intervals {
		key := intervals[i].start.In(loc).Format("2006-01-02")
		out[key] += intervals[i].seconds
	}
	return out
}

func bestDay(days map[string]float64) (string, float64) {
	bestDate := ""
	bestTotal := 0.0
	for date, seconds := range days {
		if seconds > bestTotal || (seconds == bestTotal && (bestDate == "" || date < bestDate)) {
			bestDate = date
			bestTotal = seconds
		}
	}
	return bestDate, bestTotal
}

func activeDays(days map[string]float64) int {
	count := 0
	for _, seconds := range days {
		if seconds > 0 {
			count++
		}
	}
	return count
}

func daySpan(start, end time.Time) int {
	if !end.After(start) {
		return 0
	}
	return int(end.Sub(start).Hours() / 24)
}

func sumLineChanges(intervals []heartbeatInterval) (int, int, int, int) {
	aiAdditions := 0
	aiDeletions := 0
	humanAdditions := 0
	humanDeletions := 0

	for i := range intervals {
		aiAdd, aiDel := splitLineChanges(intervals[i].record.AILineChanges)
		humanAdd, humanDel := splitLineChanges(intervals[i].record.HumanLineChanges)
		aiAdditions += aiAdd
		aiDeletions += aiDel
		humanAdditions += humanAdd
		humanDeletions += humanDel
	}

	return aiAdditions, aiDeletions, humanAdditions, humanDeletions
}

func timeFieldsMap(totalSeconds float64) map[string]any {
	hours, minutes, seconds := durationParts(totalSeconds)
	return map[string]any{
		"decimal": decimalDuration(totalSeconds),
		"digital": digitalDuration(hours, minutes),
		"text":    humanizeDuration(totalSeconds),
		"hours":   hours,
		"minutes": minutes,
		"seconds": seconds,
	}
}

func durationParts(totalSeconds float64) (int, int, int) {
	rounded := int(totalSeconds + 0.5)
	if rounded < 0 {
		rounded = 0
	}
	hours := rounded / 3600
	minutes := (rounded % 3600) / 60
	seconds := rounded % 60
	return hours, minutes, seconds
}

func decimalDuration(totalSeconds float64) string {
	return fmt.Sprintf("%.2f", totalSeconds/3600)
}

func digitalDuration(hours, minutes int) string {
	return fmt.Sprintf("%d:%02d", hours, minutes)
}

func splitLineChanges(value *int) (int, int) {
	if value == nil {
		return 0, 0
	}
	if *value >= 0 {
		return *value, 0
	}
	return 0, -*value
}

func filterHeartbeatsForProjectBranches(heartbeats []domain.HeartbeatRecord, project string, branches []string) []domain.HeartbeatRecord {
	if strings.TrimSpace(project) == "" && len(branches) == 0 {
		return heartbeats
	}

	allowedBranches := make(map[string]struct{}, len(branches))
	for _, branch := range branches {
		trimmed := strings.TrimSpace(branch)
		if trimmed != "" {
			allowedBranches[trimmed] = struct{}{}
		}
	}

	out := make([]domain.HeartbeatRecord, 0, len(heartbeats))
	for i := range heartbeats {
		record := heartbeats[i]
		if strings.TrimSpace(project) != "" && record.Project != project {
			continue
		}
		if len(allowedBranches) > 0 {
			if _, ok := allowedBranches[record.Branch]; !ok {
				continue
			}
		}
		out = append(out, record)
	}
	return out
}

func parseDayInLocation(value string, loc *time.Location) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, fmt.Errorf("date is required")
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("date must use YYYY-MM-DD format")
	}
	return parsed, nil
}

func normalizeSliceBy(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", "project":
		return "project"
	case "entity":
		return "entity"
	case "language":
		return "language"
	case "dependencies":
		return "dependencies"
	case "os":
		return "os"
	case "editor":
		return "editor"
	case "category":
		return "category"
	case "machine":
		return "machine"
	default:
		return "project"
	}
}

func sliceKeyForInterval(record domain.HeartbeatRecord, sliceBy string) (string, string, string) {
	switch sliceBy {
	case "entity":
		value, _ := entityBucketValue(record)
		return "entity", value, ""
	case "language":
		value, _ := languageBucketValue(record)
		return "language", value, ""
	case "dependencies":
		value, _ := dependencyBucketValue(record)
		if value == "" {
			value = "Unknown"
		}
		return "dependencies", value, ""
	case "os":
		value, _ := operatingSystemBucketValue(record)
		if value == "" {
			value = "Unknown"
		}
		return "os", value, ""
	case "editor":
		value, _ := editorBucketValue(record)
		if value == "" {
			value = "Unknown"
		}
		return "editor", value, ""
	case "category":
		value, _ := categoryBucketValue(record)
		return "category", value, ""
	case "machine":
		value, _ := machineBucketValue(record)
		return "machine", value, strings.TrimSpace(record.SourceMachineNameID)
	default:
		value, _ := projectBucketValue(record)
		return "project", value, ""
	}
}

func categoryBucketValue(record domain.HeartbeatRecord) (string, bool) {
	return displayCategoryName(record.Category), true
}

func projectBucketValue(record domain.HeartbeatRecord) (string, bool) {
	name := strings.TrimSpace(record.Project)
	if name == "" {
		name = "Unknown Project"
	}
	return name, true
}

func languageBucketValue(record domain.HeartbeatRecord) (string, bool) {
	name := strings.TrimSpace(record.Language)
	if name == "" {
		name = "Other"
	}
	return name, true
}

func entityBucketValue(record domain.HeartbeatRecord) (string, bool) {
	name := strings.TrimSpace(record.Entity)
	if name == "" {
		name = "Unknown Entity"
	}
	return name, true
}

func branchBucketValue(record domain.HeartbeatRecord) (string, bool) {
	name := strings.TrimSpace(record.Branch)
	if name == "" {
		return "", false
	}
	return name, true
}

func dependencyBucketValue(record domain.HeartbeatRecord) (string, bool) {
	if len(record.Dependencies) == 0 {
		return "", false
	}
	name := strings.TrimSpace(record.Dependencies[0])
	if name == "" {
		return "", false
	}
	return name, true
}

func editorBucketValue(record domain.HeartbeatRecord) (string, bool) {
	name := inferEditor(record)
	if name == "" {
		return "", false
	}
	return name, true
}

func operatingSystemBucketValue(record domain.HeartbeatRecord) (string, bool) {
	name := inferOperatingSystem(record)
	if name == "" {
		return "", false
	}
	return name, true
}

func machineBucketValue(record domain.HeartbeatRecord) (string, bool) {
	name := displayMachineName(record)
	if name == "" {
		return "", false
	}
	return name, true
}

func inferEditor(record domain.HeartbeatRecord) string {
	plugin := strings.TrimSpace(strings.ToLower(record.Plugin))
	if plugin == "" {
		return ""
	}

	editor := plugin
	if slash := strings.Index(editor, "/"); slash >= 0 {
		editor = editor[:slash]
	}
	if space := strings.Index(editor, " "); space >= 0 {
		editor = editor[:space]
	}

	switch editor {
	case "code", "vscode":
		return "VS Code"
	case "cursor":
		return "Cursor"
	case "nvim", "neovim":
		return "Neovim"
	case "jetbrains":
		return "JetBrains"
	default:
		if editor == "" {
			return ""
		}
		return strings.ToUpper(editor[:1]) + editor[1:]
	}
}

func inferOperatingSystem(record domain.HeartbeatRecord) string {
	plugin := strings.ToLower(strings.TrimSpace(record.Plugin))
	switch {
	case strings.Contains(plugin, "win32") || strings.Contains(plugin, "windows"):
		return "Windows"
	case strings.Contains(plugin, "darwin") || strings.Contains(plugin, "mac"):
		return "macOS"
	case strings.Contains(plugin, "linux"):
		return "Linux"
	}

	entity := strings.TrimSpace(record.Entity)
	switch {
	case strings.HasPrefix(entity, "http://"), strings.HasPrefix(entity, "https://"):
		return ""
	case len(entity) > 1 && entity[1] == ':':
		return "Windows"
	case strings.HasPrefix(entity, "/Users/"):
		return "macOS"
	case strings.HasPrefix(entity, "/home/"), strings.HasPrefix(entity, "/srv/"), strings.HasPrefix(entity, "/etc/"):
		return "Linux"
	default:
		return ""
	}
}

func displayMachineName(record domain.HeartbeatRecord) string {
	if strings.TrimSpace(record.MachineName) != "" {
		return record.MachineName
	}
	if strings.TrimSpace(record.SourceMachineNameID) != "" {
		return "Machine " + truncateID(record.SourceMachineNameID)
	}
	return ""
}

func truncateID(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= 8 {
		return trimmed
	}
	return trimmed[:8]
}

func percentOf(value, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return (value / total) * 100
}

func sortedBucketKeys(accumulators map[string]*bucketAccumulator) []string {
	keys := make([]string, 0, len(accumulators))
	for key := range accumulators {
		keys = append(keys, key)
	}

	sort.Slice(keys, func(i, j int) bool {
		left := accumulators[keys[i]]
		right := accumulators[keys[j]]
		if left.seconds == right.seconds {
			return keys[i] < keys[j]
		}
		return left.seconds > right.seconds
	})

	return keys
}

func startOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func beginningOfWeek(today time.Time) time.Time {
	offset := (int(today.Weekday()) + 6) % 7
	return today.AddDate(0, 0, -offset)
}

func limitForWindow(startLocal, endLocal time.Time, loc *time.Location) time.Time {
	nowLocal := time.Now().In(loc)
	dayStart := startOfDay(nowLocal)
	if !startLocal.Before(dayStart) && startLocal.Before(dayStart.Add(24*time.Hour)) && nowLocal.Before(endLocal) {
		return nowLocal.UTC()
	}
	if endLocal.After(nowLocal) && startLocal.Before(nowLocal) {
		return nowLocal.UTC()
	}
	return endLocal.UTC()
}

func limitForDay(dayStartLocal, nowLocal time.Time) time.Time {
	dayEndLocal := dayStartLocal.Add(24 * time.Hour)
	if nowLocal.Before(dayStartLocal) {
		return dayStartLocal
	}
	if nowLocal.After(dayEndLocal) {
		return dayEndLocal
	}
	return nowLocal
}

func humanReadableDayRangeText(dayStartLocal, nowLocal time.Time) string {
	today := startOfDay(nowLocal)
	switch {
	case dayStartLocal.Equal(today):
		return "Today"
	case dayStartLocal.Equal(today.AddDate(0, 0, -1)):
		return "Yesterday"
	default:
		return dayStartLocal.Format("Jan 2, 2006")
	}
}

func mergeMaps(base map[string]any, extra map[string]any) map[string]any {
	for key, value := range extra {
		base[key] = value
	}
	return base
}
