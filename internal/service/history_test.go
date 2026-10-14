package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/IoTOpen/go-lynx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_GetHistory(t *testing.T) {
	t.Parallel()

	log := &lynx.V3Log{
		Total:    1,
		LastTime: 1_700_000_000,
		Count:    1,
		Data: []lynx.LogEntry{
			{Topic: "sensor/temperature", Value: 21.5, Timestamp: 1_700_000_000},
		},
	}
	options := lynx.LogOptionsV3{
		Limit:        50,
		Offset:       10,
		From:         time.Unix(1_699_000_000, 0).UTC(),
		To:           time.Unix(1_700_000_000, 0).UTC(),
		Order:        lynx.LogOrderAsc,
		TopicFilter:  []string{"existing/topic"},
		AggrMethod:   "mean",
		AggrInterval: time.Hour,
	}
	filteredOptions := options
	filteredOptions.TopicFilter = []string{"sensor/humidity", "sensor/temperature"}

	tests := map[string]struct {
		givenInstallationID   int64
		givenFunctionIDs      []int64
		givenOptions          lynx.LogOptionsV3
		givenFunctions        lynx.FunctionList
		givenFunctionsErr     error
		givenLogErr           error
		wantLog               *lynx.V3Log
		wantOptions           lynx.LogOptionsV3
		wantErr               bool
		wantGetFunctionsCalls int
		wantLogCalls          int
	}{
		"zero installation ID fails before calling clients": {
			givenInstallationID: 0,
			wantErr:             true,
		},
		"negative installation ID fails before calling clients": {
			givenInstallationID: -1,
			wantErr:             true,
		},
		"negative limit fails before calling clients": {
			givenInstallationID: 42,
			givenOptions:        lynx.LogOptionsV3{Limit: -1},
			wantErr:             true,
		},
		"limit above maximum fails before calling clients": {
			givenInstallationID: 42,
			givenOptions:        lynx.LogOptionsV3{Limit: 10_001},
			wantErr:             true,
		},
		"zero options are passed through for client defaults": {
			givenInstallationID: 42,
			wantLog:             log,
			wantLogCalls:        1,
		},
		"maximum limit is accepted": {
			givenInstallationID: 42,
			givenOptions:        lynx.LogOptionsV3{Limit: 10_000},
			wantLog:             log,
			wantOptions:         lynx.LogOptionsV3{Limit: 10_000},
			wantLogCalls:        1,
		},
		"installation history preserves supplied options": {
			givenInstallationID: 42,
			givenOptions:        options,
			wantLog:             log,
			wantOptions:         options,
			wantLogCalls:        1,
		},
		"function topics replace the filter and preserve other options": {
			givenInstallationID: 42,
			givenFunctionIDs:    []int64{2, 1, 2},
			givenOptions:        options,
			givenFunctions: lynx.FunctionList{
				{ID: 1, Meta: lynx.Meta{
					"topic_read": "sensor/temperature", "topic_write": "sensor/temperature",
				}},
				{ID: 2, Meta: lynx.Meta{"topic_read": "sensor/humidity"}},
				{ID: 3, Meta: lynx.Meta{"topic_read": "unrelated/topic"}},
			},
			wantLog:               log,
			wantOptions:           filteredOptions,
			wantGetFunctionsCalls: 1,
			wantLogCalls:          1,
		},
		"invalid function ID fails before calling clients": {
			givenInstallationID: 42,
			givenFunctionIDs:    []int64{0},
			wantErr:             true,
		},
		"function loading error prevents loading history": {
			givenInstallationID:   42,
			givenFunctionIDs:      []int64{1},
			givenFunctionsErr:     fmt.Errorf("mock error"),
			wantErr:               true,
			wantGetFunctionsCalls: 1,
		},
		"log client error is returned without a result": {
			givenInstallationID: 42,
			givenOptions:        options,
			givenLogErr:         fmt.Errorf("mock error"),
			wantOptions:         options,
			wantErr:             true,
			wantLogCalls:        1,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			v2 := &v2ClientMock{
				GetFunctionsFunc: func(int64, lynx.Filter) (lynx.FunctionList, error) {
					return tt.givenFunctions, tt.givenFunctionsErr
				},
			}
			v3 := &v3ClientMock{
				LogFunc: func(int64, *lynx.LogOptionsV3) (*lynx.V3Log, error) {
					return log, tt.givenLogErr
				},
			}
			s := &Service{V2Client: v2, V3Client: v3}

			gotLog, err := s.GetHistory(tt.givenInstallationID, tt.givenFunctionIDs, tt.givenOptions)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantLog, gotLog)

			functionCalls := v2.GetFunctionsCalls()
			require.Len(t, functionCalls, tt.wantGetFunctionsCalls)
			for _, call := range functionCalls {
				assert.Equal(t, tt.givenInstallationID, call.InstallationID)
				assert.Equal(t, lynx.Filter{}, call.Filter)
			}

			logCalls := v3.LogCalls()
			require.Len(t, logCalls, tt.wantLogCalls)
			for _, call := range logCalls {
				assert.Equal(t, tt.givenInstallationID, call.InstallationID)
				require.NotNil(t, call.Opts)
				assert.Equal(t, tt.wantOptions, *call.Opts)
			}
		})
	}
}
