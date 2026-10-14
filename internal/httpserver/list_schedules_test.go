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

func TestServer_NewListSchedulesTool(t *testing.T) {
	t.Parallel()

	active, inactive := true, false
	output := &domain.ListSchedulesResult{
		Installation: &lynx.InstallationRow{ID: 42}, Total: 1, Limit: 10, Offset: 2,
		Schedules: []domain.ScheduleResult{{Schedule: &lynx.Schedule{ID: 7, Topic: "temperature"}}},
	}
	tests := map[string]struct {
		givenInput  listSchedulesInput
		givenOutput *domain.ListSchedulesResult
		givenErr    error
		wantOptions service.ScheduleOptions
		wantOutput  domain.ListSchedulesResult
		wantErr     bool
	}{
		"all filters are forwarded": {
			givenInput:  listSchedulesInput{InstallationID: 42, FunctionIDs: []int64{7, 9}, Topics: []string{"temperature"}, Active: &active, Executor: "edge", Limit: 10, Offset: 2},
			givenOutput: output, wantOutput: *output,
			wantOptions: service.ScheduleOptions{FunctionIDs: []int64{7, 9}, Topics: []string{"temperature"}, Active: &active, Executor: "edge", Limit: 10, Offset: 2},
		},
		"inactive filter is preserved": {
			givenInput: listSchedulesInput{InstallationID: 42, Active: &inactive}, givenOutput: output, wantOutput: *output,
			wantOptions: service.ScheduleOptions{Active: &inactive},
		},
		"omitted filters remain unset": {
			givenInput: listSchedulesInput{InstallationID: 42}, givenOutput: output, wantOutput: *output,
		},
		"service error": {
			givenInput: listSchedulesInput{InstallationID: 42}, givenErr: fmt.Errorf("mock error"), wantErr: true,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mock := &mcpServiceMock{ListSchedulesFunc: func(id int64, options service.ScheduleOptions) (*domain.ListSchedulesResult, error) {
				assert.Equal(t, tt.givenInput.InstallationID, id)
				assert.Equal(t, tt.wantOptions, options)
				return tt.givenOutput, tt.givenErr
			}}
			s := &Server{Service: mock}
			tool, handler := s.newListSchedulesTool()

			gotResult, gotOutput, err := handler(t.Context(), nil, tt.givenInput)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Nil(t, gotResult)
			assert.Equal(t, tt.wantOutput, gotOutput)
			assert.Len(t, mock.ListSchedulesCalls(), 1)
			require.NotNil(t, tool)
			assert.Equal(t, "iot.list_schedules", tool.Name)
			require.NotNil(t, tool.Annotations)
			assert.True(t, tool.Annotations.ReadOnlyHint)
			assert.True(t, tool.Annotations.IdempotentHint)
		})
	}
}
