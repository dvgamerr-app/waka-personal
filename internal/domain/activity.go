package domain

type ActivityDay struct {
	Date         string  `json:"date"`
	TotalSeconds float64 `json:"total_seconds"`
	Intensity    int     `json:"intensity"`
}

type ActivityPeakDay struct {
	Date         string  `json:"date"`
	TotalSeconds float64 `json:"total_seconds"`
}

type ActivityTask struct {
	StartedAt      string  `json:"started_at"`
	EndedAt        string  `json:"ended_at"`
	TotalSeconds   float64 `json:"total_seconds"`
	HeartbeatCount int     `json:"heartbeat_count"`
	EntityCount    int     `json:"entity_count"`
	AgentName      string  `json:"agent_name,omitempty"`
	Project        string  `json:"project,omitempty"`
}

type ActivityTokenUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	TotalTokens  int64 `json:"total_tokens"`
}

type ActivityOverview struct {
	Timezone                string             `json:"timezone"`
	Year                    int                `json:"year"`
	RangeStart              string             `json:"range_start"`
	RangeEnd                string             `json:"range_end"`
	PeriodDays              int                `json:"period_days"`
	CalendarDays            int                `json:"calendar_days"`
	TimeoutMinutes          int                `json:"timeout_minutes"`
	WritesOnly              bool               `json:"writes_only"`
	TotalSeconds            float64            `json:"total_seconds"`
	ActiveDays              int                `json:"active_days"`
	AverageActiveDaySeconds float64            `json:"average_active_day_seconds"`
	LongestStreakDays       int                `json:"longest_streak_days"`
	LongestStreakStart      string             `json:"longest_streak_start,omitempty"`
	LongestStreakEnd        string             `json:"longest_streak_end,omitempty"`
	PeakDay                 ActivityPeakDay    `json:"peak_day"`
	LongestTask             ActivityTask       `json:"longest_task"`
	TokenUsage              ActivityTokenUsage `json:"token_usage"`
	Days                    []ActivityDay      `json:"days"`
	GeneratedAt             string             `json:"generated_at"`
}
