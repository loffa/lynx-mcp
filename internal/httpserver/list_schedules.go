package httpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"lynx-mcp/internal/domain"
	"lynx-mcp/internal/service"
)

type listSchedulesInput struct {
	InstallationID int64    `json:"installation_id"`
	FunctionIDs    []int64  `json:"function_ids,omitempty"`
	Topics         []string `json:"topics,omitempty"`
	Active         *bool    `json:"active,omitempty"`
	Executor       string   `json:"executor,omitempty"`
	Limit          int      `json:"limit,omitempty"`
	Offset         int      `json:"offset,omitempty"`
}

func (s *Server) newListSchedulesTool() (*mcp.Tool, mcp.ToolHandlerFor[listSchedulesInput, domain.ListSchedulesResult]) {
	return &mcp.Tool{
		Name:        "iot.list_schedules",
		Title:       "List IoT Open Lynx schedules",
		Description: "List current schedule definitions for an installation, optionally filtered by functions, exact topics, active status, or executor. With no function or topic filter, include all schedules, even those with no matching function. Returns the full installation including metadata, native schedules, and compact matching function summaries with their exact topic_* metadata keys. Use iot.get_object for full function metadata. Matching functions include all functions sharing the target topic. Results are sorted by schedule ID and paginated after filtering; total counts all matches. These definitions identify possible explanations for value changes, but do not prove historical execution or indirect effects through automations. Timing fields are returned as configured; execution times are not calculated.",
		InputSchema: map[string]any{
			"type":                 "object",
			"required":             []string{domain.InstallationIDProperty},
			"additionalProperties": false,
			"properties": map[string]any{
				domain.InstallationIDProperty: map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Lynx installation ID containing the schedules.",
				},
				"function_ids": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type":    "integer",
						"minimum": 1,
					},
					"description": "Optional function IDs. Match schedules targeting any topic_* metadata value on these functions. Unknown function IDs are errors.",
				},
				"topics": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type":      "string",
						"minLength": 1,
					},
					"description": "Optional exact MQTT topics, without wildcards. Combined with function topics as a union. The installation's own client-ID prefix may be present or absent.",
				},
				"active": map[string]any{
					"type":        "boolean",
					"description": "Optional active status filter. Omit to include both active and inactive schedules.",
				},
				"executor": map[string]any{
					"type":        "string",
					"description": "Optional exact executor name. Omit to include all executors.",
				},
				domain.LimitProperty: map[string]any{
					"type":        "integer",
					"minimum":     1,
					"maximum":     10_000,
					"default":     100,
					"description": "Maximum number of schedules to return, from 1 to 10000. Defaults to 100.",
				},
				domain.OffsetProperty: map[string]any{
					"type":        "integer",
					"minimum":     0,
					"default":     0,
					"description": "Number of matching schedules to skip. Defaults to 0.",
				},
			},
		},
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input listSchedulesInput) (*mcp.CallToolResult, domain.ListSchedulesResult, error) {
		output, err := s.Service.ListSchedules(input.InstallationID, service.ScheduleOptions{
			FunctionIDs: input.FunctionIDs,
			Topics:      input.Topics,
			Active:      input.Active,
			Executor:    input.Executor,
			Limit:       input.Limit,
			Offset:      input.Offset,
		})
		if err != nil {
			return nil, domain.ListSchedulesResult{}, err
		}

		return nil, *output, nil
	}
}
