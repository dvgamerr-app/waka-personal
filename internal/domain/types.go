package domain

import (
	"encoding/json"
	"time"
)

type HeartbeatPayload struct {
	ID               string          `json:"id,omitempty"`
	Entity           string          `json:"entity"`
	Type             string          `json:"type,omitempty"`
	Category         string          `json:"category,omitempty"`
	Time             float64         `json:"time"`
	Project          string          `json:"project,omitempty"`
	AlternateProject string          `json:"alternate_project,omitempty"`
	Branch           string          `json:"branch,omitempty"`
	Language         string          `json:"language,omitempty"`
	Dependencies     json.RawMessage `json:"dependencies,omitempty"`
	Lines            *int            `json:"lines,omitempty"`
	LinesInFile      *int            `json:"lines_in_file,omitempty"`
	Lineno           *int            `json:"lineno,omitempty"`
	Cursorpos        *int            `json:"cursorpos,omitempty"`
	IsWrite          bool            `json:"is_write,omitempty"`
	IsUnsavedEntity  bool            `json:"is_unsaved_entity,omitempty"`
	ProjectRootCount *int            `json:"project_root_count,omitempty"`
	ProjectFolder    string          `json:"project_folder,omitempty"`
	AILineChanges    *int            `json:"ai_line_changes,omitempty"`
	HumanLineChanges *int            `json:"human_line_changes,omitempty"`
	Plugin           string          `json:"plugin,omitempty"`
	MachineNameID    string          `json:"machine_name_id,omitempty"`
	UserAgentID      string          `json:"user_agent_id,omitempty"`
	CreatedAt        string          `json:"created_at,omitempty"`
}

type HeartbeatRecord struct {
	ID                  string
	SourceHeartbeatID   string
	DedupeHash          string
	Time                time.Time
	SourceCreatedAt     *time.Time
	Entity              string
	Type                string
	Category            string
	Project             string
	Branch              string
	Language            string
	ProjectRootCount    *int
	ProjectFolder       string
	Lineno              *int
	Cursorpos           *int
	Lines               *int
	IsWrite             bool
	IsUnsavedEntity     bool
	AILineChanges       *int
	HumanLineChanges    *int
	MachineName         string
	SourceMachineNameID string
	Plugin              string
	SourceUserAgentID   string
	Dependencies        []string
	ImportBatchID       *string
	OriginPayload       []byte
}

type ProfileSnapshot struct {
	ExternalUserID string
	Username       string
	DisplayName    string
	FullName       string
	Email          string
	Photo          string
	ProfileURL     string
	Timezone       string
	Plan           string
	TimeoutMinutes *int
	WritesOnly     *bool
	City           []byte
	LastBranch     string
	LastLanguage   string
	LastPlugin     string
	LastProject    string
	ProfileJSON    []byte
}

type ImportBatch struct {
	ID           string
	SourcePath   string
	SourceFormat string
	SourceSHA256 string
	Status       string
	RangeStart   *time.Time
	RangeEnd     *time.Time
	ImportedRows int64
	SkippedRows  int64
	ErrorText    *string
}

type StatusbarCategory struct {
	Name         string  `json:"name"`
	Text         string  `json:"text"`
	TotalSeconds float64 `json:"total_seconds"`
}

type StatusbarTodayData struct {
	GrandTotal struct {
		Text         string  `json:"text"`
		TotalSeconds float64 `json:"total_seconds"`
	} `json:"grand_total"`
	Categories      []StatusbarCategory `json:"categories"`
	HasTeamFeatures bool                `json:"has_team_features"`
	Timezone        string              `json:"timezone"`
}
