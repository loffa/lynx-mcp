package service

import (
	"fmt"
	"testing"

	"github.com/IoTOpen/go-lynx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lynx-mcp/internal/domain"
)

func TestService_LoadFunctionTopics(t *testing.T) {
	t.Parallel()

	first := &lynx.Function{ID: 1, Meta: lynx.Meta{"topic_read": "sensor/temperature"}}
	second := &lynx.Function{ID: 2, Meta: lynx.Meta{"topic_read": "sensor/humidity"}}

	tests := map[string]struct {
		givenFunctionIDs      []int64
		givenFunctions        lynx.FunctionList
		givenClientErr        error
		wantFunctions         functionTopicList
		wantErr               bool
		wantGetFunctionsCalls int
	}{
		"empty IDs fail before loading functions": {
			wantErr: true,
		},
		"zero ID fails before loading functions": {
			givenFunctionIDs: []int64{0},
			wantErr:          true,
		},
		"negative ID fails before loading functions": {
			givenFunctionIDs: []int64{-1},
			wantErr:          true,
		},
		"client error is returned": {
			givenFunctionIDs:      []int64{1},
			givenClientErr:        fmt.Errorf("mock error"),
			wantErr:               true,
			wantGetFunctionsCalls: 1,
		},
		"empty installation has no requested function": {
			givenFunctionIDs:      []int64{1},
			wantErr:               true,
			wantGetFunctionsCalls: 1,
		},
		"missing requested function fails the whole request": {
			givenFunctionIDs:      []int64{1, 2},
			givenFunctions:        lynx.FunctionList{first},
			wantErr:               true,
			wantGetFunctionsCalls: 1,
		},
		"function without configured topics fails": {
			givenFunctionIDs: []int64{1},
			givenFunctions: lynx.FunctionList{
				{ID: 1, Meta: lynx.Meta{"name": "Temperature", "topic_read": ""}},
			},
			wantErr:               true,
			wantGetFunctionsCalls: 1,
		},
		"only requested functions are selected": {
			givenFunctionIDs: []int64{2},
			givenFunctions:   lynx.FunctionList{first, second},
			wantFunctions: functionTopicList{
				{function: second, topics: map[string]string{"read": "sensor/humidity"}},
			},
			wantGetFunctionsCalls: 1,
		},
		"duplicate IDs are removed while preserving request order": {
			givenFunctionIDs: []int64{2, 1, 2},
			givenFunctions:   lynx.FunctionList{first, second},
			wantFunctions: functionTopicList{
				{function: second, topics: map[string]string{"read": "sensor/humidity"}},
				{function: first, topics: map[string]string{"read": "sensor/temperature"}},
			},
			wantGetFunctionsCalls: 1,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client := &v2ClientMock{
				GetFunctionsFunc: func(int64, lynx.Filter) (lynx.FunctionList, error) {
					return tt.givenFunctions, tt.givenClientErr
				},
			}
			s := &Service{V2Client: client}

			gotFunctions, err := s.loadFunctionTopics(42, tt.givenFunctionIDs)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantFunctions, gotFunctions)

			calls := client.GetFunctionsCalls()
			require.Len(t, calls, tt.wantGetFunctionsCalls)
			for _, call := range calls {
				assert.Equal(t, int64(42), call.InstallationID)
				assert.Equal(t, lynx.Filter{}, call.Filter)
			}
		})
	}
}

func TestUniqueFunctionIDs(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		givenIDs []int64
		wantIDs  []int64
		wantErr  bool
	}{
		"nil IDs": {
			wantErr: true,
		},
		"empty IDs": {
			givenIDs: []int64{},
			wantErr:  true,
		},
		"zero ID": {
			givenIDs: []int64{0},
			wantErr:  true,
		},
		"negative ID after valid IDs": {
			givenIDs: []int64{1, 2, -1},
			wantErr:  true,
		},
		"single ID": {
			givenIDs: []int64{1},
			wantIDs:  []int64{1},
		},
		"distinct IDs retain their order": {
			givenIDs: []int64{3, 1, 2},
			wantIDs:  []int64{3, 1, 2},
		},
		"duplicate IDs retain their first occurrence": {
			givenIDs: []int64{3, 1, 3, 2, 1},
			wantIDs:  []int64{3, 1, 2},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gotIDs, err := uniqueFunctionIDs(tt.givenIDs)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantIDs, gotIDs)
		})
	}
}

func TestAllFunctionTopics(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		givenMeta  lynx.Meta
		wantTopics map[string]string
	}{
		"nil metadata": {
			wantTopics: map[string]string{},
		},
		"unrelated metadata is ignored": {
			givenMeta:  lynx.Meta{"name": "Temperature", "other_topic_read": "ignored"},
			wantTopics: map[string]string{},
		},
		"empty keys and empty topics are ignored": {
			givenMeta:  lynx.Meta{"topic_": "ignored", "topic_read": ""},
			wantTopics: map[string]string{},
		},
		"topic prefixes are removed and channel names are retained": {
			givenMeta: lynx.Meta{
				"name":             "Temperature",
				"topic_read":       "sensor/temperature",
				"topic_write":      "sensor/setpoint",
				"topic_write_mode": "sensor/mode",
			},
			wantTopics: map[string]string{
				"read":       "sensor/temperature",
				"write":      "sensor/setpoint",
				"write_mode": "sensor/mode",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gotTopics := allFunctionTopics(&lynx.Function{Meta: tt.givenMeta})
			assert.Equal(t, tt.wantTopics, gotTopics)
		})
	}
}

func TestFunctionTopicListUniqueTopics(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		givenFunctions functionTopicList
		wantTopics     []string
	}{
		"no functions": {},
		"functions without topics": {
			givenFunctions: functionTopicList{{topics: map[string]string{}}},
		},
		"topics are sorted and deduplicated across keys and functions": {
			givenFunctions: functionTopicList{
				{topics: map[string]string{
					"read": "sensor/temperature", "write": "sensor/temperature",
				}},
				{topics: map[string]string{
					"read": "sensor/humidity", "write": "sensor/temperature",
				}},
			},
			wantTopics: []string{"sensor/humidity", "sensor/temperature"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gotTopics := tt.givenFunctions.uniqueTopics()
			assert.Equal(t, tt.wantTopics, gotTopics)
		})
	}
}

func TestSummarizeFunction(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		givenFunction lynx.Function
		wantSummary   domain.FunctionSummary
	}{
		"name and model type are included": {
			givenFunction: lynx.Function{
				ID:   7,
				Type: "temperature",
				Meta: lynx.Meta{"name": "Room temperature", "topic_read": "sensor/temperature"},
			},
			wantSummary: domain.FunctionSummary{ID: 7, Name: "Room temperature", ModelType: "temperature"},
		},
		"missing name is empty": {
			givenFunction: lynx.Function{ID: 7, Type: "temperature"},
			wantSummary:   domain.FunctionSummary{ID: 7, ModelType: "temperature"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			gotSummary := summarizeFunction(&tt.givenFunction)
			assert.Equal(t, tt.wantSummary, gotSummary)
		})
	}
}
