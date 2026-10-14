package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/IoTOpen/go-lynx"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lynx-mcp/internal/domain"
)

func TestServer_MCPHandler(t *testing.T) {
	t.Parallel()

	write := domain.ControlWrite{TopicKey: "topic_write", Value: 1}
	output := &domain.ControlFunctionResult{
		Installation: &lynx.InstallationRow{ID: 42, Meta: lynx.Meta{}, ProtectedMeta: lynx.Meta{}, Users: []int64{}},
		Function:     &lynx.Function{ID: 7, Meta: lynx.Meta{}, ProtectedMeta: lynx.Meta{}},
		Complete:     true,
		Writes:       []domain.ControlWriteResult{{ControlWrite: write, Topic: "100/actuator/set", Status: "submitted", Validation: domain.ControlValueValidation{Mode: "unconstrained"}}},
	}
	partialOutput := &domain.ControlFunctionResult{
		Installation: output.Installation, Function: output.Function,
		Writes: []domain.ControlWriteResult{{ControlWrite: write, Topic: "100/actuator/set", Status: "failed", Error: "mock error", Validation: domain.ControlValueValidation{Mode: "unconstrained"}}},
	}
	tests := map[string]struct {
		givenOutput   *domain.ControlFunctionResult
		givenErr      error
		wantOutput    *domain.ControlFunctionResult
		wantToolError bool
	}{
		"successful control result reaches the MCP client":     {givenOutput: output, wantOutput: output},
		"service error reaches the MCP client as a tool error": {givenErr: fmt.Errorf("mock error"), wantToolError: true},
		"partial control result remains available as structured and text content": {
			givenOutput: partialOutput, givenErr: fmt.Errorf("mock error"), wantOutput: partialOutput, wantToolError: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			service := &mcpServiceMock{ControlFunctionFunc: func(_ context.Context, installationID, functionID int64, writes []domain.ControlWrite) (*domain.ControlFunctionResult, error) {
				assert.Equal(t, int64(42), installationID)
				assert.Equal(t, int64(7), functionID)
				assert.Equal(t, []domain.ControlWrite{write}, writes)
				return tt.givenOutput, tt.givenErr
			}}
			s := &Server{Service: service}
			httpServer := httptest.NewServer(s.mcpHandler())
			t.Cleanup(httpServer.Close)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			t.Cleanup(cancel)
			client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
			session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
				Endpoint: httpServer.URL, HTTPClient: httpServer.Client(), DisableStandaloneSSE: true,
			}, nil)
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, session.Close()) })

			initialized := session.InitializeResult()
			require.NotNil(t, initialized)
			require.NotNil(t, initialized.ServerInfo)
			assert.Equal(t, "lynx-mcp", initialized.ServerInfo.Name)
			assert.Equal(t, version, initialized.ServerInfo.Version)

			tools, err := session.ListTools(ctx, nil)
			require.NoError(t, err)
			require.NotNil(t, tools)
			var gotNames []string
			for _, tool := range tools.Tools {
				gotNames = append(gotNames, tool.Name)
			}
			assert.ElementsMatch(t, []string{
				"iot.search_objects", "iot.get_object", "iot.list_schedules", "iot.list_automations",
				"iot.control_function", "iot.get_history", "iot.get_current_values",
			}, gotNames)

			gotResult, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "iot.control_function",
				Arguments: controlFunctionInput{InstallationID: 42, FunctionID: 7, Writes: []domain.ControlWrite{write}},
			})
			require.NoError(t, err)
			require.NotNil(t, gotResult)
			assert.Equal(t, tt.wantToolError, gotResult.IsError)
			assert.Len(t, service.ControlFunctionCalls(), 1)
			if tt.wantOutput == nil {
				assert.Nil(t, gotResult.StructuredContent)
			} else {
				encodedOutput, err := json.Marshal(gotResult.StructuredContent)
				require.NoError(t, err)
				var gotOutput domain.ControlFunctionResult
				require.NoError(t, json.Unmarshal(encodedOutput, &gotOutput))
				assert.Equal(t, *tt.wantOutput, gotOutput)
				require.Len(t, gotResult.Content, 1)
				textContent, ok := gotResult.Content[0].(*mcp.TextContent)
				require.True(t, ok)
				assert.JSONEq(t, string(encodedOutput), textContent.Text)
			}
		})
	}
}
