package httpserver

import (
	"fmt"
	"testing"

	"github.com/IoTOpen/go-lynx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lynx-mcp/internal/domain"
)

func TestServer_NewGetCurrentValuesTool(t *testing.T) {
	t.Parallel()

	output := &domain.CurrentValuesResult{Functions: []domain.CurrentValueResult{
		{Function: &lynx.Function{ID: 7}, Topics: []domain.CurrentValueTopic{
			{Key: "read", Topic: "temperature", Status: &lynx.LogEntry{Value: 21.5}},
		}},
	}}
	tests := map[string]struct {
		givenInput  getCurrentValuesInput
		givenOutput *domain.CurrentValuesResult
		givenErr    error
		wantOutput  domain.CurrentValuesResult
		wantErr     bool
	}{
		"IDs and output are forwarded": {
			givenInput:  getCurrentValuesInput{InstallationID: 42, FunctionIDs: []int64{7, 9}},
			givenOutput: output, wantOutput: *output,
		},
		"service error": {
			givenInput: getCurrentValuesInput{InstallationID: 42, FunctionIDs: []int64{7}},
			givenErr:   fmt.Errorf("mock error"), wantErr: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			service := &mcpServiceMock{GetCurrentValuesFunc: func(id int64, functionIDs []int64) (*domain.CurrentValuesResult, error) {
				assert.Equal(t, tt.givenInput.InstallationID, id)
				assert.Equal(t, tt.givenInput.FunctionIDs, functionIDs)
				return tt.givenOutput, tt.givenErr
			}}
			s := &Server{Service: service}
			tool, handler := s.newGetCurrentValuesTool()

			gotResult, gotOutput, err := handler(t.Context(), nil, tt.givenInput)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Nil(t, gotResult)
			assert.Equal(t, tt.wantOutput, gotOutput)
			assert.Len(t, service.GetCurrentValuesCalls(), 1)
			require.NotNil(t, tool)
			assert.Equal(t, "iot.get_current_values", tool.Name)
			require.NotNil(t, tool.Annotations)
			assert.True(t, tool.Annotations.ReadOnlyHint)
			assert.True(t, tool.Annotations.IdempotentHint)
		})
	}
}
