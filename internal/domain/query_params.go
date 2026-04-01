package domain

type DurationQueryParams struct {
	Date           string
	Project        string
	Branches       []string
	SliceBy        string
	Timezone       string
	TimeoutMinutes *int
	WritesOnly     *bool
}

type SummaryQueryParams struct {
	Start          string
	End            string
	Range          string
	Project        string
	Branches       []string
	Timezone       string
	TimeoutMinutes *int
	WritesOnly     *bool
}

type StatsQueryParams struct {
	Range          string
	Timezone       string
	TimeoutMinutes *int
	WritesOnly     *bool
}
