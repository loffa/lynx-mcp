package httpserver

import (
	"context"

	lynx "github.com/IoTOpen/go-lynx"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"lynx-mcp/internal/domain"
)

type searchObjectsInput struct {
	InstallationID int64             `json:"installation_id,omitempty"`
	Query          string            `json:"query,omitempty"`
	Types          []string          `json:"types,omitempty"`
	Limit          int               `json:"limit,omitempty"`
	Offset         int               `json:"offset,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

func (i searchObjectsInput) searchOptions() lynx.SearchOptions {
	return lynx.SearchOptions{
		Query:    i.Query,
		Types:    i.Types,
		Limit:    i.Limit,
		Offset:   i.Offset,
		Metadata: i.Metadata,
	}
}

func (s *Server) newSearchObjectsTool() (*mcp.Tool, mcp.ToolHandlerFor[searchObjectsInput, domain.SearchObjectsResult]) {
	return &mcp.Tool{
		Name:        "iot.search_objects",
		Title:       "Search IoT Open Lynx objects",
		Description: "Search accessible Lynx installations, devices, and functions by name and metadata. Supply installation_id to list that installation's functions and devices directly. Results use type for the entity category and model_type for the DeviceX or FunctionX type; model_type is empty for installations.",
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"anyOf": []map[string]any{
				{"required": []string{"query"}},
				{"required": []string{domain.InstallationIDProperty}},
			},
			"properties": map[string]any{
				domain.InstallationIDProperty: map[string]any{
					"type":        "integer",
					"minimum":     1,
					"description": "Optional Lynx installation ID. When supplied, list its functions and devices directly; query must be omitted or '*'.",
				},
				"query": map[string]any{
					"type":        "string",
					"minLength":   1,
					"description": "Required when installation_id is omitted. Use '*' to search all accessible objects.",
				},
				"types": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
						"enum": []string{domain.ObjectTypeInstallation, domain.ObjectTypeDevice, domain.ObjectTypeFunction},
					},
					"description": "Optional entity types to search: installation, device, or function. Installation-scoped searches support only device and function.",
				},
				domain.LimitProperty: map[string]any{
					"type":        "integer",
					"minimum":     1,
					"maximum":     10_000,
					"default":     100,
					"description": "Maximum number of results to return, from 1 to 10000. Defaults to 100.",
				},
				domain.OffsetProperty: map[string]any{
					"type":        "integer",
					"minimum":     0,
					"default":     0,
					"description": "Number of matching results to skip. Defaults to 0.",
				},
				"metadata": map[string]any{
					"type":                 "object",
					"additionalProperties": map[string]any{"type": "string"},
					"description":          "Metadata key-value filters. Values may use the '*' wildcard.",
				},
			},
		},
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input searchObjectsInput) (*mcp.CallToolResult, domain.SearchObjectsResult, error) {
		output, err := s.Service.SearchObjects(input.InstallationID, input.searchOptions())
		if err != nil {
			return nil, domain.SearchObjectsResult{}, err
		}

		return nil, *output, nil
	}
}
