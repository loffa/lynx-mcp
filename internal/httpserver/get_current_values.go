package httpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"lynx-mcp/internal/domain"
)

type getCurrentValuesInput struct {
	InstallationID int64   `json:"installation_id" jsonschema:"Lynx installation ID containing the functions."`
	FunctionIDs    []int64 `json:"function_ids" jsonschema:"Lynx function IDs whose current topic values to retrieve."`
}

func (s *Server) newGetCurrentValuesTool() (*mcp.Tool, mcp.ToolHandlerFor[getCurrentValuesInput, domain.CurrentValuesResult]) {
	return &mcp.Tool{
		Name:        "iot.get_current_values",
		Title:       "Get current function values",
		Description: "Get the latest known values for every topic configured on the specified Lynx functions. Configured topics with no reported value are returned with null status.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input getCurrentValuesInput) (*mcp.CallToolResult, domain.CurrentValuesResult, error) {
		output, err := s.Service.GetCurrentValues(input.InstallationID, input.FunctionIDs)
		if err != nil {
			return nil, domain.CurrentValuesResult{}, err
		}

		return nil, *output, nil
	}
}
