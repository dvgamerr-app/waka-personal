package domain

type ResumeQueryParams struct {
	Timezone string
	Months   int
}

type ResumeCategory struct {
	Name    string  `json:"name"`
	Percent float64 `json:"percent"`
}

type ResumeWorkType struct {
	Categories        []ResumeCategory `json:"categories"`
	EditorsCount      int              `json:"editorsCount"`
	LongestStreakDays int              `json:"longestStreakDays"`
}

type ResumeMonthlyLines struct {
	Month        string  `json:"month"`
	AIPercent    float64 `json:"aiLines"`
	HumanPercent float64 `json:"humanLines"`
}

type ResumeModel struct {
	Name  string  `json:"name"`
	Hours float64 `json:"hours"`
}

type ResumeTokens struct {
	InputTokens  int64   `json:"inputTokens"`
	OutputTokens int64   `json:"outputTokens"`
	EstCostUSD   float64 `json:"estCostUSD"`
}

type ResumeAIAssisted struct {
	AILinePercent         float64              `json:"aiLinePercent"`
	AgentSessions         int                  `json:"agentSessions"`
	LongestTaskSeconds    float64              `json:"longestTaskSeconds"`
	LinesPerMillionTokens float64              `json:"linesPerMillionTokens"`
	Monthly               []ResumeMonthlyLines `json:"monthly"`
	Models                []ResumeModel        `json:"models"`
	Tokens                ResumeTokens         `json:"tokens"`
}

type ResumeOverview struct {
	Timezone    string           `json:"timezone"`
	RangeStart  string           `json:"rangeStart"`
	RangeEnd    string           `json:"rangeEnd"`
	WorkType    ResumeWorkType   `json:"workType"`
	AIAssisted  ResumeAIAssisted `json:"aiAssisted"`
	GeneratedAt string           `json:"generatedAt"`
}
