package httpserver

import (
	"fmt"
	"testing"
	"time"

	"github.com/IoTOpen/go-lynx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetHistoryInput_LogOptions(t *testing.T) {
	t.Parallel()

	from := time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC)
	to := time.Date(2026, time.October, 2, 12, 0, 0, 0, time.UTC)
	tests := map[string]struct {
		givenInput  getHistoryInput
		wantOptions lynx.LogOptionsV3
		wantErr     bool
	}{
		"omitted options retain client defaults": {},
		"all options are converted": {
			givenInput:  getHistoryInput{From: "2026-10-01T12:00:00Z", To: "2026-10-02T12:00:00Z", Limit: 25, AggregationMethod: "mean", AggregationInterval: "1h30m"},
			wantOptions: lynx.LogOptionsV3{From: from, To: to, Limit: 25, AggrMethod: "mean", AggrInterval: 90 * time.Minute},
		},
		"only from timestamp":            {givenInput: getHistoryInput{From: "2026-10-01T12:00:00Z"}, wantOptions: lynx.LogOptionsV3{From: from}},
		"only to timestamp":              {givenInput: getHistoryInput{To: "2026-10-02T12:00:00Z"}, wantOptions: lynx.LogOptionsV3{To: to}},
		"aggregation without interval":   {givenInput: getHistoryInput{AggregationMethod: "percentile(95)"}, wantOptions: lynx.LogOptionsV3{AggrMethod: "percentile(95)"}},
		"invalid from timestamp":         {givenInput: getHistoryInput{From: "invalid"}, wantErr: true},
		"invalid to timestamp":           {givenInput: getHistoryInput{From: "2026-10-01T12:00:00Z", To: "invalid"}, wantErr: true},
		"unsupported aggregation method": {givenInput: getHistoryInput{AggregationMethod: "unknown"}, wantErr: true},
		"interval requires a method":     {givenInput: getHistoryInput{AggregationInterval: "1h"}, wantErr: true},
		"invalid interval":               {givenInput: getHistoryInput{AggregationMethod: "mean", AggregationInterval: "invalid"}, wantErr: true},
		"zero interval":                  {givenInput: getHistoryInput{AggregationMethod: "mean", AggregationInterval: "0s"}, wantErr: true},
		"negative interval":              {givenInput: getHistoryInput{AggregationMethod: "mean", AggregationInterval: "-1h"}, wantErr: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			gotOptions, err := tt.givenInput.logOptions()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantOptions, gotOptions)
		})
	}
}

func TestIsAggregationMethod(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		givenMethod string
		wantValid   bool
	}{
		"omitted":                     {wantValid: true},
		"count":                       {givenMethod: "count", wantValid: true},
		"first":                       {givenMethod: "first", wantValid: true},
		"last":                        {givenMethod: "last", wantValid: true},
		"avg":                         {givenMethod: "avg", wantValid: true},
		"mean":                        {givenMethod: "mean", wantValid: true},
		"spread":                      {givenMethod: "spread", wantValid: true},
		"max":                         {givenMethod: "max", wantValid: true},
		"min":                         {givenMethod: "min", wantValid: true},
		"sum":                         {givenMethod: "sum", wantValid: true},
		"stddev":                      {givenMethod: "stddev", wantValid: true},
		"median":                      {givenMethod: "median", wantValid: true},
		"percentile":                  {givenMethod: "percentile(95)", wantValid: true},
		"fractional percentile":       {givenMethod: "percentile(99.5)", wantValid: true},
		"integral":                    {givenMethod: "integral(3600)", wantValid: true},
		"unknown method":              {givenMethod: "unknown"},
		"case mismatch":               {givenMethod: "Mean"},
		"missing argument":            {givenMethod: "percentile()"},
		"missing opening parenthesis": {givenMethod: "percentile95)"},
		"missing closing parenthesis": {givenMethod: "percentile(95"},
		"extra trailing characters":   {givenMethod: "percentile(95)extra"},
		"non numeric argument":        {givenMethod: "integral(invalid)"},
		"NaN argument":                {givenMethod: "percentile(NaN)"},
		"positive infinity argument":  {givenMethod: "integral(+Inf)"},
		"negative infinity argument":  {givenMethod: "percentile(-Inf)"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.wantValid, isAggregationMethod(tt.givenMethod))
		})
	}
}

func TestServer_NewGetHistoryTool(t *testing.T) {
	t.Parallel()

	output := &lynx.V3Log{Total: 1, Count: 1, LastTime: 1_700_000_000, Data: []lynx.LogEntry{{Topic: "temperature", Value: 21.5}}}
	tests := map[string]struct {
		givenInput  getHistoryInput
		givenOutput *lynx.V3Log
		givenErr    error
		wantOptions lynx.LogOptionsV3
		wantOutput  lynx.V3Log
		wantErr     bool
		wantCalls   int
	}{
		"function IDs and parsed options are forwarded": {
			givenInput:  getHistoryInput{InstallationID: 42, FunctionIDs: []int64{7, 9}, From: "2026-10-01T12:00:00Z", To: "2026-10-02T12:00:00Z", Limit: 25, AggregationMethod: "mean", AggregationInterval: "1h"},
			givenOutput: output, wantOutput: *output, wantCalls: 1,
			wantOptions: lynx.LogOptionsV3{From: time.Date(2026, time.October, 1, 12, 0, 0, 0, time.UTC), To: time.Date(2026, time.October, 2, 12, 0, 0, 0, time.UTC), Limit: 25, AggrMethod: "mean", AggrInterval: time.Hour},
		},
		"installation history preserves defaults": {
			givenInput: getHistoryInput{InstallationID: 42}, givenOutput: output, wantOutput: *output, wantCalls: 1,
		},
		"invalid options prevent a service call": {
			givenInput: getHistoryInput{InstallationID: 42, From: "invalid"}, wantErr: true,
		},
		"service error": {
			givenInput: getHistoryInput{InstallationID: 42}, givenErr: fmt.Errorf("mock error"), wantErr: true, wantCalls: 1,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			service := &mcpServiceMock{GetHistoryFunc: func(id int64, functionIDs []int64, options lynx.LogOptionsV3) (*lynx.V3Log, error) {
				assert.Equal(t, tt.givenInput.InstallationID, id)
				assert.Equal(t, tt.givenInput.FunctionIDs, functionIDs)
				assert.Equal(t, tt.wantOptions, options)
				return tt.givenOutput, tt.givenErr
			}}
			s := &Server{Service: service}
			tool, handler := s.newGetHistoryTool()

			gotResult, gotOutput, err := handler(t.Context(), nil, tt.givenInput)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Nil(t, gotResult)
			assert.Equal(t, tt.wantOutput, gotOutput)
			assert.Len(t, service.GetHistoryCalls(), tt.wantCalls)
			require.NotNil(t, tool)
			assert.Equal(t, "iot.get_history", tool.Name)
			require.NotNil(t, tool.Annotations)
			assert.True(t, tool.Annotations.ReadOnlyHint)
			assert.True(t, tool.Annotations.IdempotentHint)
		})
	}
}
