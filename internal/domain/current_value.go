package domain

import "github.com/IoTOpen/go-lynx"

// CurrentValueResult associates a function with the latest values for its configured topics.
type CurrentValueResult struct {
	Function *lynx.Function      `json:"function"`
	Topics   []CurrentValueTopic `json:"topics"`
}

// CurrentValuesResult contains the current topic values for the requested functions.
type CurrentValuesResult struct {
	Functions []CurrentValueResult `json:"functions"`
}

// CurrentValueTopic identifies a configured topic and its latest status, if available.
type CurrentValueTopic struct {
	Key    string         `json:"key"`
	Topic  string         `json:"topic"`
	Status *lynx.LogEntry `json:"status"`
}
