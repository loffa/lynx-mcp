package httpserver

import (
	"context"
	"fmt"
	"testing"

	"github.com/IoTOpen/go-lynx"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lynx-mcp/internal/domain"
)

func TestServer_NewControlFunctionTool(t *testing.T) {
	t.Parallel()

	input := controlFunctionInput{InstallationID: 42, FunctionID: 7, Writes: []domain.ControlWrite{{TopicKey: "topic_write", Value: 1}}}
	output := &domain.ControlFunctionResult{
		Installation: &lynx.InstallationRow{ID: 42}, Function: &lynx.Function{ID: 7}, Complete: true,
		Writes: []domain.ControlWriteResult{{ControlWrite: input.Writes[0], Topic: "100/actuator/set", Status: "submitted"}},
	}
	partialOutput := &domain.ControlFunctionResult{
		Installation: output.Installation, Function: output.Function,
		Writes: []domain.ControlWriteResult{{ControlWrite: input.Writes[0], Topic: "100/actuator/set", Status: "failed", Error: "mock error"}},
	}
	tests := map[string]struct {
		givenOutput *domain.ControlFunctionResult
		givenErr    error
		givenCancel bool
		wantResult  *mcp.CallToolResult
		wantOutput  domain.ControlFunctionResult
		wantErr     bool
	}{
		"successful submission":                    {givenOutput: output, wantOutput: *output},
		"failure without output returns the error": {givenErr: fmt.Errorf("mock error"), wantErr: true},
		"partial failure retains outcomes and marks the MCP result as an error": {
			givenOutput: partialOutput, givenErr: fmt.Errorf("mock error"),
			wantResult: &mcp.CallToolResult{IsError: true}, wantOutput: *partialOutput,
		},
		"canceled context is forwarded": {givenCancel: true, givenErr: fmt.Errorf("mock error"), wantErr: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tt.givenCancel {
				cancel()
			}
			service := &mcpServiceMock{ControlFunctionFunc: func(gotCtx context.Context, installationID, functionID int64, writes []domain.ControlWrite) (*domain.ControlFunctionResult, error) {
				assert.Same(t, ctx, gotCtx)
				assert.Equal(t, input.InstallationID, installationID)
				assert.Equal(t, input.FunctionID, functionID)
				assert.Equal(t, input.Writes, writes)
				return tt.givenOutput, tt.givenErr
			}}
			s := &Server{Service: service}
			tool, handler := s.newControlFunctionTool()

			gotResult, gotOutput, err := handler(ctx, nil, input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantResult, gotResult)
			assert.Equal(t, tt.wantOutput, gotOutput)
			assert.Len(t, service.ControlFunctionCalls(), 1)
			require.NotNil(t, tool)
			assert.Equal(t, "iot.control_function", tool.Name)
			require.NotNil(t, tool.Annotations)
			assert.False(t, tool.Annotations.ReadOnlyHint)
			assert.False(t, tool.Annotations.IdempotentHint)
			require.NotNil(t, tool.Annotations.DestructiveHint)
			assert.True(t, *tool.Annotations.DestructiveHint)
		})
	}
}
