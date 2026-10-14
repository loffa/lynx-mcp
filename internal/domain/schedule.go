package domain

import "github.com/IoTOpen/go-lynx"

// ListSchedulesResult contains a page of schedule definitions in an installation.
type ListSchedulesResult struct {
	Installation *lynx.InstallationRow `json:"installation"`
	Total        int                   `json:"total"`
	Limit        int                   `json:"limit"`
	Offset       int                   `json:"offset"`
	Schedules    []ScheduleResult      `json:"schedules"`
}

// ScheduleResult pairs a schedule with functions sharing its target topic.
type ScheduleResult struct {
	Schedule         *lynx.Schedule          `json:"schedule"`
	MatchedFunctions []ScheduleFunctionMatch `json:"matched_functions"`
}

// ScheduleFunctionMatch identifies the function metadata keys that match a schedule topic.
type ScheduleFunctionMatch struct {
	FunctionSummary

	TopicKeys []string `json:"topic_keys"`
}
