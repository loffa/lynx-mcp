package httpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"lynx-mcp/internal/domain"
	"lynx-mcp/internal/service"
)

type listAutomationsInput struct {
	InstallationID       int64 `json:"installation_id"`
	InstanceID           int64 `json:"instance_id,omitempty"`
	AppID                int64 `json:"app_id,omitempty"`
	IncludeConfigOptions *bool `json:"include_config_options,omitempty"`
	Limit                int   `json:"limit,omitempty"`
	Offset               int   `json:"offset,omitempty"`
}

func (s *Server) newListAutomationsTool() (*mcp.Tool, mcp.ToolHandlerFor[listAutomationsInput, domain.ListAutomationsResult]) {
	return &mcp.Tool{
		Name:        "iot.list_automations",
		Title:       "List IoT Open Lynx automations",
		Description: "List configured EdgeApp instances in an installation, optionally restricted to one instance or app. Always returns the full installation including metadata and each instance's saved configuration, version, name, and timestamps. Enriches instances with native EdgeApp details and, by default, the original app.json configuration definition for the configured version. Configuration definitions describe parameters and UI selectors; they are not JSON Schema. Optional enrichment failures preserve instances and add warnings. Results are sorted by instance ID and paginated after filtering; total counts all matches. These are current configurations and do not establish runtime health or historical execution. Function relationships are not inferred from arbitrary configuration values.",
		InputSchema: map[string]any{
			"type":                 "object",
			"required":             []string{domain.InstallationIDProperty},
			"additionalProperties": false,
			"properties": map[string]any{
				domain.InstallationIDProperty: map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Lynx installation ID containing the EdgeApp instances.",
				},
				"instance_id": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Optional known instance ID. Retrieved directly within the installation; unavailable instances are errors. The response still uses the automations array and pagination.",
				},
				"app_id": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Optional EdgeApp ID. Restrict results to instances of this app. Combined with instance_id as an intersection.",
				},
				"include_config_options": map[string]any{
					"type":        "boolean",
					"default":     true,
					"description": "Include the original configuration definition for each instance's app and version. Actual instance configuration is always included.",
				},
				domain.LimitProperty: map[string]any{
					"type":        "integer",
					"minimum":     1,
					"maximum":     10_000,
					"default":     100,
					"description": "Maximum number of instances to return. Defaults to 100.",
				},
				domain.OffsetProperty: map[string]any{
					"type":        "integer",
					"minimum":     0,
					"default":     0,
					"description": "Number of matching instances to skip. Defaults to 0.",
				},
			},
		},
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input listAutomationsInput) (*mcp.CallToolResult, domain.ListAutomationsResult, error) {
		includeConfigOptions := input.IncludeConfigOptions == nil || *input.IncludeConfigOptions

		output, err := s.Service.ListAutomations(input.InstallationID, service.AutomationOptions{
			InstanceID:           input.InstanceID,
			AppID:                input.AppID,
			IncludeConfigOptions: includeConfigOptions,
			Limit:                input.Limit,
			Offset:               input.Offset,
		})
		if err != nil {
			return nil, domain.ListAutomationsResult{}, err
		}

		return nil, *output, nil
	}
}
