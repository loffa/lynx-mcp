package service

import (
	"fmt"
	"testing"

	"github.com/IoTOpen/go-lynx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lynx-mcp/internal/domain"
)

func TestService_GetCurrentValues(t *testing.T) {
	t.Parallel()

	first := &lynx.Function{ID: 1, Meta: lynx.Meta{"topic_write": "setpoint", "topic_read": "temperature"}}
	second := &lynx.Function{ID: 2, Meta: lynx.Meta{"topic_read": "temperature"}}
	status := &lynx.LogEntry{Topic: "temperature", Value: 21.5}

	tests := map[string]struct {
		givenInstallationID int64
		givenFunctionIDs    []int64
		givenFunctions      lynx.FunctionList
		givenFunctionsErr   error
		givenStatus         lynx.Status
		givenStatusErr      error
		wantResult          *domain.CurrentValuesResult
		wantErr             bool
		wantFunctionsCalls  int
		wantStatusCalls     int
		wantTopics          []string
	}{
		"zero installation":     {wantErr: true},
		"negative installation": {givenInstallationID: -1, wantErr: true},
		"empty function IDs":    {givenInstallationID: 42, wantErr: true},
		"function client error": {
			givenInstallationID: 42, givenFunctionIDs: []int64{1}, givenFunctionsErr: fmt.Errorf("mock error"),
			wantErr: true, wantFunctionsCalls: 1,
		},
		"status client error": {
			givenInstallationID: 42, givenFunctionIDs: []int64{1}, givenFunctions: lynx.FunctionList{first},
			givenStatusErr: fmt.Errorf("mock error"), wantErr: true,
			wantFunctionsCalls: 1, wantStatusCalls: 1, wantTopics: []string{"setpoint", "temperature"},
		},
		"statuses are matched by topic and missing statuses remain nil": {
			givenInstallationID: 42, givenFunctionIDs: []int64{2, 1, 2},
			givenFunctions: lynx.FunctionList{first, second},
			givenStatus:    lynx.Status{status, {Topic: "unrelated", Value: 99}},
			wantResult: &domain.CurrentValuesResult{Functions: []domain.CurrentValueResult{
				{Function: second, Topics: []domain.CurrentValueTopic{{Key: "read", Topic: "temperature", Status: status}}},
				{Function: first, Topics: []domain.CurrentValueTopic{
					{Key: "read", Topic: "temperature", Status: status},
					{Key: "write", Topic: "setpoint"},
				}},
			}},
			wantFunctionsCalls: 1, wantStatusCalls: 1, wantTopics: []string{"setpoint", "temperature"},
		},
		"empty status does not fail the request": {
			givenInstallationID: 42, givenFunctionIDs: []int64{2}, givenFunctions: lynx.FunctionList{second},
			wantResult: &domain.CurrentValuesResult{Functions: []domain.CurrentValueResult{
				{Function: second, Topics: []domain.CurrentValueTopic{{Key: "read", Topic: "temperature"}}},
			}},
			wantFunctionsCalls: 1, wantStatusCalls: 1, wantTopics: []string{"temperature"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client := &v2ClientMock{
				GetFunctionsFunc: func(int64, lynx.Filter) (lynx.FunctionList, error) {
					return tt.givenFunctions, tt.givenFunctionsErr
				},
				StatusFunc: func(int64, []string) (lynx.Status, error) {
					return tt.givenStatus, tt.givenStatusErr
				},
			}
			s := &Service{V2Client: client}

			gotResult, err := s.GetCurrentValues(tt.givenInstallationID, tt.givenFunctionIDs)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantResult, gotResult)
			require.Len(t, client.GetFunctionsCalls(), tt.wantFunctionsCalls)
			for _, call := range client.GetFunctionsCalls() {
				assert.Equal(t, tt.givenInstallationID, call.InstallationID)
				assert.Equal(t, lynx.Filter{}, call.Filter)
			}
			require.Len(t, client.StatusCalls(), tt.wantStatusCalls)
			for _, call := range client.StatusCalls() {
				assert.Equal(t, tt.givenInstallationID, call.InstallationID)
				assert.Equal(t, tt.wantTopics, call.TopicFilter)
			}
		})
	}
}
