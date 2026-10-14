package httpserver

import (
	"fmt"
	"testing"

	"github.com/IoTOpen/go-lynx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lynx-mcp/internal/domain"
)

func TestServer_NewGetObjectTool(t *testing.T) {
	t.Parallel()

	includeRelated, excludeRelated := true, false
	result := &domain.GetObjectResult{
		Type:         domain.ObjectTypeDevice,
		ID:           7,
		Installation: &lynx.InstallationRow{ID: 42},
		Device:       &lynx.Device{ID: 7, InstallationID: 42, Type: "sensor"},
		Functions:    []domain.FunctionSummary{{ID: 9, Name: "Temperature", ModelType: "temperature"}},
		Warnings:     []domain.ObjectWarning{{Code: "example_warning", Message: "An enrichment warning"}},
	}

	tests := map[string]struct {
		givenInput         getObjectInput
		givenResult        *domain.GetObjectResult
		givenErr           error
		wantIncludeRelated bool
		wantOutput         domain.GetObjectResult
		wantErr            bool
	}{
		"omitted include_related defaults to true": {
			givenInput:         getObjectInput{Type: domain.ObjectTypeDevice, ID: 7, InstallationID: 42},
			givenResult:        result,
			wantIncludeRelated: true,
			wantOutput:         *result,
		},
		"explicit include_related true is forwarded": {
			givenInput:         getObjectInput{Type: domain.ObjectTypeDevice, ID: 7, InstallationID: 42, IncludeRelated: &includeRelated},
			givenResult:        result,
			wantIncludeRelated: true,
			wantOutput:         *result,
		},
		"explicit include_related false is forwarded": {
			givenInput:  getObjectInput{Type: domain.ObjectTypeDevice, ID: 7, InstallationID: 42, IncludeRelated: &excludeRelated},
			givenResult: result,
			wantOutput:  *result,
		},
		"installation without installation_id is forwarded": {
			givenInput:         getObjectInput{Type: domain.ObjectTypeInstallation, ID: 42},
			givenResult:        &domain.GetObjectResult{Type: domain.ObjectTypeInstallation, ID: 42, Installation: result.Installation},
			wantIncludeRelated: true,
			wantOutput:         domain.GetObjectResult{Type: domain.ObjectTypeInstallation, ID: 42, Installation: result.Installation},
		},
		"service error returns an empty output": {
			givenInput:         getObjectInput{Type: domain.ObjectTypeFunction, ID: 9, InstallationID: 42},
			givenErr:           fmt.Errorf("mock error"),
			wantIncludeRelated: true,
			wantErr:            true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			service := &mcpServiceMock{
				GetObjectFunc: func(objectType string, objectID, installationID int64, includeRelated bool) (*domain.GetObjectResult, error) {
					assert.Equal(t, tt.givenInput.Type, objectType)
					assert.Equal(t, tt.givenInput.ID, objectID)
					assert.Equal(t, tt.givenInput.InstallationID, installationID)
					assert.Equal(t, tt.wantIncludeRelated, includeRelated)
					return tt.givenResult, tt.givenErr
				},
			}
			s := &Server{Service: service}
			tool, handler := s.newGetObjectTool()

			gotResult, gotOutput, err := handler(t.Context(), nil, tt.givenInput)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Nil(t, gotResult)
			assert.Equal(t, tt.wantOutput, gotOutput)
			assert.Len(t, service.GetObjectCalls(), 1)

			require.NotNil(t, tool)
			assert.Equal(t, "iot.get_object", tool.Name)
			require.NotNil(t, tool.Annotations)
			assert.True(t, tool.Annotations.ReadOnlyHint)
			assert.True(t, tool.Annotations.IdempotentHint)
		})
	}
}
