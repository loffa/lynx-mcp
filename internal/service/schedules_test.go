package service

import (
	"fmt"
	"testing"

	"github.com/IoTOpen/go-lynx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lynx-mcp/internal/domain"
)

func TestService_ListSchedules_Validation(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		givenInstallationID int64
		givenOptions        ScheduleOptions
	}{
		"zero installation":     {},
		"negative installation": {givenInstallationID: -1},
		"negative limit":        {givenInstallationID: 42, givenOptions: ScheduleOptions{Limit: -1}},
		"negative offset":       {givenInstallationID: 42, givenOptions: ScheduleOptions{Offset: -1}},
		"invalid function ID":   {givenInstallationID: 42, givenOptions: ScheduleOptions{FunctionIDs: []int64{0}}},
		"empty topic":           {givenInstallationID: 42, givenOptions: ScheduleOptions{Topics: []string{""}}},
		"single level wildcard": {givenInstallationID: 42, givenOptions: ScheduleOptions{Topics: []string{"sensor/+"}}},
		"multi level wildcard":  {givenInstallationID: 42, givenOptions: ScheduleOptions{Topics: []string{"sensor/#"}}},
		"null in topic":         {givenInstallationID: 42, givenOptions: ScheduleOptions{Topics: []string{"sensor/\x00"}}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := &Service{}
			gotResult, err := s.ListSchedules(tt.givenInstallationID, tt.givenOptions)
			require.Error(t, err)
			assert.Nil(t, gotResult)
		})
	}
}

func TestService_ListSchedules(t *testing.T) {
	t.Parallel()

	installation := &lynx.InstallationRow{ID: 42, ClientID: 100}
	functions := lynx.FunctionList{
		{ID: 2, Type: "switch", Meta: lynx.Meta{"name": "Switch", "topic_read": "100/temperature", "topic_write": "temperature"}},
		{ID: 1, Type: "temperature", Meta: lynx.Meta{"name": "Temperature", "topic_read": "temperature"}},
		{ID: 3, Type: "humidity", Meta: lynx.Meta{"topic_read": "humidity"}},
		{ID: 4, Meta: lynx.Meta{"name": "No topics"}},
	}
	schedules := []*lynx.Schedule{
		{ID: 3, Topic: "100/temperature", Active: true, Executor: "local"},
		{ID: 1, Topic: "temperature", Active: false, Executor: "local"},
		{ID: 2, Topic: "humidity", Active: true, Executor: "remote"},
		{ID: 4, Topic: "101/temperature", Active: true, Executor: "local"},
	}
	temperatureMatches := []domain.ScheduleFunctionMatch{
		{FunctionSummary: domain.FunctionSummary{ID: 1, Name: "Temperature", ModelType: "temperature"}, TopicKeys: []string{"topic_read"}},
		{FunctionSummary: domain.FunctionSummary{ID: 2, Name: "Switch", ModelType: "switch"}, TopicKeys: []string{"topic_read", "topic_write"}},
	}
	first := domain.ScheduleResult{Schedule: schedules[1], MatchedFunctions: temperatureMatches}
	second := domain.ScheduleResult{Schedule: schedules[2], MatchedFunctions: []domain.ScheduleFunctionMatch{
		{FunctionSummary: domain.FunctionSummary{ID: 3, ModelType: "humidity"}, TopicKeys: []string{"topic_read"}},
	}}
	third := domain.ScheduleResult{Schedule: schedules[0], MatchedFunctions: temperatureMatches}
	fourth := domain.ScheduleResult{Schedule: schedules[3], MatchedFunctions: []domain.ScheduleFunctionMatch{}}
	active, inactive := true, false

	tests := map[string]struct {
		givenOptions         ScheduleOptions
		givenEmpty           bool
		givenInstallationErr error
		givenFunctionsErr    error
		givenSchedulesErr    error
		wantSchedules        []domain.ScheduleResult
		wantTotal            int
		wantLimit            int
		wantErr              bool
		wantFunctionsCalls   int
		wantSchedulesCalls   int
	}{
		"sorted schedules include sorted function and topic matches": {
			wantSchedules: []domain.ScheduleResult{first, second, third, fourth}, wantTotal: 4, wantLimit: 100,
			wantFunctionsCalls: 1, wantSchedulesCalls: 1,
		},
		"active schedules": {
			givenOptions: ScheduleOptions{Active: &active}, wantSchedules: []domain.ScheduleResult{second, third, fourth}, wantTotal: 3, wantLimit: 100,
			wantFunctionsCalls: 1, wantSchedulesCalls: 1,
		},
		"inactive schedules": {
			givenOptions: ScheduleOptions{Active: &inactive}, wantSchedules: []domain.ScheduleResult{first}, wantTotal: 1, wantLimit: 100,
			wantFunctionsCalls: 1, wantSchedulesCalls: 1,
		},
		"executor is forwarded and checked locally": {
			givenOptions: ScheduleOptions{Executor: "local"}, wantSchedules: []domain.ScheduleResult{first, third, fourth}, wantTotal: 3, wantLimit: 100,
			wantFunctionsCalls: 1, wantSchedulesCalls: 1,
		},
		"topic filter normalizes only the current client prefix": {
			givenOptions: ScheduleOptions{Topics: []string{"100/temperature"}}, wantSchedules: []domain.ScheduleResult{first, third}, wantTotal: 2, wantLimit: 100,
			wantFunctionsCalls: 1, wantSchedulesCalls: 1,
		},
		"function filter deduplicates IDs": {
			givenOptions: ScheduleOptions{FunctionIDs: []int64{2, 2}}, wantSchedules: []domain.ScheduleResult{first, third}, wantTotal: 2, wantLimit: 100,
			wantFunctionsCalls: 1, wantSchedulesCalls: 1,
		},
		"explicit topics and function topics are combined": {
			givenOptions: ScheduleOptions{Topics: []string{"humidity"}, FunctionIDs: []int64{1}}, wantSchedules: []domain.ScheduleResult{first, second, third}, wantTotal: 3, wantLimit: 100,
			wantFunctionsCalls: 1, wantSchedulesCalls: 1,
		},
		"filters are applied before pagination": {
			givenOptions: ScheduleOptions{Active: &active, Limit: 1, Offset: 1}, wantSchedules: []domain.ScheduleResult{third}, wantTotal: 3, wantLimit: 1,
			wantFunctionsCalls: 1, wantSchedulesCalls: 1,
		},
		"offset beyond results": {
			givenOptions: ScheduleOptions{Offset: 99}, wantSchedules: []domain.ScheduleResult{}, wantTotal: 4, wantLimit: 100,
			wantFunctionsCalls: 1, wantSchedulesCalls: 1,
		},
		"function without topics does not broaden the filter": {
			givenOptions: ScheduleOptions{FunctionIDs: []int64{4}}, wantSchedules: []domain.ScheduleResult{}, wantLimit: 100,
			wantFunctionsCalls: 1, wantSchedulesCalls: 1,
		},
		"empty installation": {
			givenEmpty: true, wantSchedules: []domain.ScheduleResult{}, wantLimit: 100, wantFunctionsCalls: 1, wantSchedulesCalls: 1,
		},
		"installation error":            {givenInstallationErr: fmt.Errorf("mock error"), wantErr: true},
		"function error":                {givenFunctionsErr: fmt.Errorf("mock error"), wantErr: true, wantFunctionsCalls: 1},
		"requested function is missing": {givenOptions: ScheduleOptions{FunctionIDs: []int64{999}}, wantErr: true, wantFunctionsCalls: 1},
		"schedule error":                {givenSchedulesErr: fmt.Errorf("mock error"), wantErr: true, wantFunctionsCalls: 1, wantSchedulesCalls: 1},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client := &v2ClientMock{
				GetInstallationRowFunc: func(id int64) (*lynx.InstallationRow, error) {
					assert.Equal(t, int64(42), id)
					return installation, tt.givenInstallationErr
				},
				GetFunctionsFunc: func(id int64, filter lynx.Filter) (lynx.FunctionList, error) {
					assert.Equal(t, int64(42), id)
					assert.Equal(t, lynx.Filter{}, filter)
					if tt.givenEmpty {
						return nil, nil
					}
					return functions, tt.givenFunctionsErr
				},
				GetSchedulesFunc: func(id int64, executor string) ([]*lynx.Schedule, error) {
					assert.Equal(t, int64(42), id)
					assert.Equal(t, tt.givenOptions.Executor, executor)
					if tt.givenEmpty {
						return nil, nil
					}
					return schedules, tt.givenSchedulesErr
				},
			}
			s := &Service{V2Client: client}
			gotResult, err := s.ListSchedules(42, tt.givenOptions)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, gotResult)
			} else {
				require.NoError(t, err)
				require.NotNil(t, gotResult)
				assert.Equal(t, installation, gotResult.Installation)
				assert.Equal(t, tt.wantSchedules, gotResult.Schedules)
				assert.Equal(t, tt.wantTotal, gotResult.Total)
				assert.Equal(t, tt.wantLimit, gotResult.Limit)
				assert.Equal(t, tt.givenOptions.Offset, gotResult.Offset)
			}
			assert.Len(t, client.GetInstallationRowCalls(), 1)
			assert.Len(t, client.GetFunctionsCalls(), tt.wantFunctionsCalls)
			assert.Len(t, client.GetSchedulesCalls(), tt.wantSchedulesCalls)
		})
	}
}

func TestNormalizeScheduleTopic(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		givenTopic    string
		givenClientID int64
		wantTopic     string
	}{
		"relative topic":         {givenTopic: "sensor/read", givenClientID: 100, wantTopic: "sensor/read"},
		"current client prefix":  {givenTopic: "100/sensor/read", givenClientID: 100, wantTopic: "sensor/read"},
		"other client prefix":    {givenTopic: "101/sensor/read", givenClientID: 100, wantTopic: "101/sensor/read"},
		"similar numeric prefix": {givenTopic: "1000/sensor/read", givenClientID: 100, wantTopic: "1000/sensor/read"},
		"missing client ID":      {givenTopic: "100/sensor/read", wantTopic: "100/sensor/read"},
		"negative client ID":     {givenTopic: "100/sensor/read", givenClientID: -1, wantTopic: "100/sensor/read"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.wantTopic, normalizeScheduleTopic(tt.givenTopic, tt.givenClientID))
		})
	}
}
