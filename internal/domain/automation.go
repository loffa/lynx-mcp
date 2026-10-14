package domain

import "github.com/IoTOpen/go-lynx"

// ListAutomationsResult contains a page of configured EdgeApp instances in an installation.
type ListAutomationsResult struct {
	Installation *lynx.InstallationRow `json:"installation"`
	Total        int                   `json:"total"`
	Limit        int                   `json:"limit"`
	Offset       int                   `json:"offset"`
	Automations  []AutomationResult    `json:"automations"`
}

// AutomationResult combines an EdgeApp instance with optional app details and configuration options.
type AutomationResult struct {
	Instance *lynx.EdgeAppConfig `json:"instance"`
	// Keep the native app fields without exposing recursive client types to schema inference.
	App           map[string]any  `json:"app,omitempty"`
	ConfigOptions any             `json:"config_options,omitempty"`
	Warnings      []ObjectWarning `json:"warnings,omitempty"`
}
