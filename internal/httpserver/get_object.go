package httpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"lynx-mcp/internal/domain"
)

type getObjectInput struct {
	Type           string `json:"type"`
	ID             int64  `json:"id"`
	InstallationID int64  `json:"installation_id,omitempty"`
	IncludeRelated *bool  `json:"include_related,omitempty"`
}

func (s *Server) newGetObjectTool() (*mcp.Tool, mcp.ToolHandlerFor[getObjectInput, domain.GetObjectResult]) {
	return &mcp.Tool{
		Name:        "iot.get_object",
		Title:       "Get an IoT Open Lynx object",
		Description: "Get a function, device, or installation by its known ID, always including the full installation. Functions and devices require installation_id. By default, functions include the device linked through meta.device_id, and devices include all linked functions as summaries sorted by ID. Enrichment is one level deep. A missing linked device is null; optional relationship failures are reported in warnings. Set include_related to false to skip relationship lookups. Native function and device type fields describe their Lynx model type; the outer type identifies the entity category.",
		InputSchema: map[string]any{
			"type":                 "object",
			"required":             []string{"type", "id"},
			"additionalProperties": false,
			"properties": map[string]any{
				"type": map[string]any{
					"type": "string",
					"enum": []string{domain.ObjectTypeFunction, domain.ObjectTypeDevice, domain.ObjectTypeInstallation},
				},
				"id": map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Lynx ID of the object to retrieve.",
				},
				domain.InstallationIDProperty: map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Required for functions and devices. For an installation, omit or supply the same value as id.",
				},
				"include_related": map[string]any{
					"type":        "boolean",
					"default":     true,
					"description": "Include a function's linked device or a device's linked function summaries. Installation is always included.",
				},
			},
		},
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input getObjectInput) (*mcp.CallToolResult, domain.GetObjectResult, error) {
		includeRelated := input.IncludeRelated == nil || *input.IncludeRelated

		output, err := s.Service.GetObject(input.Type, input.ID, input.InstallationID, includeRelated)
		if err != nil {
			return nil, domain.GetObjectResult{}, err
		}

		return nil, *output, nil
	}
}
