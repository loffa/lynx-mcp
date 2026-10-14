package httpserver

import (
	"fmt"
	"testing"

	"github.com/IoTOpen/go-lynx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lynx-mcp/internal/domain"
	"lynx-mcp/internal/service"
)

func TestServer_NewListAutomationsTool(t *testing.T) {
	t.Parallel()

	include, exclude := true, false
	output := &domain.ListAutomationsResult{
		Installation: &lynx.InstallationRow{ID: 42}, Total: 1, Limit: 10, Offset: 2,
		Automations: []domain.AutomationResult{{Instance: &lynx.EdgeAppConfig{ID: 7, AppID: 9, Version: "v1"}}},
	}
	tests := map[string]struct {
		givenInput  listAutomationsInput
		givenOutput *domain.ListAutomationsResult
		givenErr    error
		wantOptions service.AutomationOptions
		wantOutput  domain.ListAutomationsResult
		wantErr     bool
	}{
		"config enrichment defaults to true and filters are forwarded": {
			givenInput:  listAutomationsInput{InstallationID: 42, InstanceID: 7, AppID: 9, Limit: 10, Offset: 2},
			givenOutput: output, wantOutput: *output,
			wantOptions: service.AutomationOptions{InstanceID: 7, AppID: 9, IncludeConfigOptions: true, Limit: 10, Offset: 2},
		},
		"explicit config enrichment true": {
			givenInput: listAutomationsInput{InstallationID: 42, IncludeConfigOptions: &include}, givenOutput: output, wantOutput: *output,
			wantOptions: service.AutomationOptions{IncludeConfigOptions: true},
		},
		"explicit config enrichment false": {
			givenInput: listAutomationsInput{InstallationID: 42, IncludeConfigOptions: &exclude}, givenOutput: output, wantOutput: *output,
		},
		"service error": {
			givenInput: listAutomationsInput{InstallationID: 42}, givenErr: fmt.Errorf("mock error"), wantErr: true,
			wantOptions: service.AutomationOptions{IncludeConfigOptions: true},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mock := &mcpServiceMock{ListAutomationsFunc: func(id int64, options service.AutomationOptions) (*domain.ListAutomationsResult, error) {
				assert.Equal(t, tt.givenInput.InstallationID, id)
				assert.Equal(t, tt.wantOptions, options)
				return tt.givenOutput, tt.givenErr
			}}
			s := &Server{Service: mock}
			tool, handler := s.newListAutomationsTool()

			gotResult, gotOutput, err := handler(t.Context(), nil, tt.givenInput)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Nil(t, gotResult)
			assert.Equal(t, tt.wantOutput, gotOutput)
			assert.Len(t, mock.ListAutomationsCalls(), 1)
			require.NotNil(t, tool)
			assert.Equal(t, "iot.list_automations", tool.Name)
			require.NotNil(t, tool.Annotations)
			assert.True(t, tool.Annotations.ReadOnlyHint)
			assert.True(t, tool.Annotations.IdempotentHint)
		})
	}
}
