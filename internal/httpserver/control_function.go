package httpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"lynx-mcp/internal/domain"
)

type controlFunctionInput struct {
	InstallationID int64                 `json:"installation_id"`
	FunctionID     int64                 `json:"function_id"`
	Writes         []domain.ControlWrite `json:"writes"`
}

func (s *Server) newControlFunctionTool() (*mcp.Tool, mcp.ToolHandlerFor[controlFunctionInput, domain.ControlFunctionResult]) {
	destructive := true

	return &mcp.Tool{
		Name:        "iot.control_function",
		Title:       "Control an IoT Open Lynx function",
		Description: "Submit numeric values to writable MQTT topics configured on a function. Requires permissions.allow_control. Use iot.get_object to inspect topic_write, topic_write_*, state_*, and min/max metadata. Validates the entire batch before publishing. Channel-specific min_<channel>/max_<channel> bounds and state_<channel>_* sets take precedence over function-level metadata. Any matching declared state is accepted; otherwise applicable inclusive bounds are enforced. States without bounds restrict values to those states; no constraints allow any finite number. Invalid constraints are errors. Adds the installation's client-ID topic prefix when absent and rejects other client IDs and duplicate destinations. Publishes in input order with QoS 1 and retain false, stopping at the first error. Results include full installation and function objects, validation details, and per-write submitted, failed, or not_attempted status. Submission does not confirm device state; delivery may be uncertain on failure. Do not blindly retry partial batches.",
		InputSchema: map[string]any{
			"type":                 "object",
			"required":             []string{domain.InstallationIDProperty, "function_id", "writes"},
			"additionalProperties": false,
			"properties": map[string]any{
				domain.InstallationIDProperty: map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Lynx installation ID containing the function.",
				},
				"function_id": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Lynx function ID whose configured writable topics to control.",
				},
				"writes": map[string]any{
					"type":        "array",
					"minItems":    1,
					"description": "Ordered writes to distinct destination topics. Every write is validated before any publishing begins.",
					"items": map[string]any{
						"type":                 "object",
						"required":             []string{"topic_key", "value"},
						"additionalProperties": false,
						"properties": map[string]any{
							"topic_key": map[string]any{
								"type":        "string",
								"pattern":     "^topic_write(_.+)?$",
								"description": "Exact writable metadata key, for example topic_write or topic_write_red. The topic itself is resolved from function metadata.",
							},
							"value": map[string]any{
								"type":        "number",
								"description": "Finite numeric value, including values from arbitrary state_* metadata keys. Validated against applicable states or inclusive bounds.",
							},
						},
					},
				},
			},
		},
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			DestructiveHint: &destructive,
		},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input controlFunctionInput) (*mcp.CallToolResult, domain.ControlFunctionResult, error) {
		output, err := s.Service.ControlFunction(ctx, input.InstallationID, input.FunctionID, input.Writes)
		if err != nil {
			if output == nil {
				return nil, domain.ControlFunctionResult{}, err
			}
			// Keep per-write outcomes in both structured and text content on partial failure.
			result := &mcp.CallToolResult{IsError: true}

			return result, *output, nil
		}

		return nil, *output, nil
	}
}
