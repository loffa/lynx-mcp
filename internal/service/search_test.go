package service

import (
	"fmt"
	"testing"

	"github.com/IoTOpen/go-lynx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lynx-mcp/internal/domain"
)

func TestService_SearchObjects_Validation(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		givenInstallationID int64
		givenOptions        lynx.SearchOptions
	}{
		"unknown type":                            {givenOptions: lynx.SearchOptions{Query: "temperature", Types: []string{"unknown"}}},
		"negative limit":                          {givenOptions: lynx.SearchOptions{Query: "temperature", Limit: -1}},
		"excessive limit":                         {givenOptions: lynx.SearchOptions{Query: "temperature", Limit: 10_001}},
		"negative offset":                         {givenOptions: lynx.SearchOptions{Query: "temperature", Offset: -1}},
		"blank global query":                      {givenOptions: lynx.SearchOptions{Query: " \t"}},
		"negative installation":                   {givenInstallationID: -1},
		"scoped search rejects a query":           {givenInstallationID: 42, givenOptions: lynx.SearchOptions{Query: "temperature"}},
		"scoped search rejects installation type": {givenInstallationID: 42, givenOptions: lynx.SearchOptions{Types: []string{"installation"}}},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s := &Service{}
			gotResult, err := s.SearchObjects(tt.givenInstallationID, tt.givenOptions)
			require.Error(t, err)
			assert.Nil(t, gotResult)
		})
	}
}

func TestService_SearchObjects_Global(t *testing.T) {
	t.Parallel()

	firstInstallation := &lynx.InstallationRow{ID: 42}
	secondInstallation := &lynx.InstallationRow{ID: 43}
	entries := []lynx.SearchEntry{
		{Type: "installation", ID: 42, Name: "Building"},
		{Type: "device", ID: 7, InstallationID: 42, Name: "Thermostat"},
		{Type: "device", ID: 8, InstallationID: 42},
		{Type: "function", ID: 7, InstallationID: 42},
		{Type: "function", ID: 8, InstallationID: 42},
		{Type: "function", ID: 7, InstallationID: 43},
	}

	tests := map[string]struct {
		givenEntries          []lynx.SearchEntry
		givenSearchErr        error
		givenInstallationErr  error
		givenDevicesErr       error
		givenFunctionsErr     error
		wantResults           []domain.SearchObjectEntry
		wantErr               bool
		wantInstallationCalls int
		wantDevicesCalls      int
		wantFunctionsCalls    int
	}{
		"results are enriched with caches scoped by installation and entity type": {
			givenEntries: entries,
			wantResults: []domain.SearchObjectEntry{
				{SearchEntry: entries[0], Installation: firstInstallation},
				{SearchEntry: entries[1], ModelType: "thermostat", Installation: firstInstallation},
				{SearchEntry: entries[2], ModelType: "sensor", Installation: firstInstallation},
				{SearchEntry: entries[3], ModelType: "temperature", Installation: firstInstallation},
				{SearchEntry: entries[4], ModelType: "humidity", Installation: firstInstallation},
				{SearchEntry: entries[5], ModelType: "switch", Installation: secondInstallation},
			},
			wantInstallationCalls: 2, wantDevicesCalls: 1, wantFunctionsCalls: 2,
		},
		"empty results remain an empty slice": {wantResults: []domain.SearchObjectEntry{}},
		"search client error":                 {givenSearchErr: fmt.Errorf("mock error"), wantErr: true},
		"unsupported result type":             {givenEntries: []lynx.SearchEntry{{Type: "unknown", ID: 7}}, wantErr: true},
		"installation client error": {
			givenEntries: entries[:1], givenInstallationErr: fmt.Errorf("mock error"), wantErr: true, wantInstallationCalls: 1,
		},
		"device client error": {
			givenEntries: entries[1:2], givenDevicesErr: fmt.Errorf("mock error"), wantErr: true,
			wantInstallationCalls: 1, wantDevicesCalls: 1,
		},
		"function client error": {
			givenEntries: entries[3:4], givenFunctionsErr: fmt.Errorf("mock error"), wantErr: true,
			wantInstallationCalls: 1, wantFunctionsCalls: 1,
		},
		"missing device model": {
			givenEntries: []lynx.SearchEntry{{Type: "device", ID: 999, InstallationID: 42}}, wantErr: true,
			wantInstallationCalls: 1, wantDevicesCalls: 1,
		},
		"missing function model": {
			givenEntries: []lynx.SearchEntry{{Type: "function", ID: 999, InstallationID: 42}}, wantErr: true,
			wantInstallationCalls: 1, wantFunctionsCalls: 1,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			searchResult := &lynx.SearchResult{Total: 50, Limit: 100, Offset: 5, Results: tt.givenEntries}
			v3 := &v3ClientMock{SearchFunc: func(opts *lynx.SearchOptions) (*lynx.SearchResult, error) {
				assert.Equal(t, &lynx.SearchOptions{Query: "temperature", Types: []string{"device", "function", "installation"}, Limit: 100, Offset: 5, Metadata: lynx.Meta{"floor": "2"}}, opts)
				return searchResult, tt.givenSearchErr
			}}
			v2 := &v2ClientMock{
				GetInstallationRowFunc: func(id int64) (*lynx.InstallationRow, error) {
					assert.Contains(t, []int64{42, 43}, id)
					if id == 43 {
						return secondInstallation, tt.givenInstallationErr
					}
					return firstInstallation, tt.givenInstallationErr
				},
				GetDevicesFunc: func(id int64, filter lynx.Filter) (lynx.DeviceList, error) {
					assert.Equal(t, int64(42), id)
					assert.Equal(t, lynx.Filter{}, filter)
					return lynx.DeviceList{{ID: 7, Type: "thermostat"}, {ID: 8, Type: "sensor"}}, tt.givenDevicesErr
				},
				GetFunctionsFunc: func(id int64, filter lynx.Filter) (lynx.FunctionList, error) {
					assert.Contains(t, []int64{42, 43}, id)
					assert.Equal(t, lynx.Filter{}, filter)
					if id == 43 {
						return lynx.FunctionList{{ID: 7, Type: "switch"}}, tt.givenFunctionsErr
					}
					return lynx.FunctionList{{ID: 7, Type: "temperature"}, {ID: 8, Type: "humidity"}}, tt.givenFunctionsErr
				},
			}
			s := &Service{V2Client: v2, V3Client: v3}

			gotResult, err := s.SearchObjects(0, lynx.SearchOptions{
				Query: " \ttemperature ", Types: []string{"device", "function", "installation"}, Offset: 5, Metadata: lynx.Meta{"floor": "2"},
			})
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, gotResult)
			} else {
				require.NoError(t, err)
				require.NotNil(t, gotResult)
				assert.Equal(t, *searchResult, gotResult.SearchResult)
				assert.Equal(t, tt.wantResults, gotResult.Results)
			}
			assert.Len(t, v3.SearchCalls(), 1)
			assert.Len(t, v2.GetInstallationRowCalls(), tt.wantInstallationCalls)
			assert.Len(t, v2.GetDevicesCalls(), tt.wantDevicesCalls)
			assert.Len(t, v2.GetFunctionsCalls(), tt.wantFunctionsCalls)
		})
	}
}

func TestService_SearchObjects_Installation(t *testing.T) {
	t.Parallel()

	installation := &lynx.InstallationRow{ID: 42}
	devices := lynx.DeviceList{
		{ID: 9, InstallationID: 42, Type: "sensor", Created: 100, Updated: 200, Meta: lynx.Meta{"name": "Sensor"}},
		{ID: 2, InstallationID: 42, Type: "thermostat"},
	}
	functions := lynx.FunctionList{
		{ID: 7, InstallationID: 42, Type: "temperature", Created: 300, Updated: 400, Meta: lynx.Meta{"name": "Temperature"}},
		{ID: 1, InstallationID: 42, Type: "humidity"},
	}
	deviceEntries := []domain.SearchObjectEntry{
		{SearchEntry: lynx.SearchEntry{Type: "device", ID: 2, InstallationID: 42}, ModelType: "thermostat", Installation: installation},
		{SearchEntry: lynx.SearchEntry{Type: "device", ID: 9, InstallationID: 42, Created: 100, Updated: 200, Meta: lynx.Meta{"name": "Sensor"}}, ModelType: "sensor", Installation: installation},
	}
	functionEntries := []domain.SearchObjectEntry{
		{SearchEntry: lynx.SearchEntry{Type: "function", ID: 1, InstallationID: 42}, ModelType: "humidity", Installation: installation},
		{SearchEntry: lynx.SearchEntry{Type: "function", ID: 7, InstallationID: 42, Created: 300, Updated: 400, Meta: lynx.Meta{"name": "Temperature"}}, ModelType: "temperature", Installation: installation},
	}

	tests := map[string]struct {
		givenOptions         lynx.SearchOptions
		givenEmpty           bool
		givenInstallationErr error
		givenDevicesErr      error
		givenFunctionsErr    error
		wantEntries          []domain.SearchObjectEntry
		wantTotal            int64
		wantLimit            int64
		wantErr              bool
		wantDevicesCalls     int
		wantFunctionsCalls   int
	}{
		"all types are sorted before pagination": {
			givenOptions: lynx.SearchOptions{Query: " * ", Limit: 2, Offset: 1, Metadata: lynx.Meta{"floor": "2"}},
			wantEntries:  []domain.SearchObjectEntry{deviceEntries[1], functionEntries[0]}, wantTotal: 4, wantLimit: 2,
			wantDevicesCalls: 1, wantFunctionsCalls: 1,
		},
		"only devices are loaded":   {givenOptions: lynx.SearchOptions{Types: []string{"device"}}, wantEntries: deviceEntries, wantTotal: 2, wantLimit: 100, wantDevicesCalls: 1},
		"only functions are loaded": {givenOptions: lynx.SearchOptions{Types: []string{"function"}}, wantEntries: functionEntries, wantTotal: 2, wantLimit: 100, wantFunctionsCalls: 1},
		"offset beyond results":     {givenOptions: lynx.SearchOptions{Offset: 99}, wantEntries: []domain.SearchObjectEntry{}, wantTotal: 4, wantLimit: 100, wantDevicesCalls: 1, wantFunctionsCalls: 1},
		"empty installation":        {givenEmpty: true, wantEntries: []domain.SearchObjectEntry{}, wantLimit: 100, wantDevicesCalls: 1, wantFunctionsCalls: 1},
		"installation error":        {givenInstallationErr: fmt.Errorf("mock error"), wantErr: true},
		"device error":              {givenDevicesErr: fmt.Errorf("mock error"), wantErr: true, wantDevicesCalls: 1},
		"function error":            {givenFunctionsErr: fmt.Errorf("mock error"), wantErr: true, wantDevicesCalls: 1, wantFunctionsCalls: 1},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client := &v2ClientMock{
				GetInstallationRowFunc: func(id int64) (*lynx.InstallationRow, error) {
					assert.Equal(t, int64(42), id)
					return installation, tt.givenInstallationErr
				},
				GetDevicesFunc: func(id int64, filter lynx.Filter) (lynx.DeviceList, error) {
					assert.Equal(t, int64(42), id)
					assert.Equal(t, lynx.Filter(tt.givenOptions.Metadata), filter)
					if tt.givenEmpty {
						return nil, tt.givenDevicesErr
					}
					return devices, tt.givenDevicesErr
				},
				GetFunctionsFunc: func(id int64, filter lynx.Filter) (lynx.FunctionList, error) {
					assert.Equal(t, int64(42), id)
					assert.Equal(t, lynx.Filter(tt.givenOptions.Metadata), filter)
					if tt.givenEmpty {
						return nil, tt.givenFunctionsErr
					}
					return functions, tt.givenFunctionsErr
				},
			}
			s := &Service{V2Client: client}

			gotResult, err := s.SearchObjects(42, tt.givenOptions)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, gotResult)
			} else {
				require.NoError(t, err)
				require.NotNil(t, gotResult)
				assert.Equal(t, tt.wantTotal, gotResult.Total)
				assert.Equal(t, tt.wantLimit, gotResult.Limit)
				assert.Equal(t, int64(tt.givenOptions.Offset), gotResult.Offset)
				assert.Equal(t, tt.wantEntries, gotResult.Results)
				require.Len(t, gotResult.SearchResult.Results, len(tt.wantEntries))
				for i, entry := range tt.wantEntries {
					assert.Equal(t, entry.SearchEntry, gotResult.SearchResult.Results[i])
				}
			}
			assert.Len(t, client.GetInstallationRowCalls(), 1)
			assert.Len(t, client.GetDevicesCalls(), tt.wantDevicesCalls)
			assert.Len(t, client.GetFunctionsCalls(), tt.wantFunctionsCalls)
		})
	}
}

func TestService_LoadSearchModelTypes(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		givenType string
		wantTypes map[int64]string
		wantErr   bool
	}{
		"empty devices":    {givenType: "device", wantTypes: map[int64]string{}},
		"empty functions":  {givenType: "function", wantTypes: map[int64]string{}},
		"unsupported type": {givenType: "unknown", wantErr: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := &Service{V2Client: &v2ClientMock{
				GetDevicesFunc:   func(int64, lynx.Filter) (lynx.DeviceList, error) { return nil, nil },
				GetFunctionsFunc: func(int64, lynx.Filter) (lynx.FunctionList, error) { return nil, nil },
			}}
			gotTypes, err := s.loadSearchModelTypes(42, tt.givenType)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantTypes, gotTypes)
		})
	}
}
