package domain

import "github.com/IoTOpen/go-lynx"

// ControlWrite requests a value for one of a function's writable topic keys.
type ControlWrite struct {
	TopicKey string  `json:"topic_key"`
	Value    float64 `json:"value"`
}

// ControlFunctionResult records validation and submission outcomes for requested writes.
type ControlFunctionResult struct {
	Installation *lynx.InstallationRow `json:"installation"`
	Function     *lynx.Function        `json:"function"`
	Complete     bool                  `json:"complete"`
	Writes       []ControlWriteResult  `json:"writes"`
}

// ControlWriteResult describes the destination, validation, and submission status of one write.
type ControlWriteResult struct {
	ControlWrite

	Topic      string                 `json:"topic"`
	Status     string                 `json:"status"`
	Validation ControlValueValidation `json:"validation"`
	Error      string                 `json:"error,omitempty"`
}

// ControlValueValidation identifies the metadata constraints applied to a control value.
type ControlValueValidation struct {
	Mode      string   `json:"mode"`
	StateKeys []string `json:"state_keys,omitempty"`
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
	MinKey    string   `json:"min_key,omitempty"`
	MaxKey    string   `json:"max_key,omitempty"`
}
