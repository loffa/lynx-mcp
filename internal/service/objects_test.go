package service

import (
	"fmt"
	"testing"

	"github.com/IoTOpen/go-lynx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lynx-mcp/internal/domain"
)

func TestService_GetObject(t *testing.T) {
	t.Parallel()

	installation := &lynx.InstallationRow{ID: 42}
	function := &lynx.Function{ID: 7, Meta: lynx.Meta{"device_id": "9"}}
	device := &lynx.Device{ID: 7}

	tests := map[string]struct {
		givenType             string
		givenID               int64
		givenInstallationID   int64
		givenFunctionErr      error
		givenDeviceErr        error
		givenInstallationErr  error
		wantResult            *domain.GetObjectResult
		wantErr               bool
		wantFunctionCalls     int
		wantDeviceCalls       int
		wantInstallationCalls int
	}{
		"unsupported type":                      {givenType: "unknown", givenID: 7, givenInstallationID: 42, wantErr: true},
		"zero object ID":                        {givenType: "device", givenInstallationID: 42, wantErr: true},
		"negative object ID":                    {givenType: "device", givenID: -1, givenInstallationID: 42, wantErr: true},
		"function requires installation":        {givenType: "function", givenID: 7, wantErr: true},
		"device requires positive installation": {givenType: "device", givenID: 7, givenInstallationID: -1, wantErr: true},
		"installation IDs must agree":           {givenType: "installation", givenID: 42, givenInstallationID: 43, wantErr: true},
		"installation ID is inferred from object ID": {
			givenType: "installation", givenID: 42,
			wantResult:            &domain.GetObjectResult{Type: "installation", ID: 42, Installation: installation},
			wantInstallationCalls: 1,
		},
		"explicit matching installation ID is accepted": {
			givenType: "installation", givenID: 42, givenInstallationID: 42,
			wantResult:            &domain.GetObjectResult{Type: "installation", ID: 42, Installation: installation},
			wantInstallationCalls: 1,
		},
		"function is loaded without relationships": {
			givenType: "function", givenID: 7, givenInstallationID: 42,
			wantResult:        &domain.GetObjectResult{Type: "function", ID: 7, Installation: installation, Function: function},
			wantFunctionCalls: 1, wantInstallationCalls: 1,
		},
		"device is loaded without relationships": {
			givenType: "device", givenID: 7, givenInstallationID: 42,
			wantResult:      &domain.GetObjectResult{Type: "device", ID: 7, Installation: installation, Device: device},
			wantDeviceCalls: 1, wantInstallationCalls: 1,
		},
		"function client error": {
			givenType: "function", givenID: 7, givenInstallationID: 42, givenFunctionErr: fmt.Errorf("mock error"),
			wantErr: true, wantFunctionCalls: 1,
		},
		"device client error": {
			givenType: "device", givenID: 7, givenInstallationID: 42, givenDeviceErr: fmt.Errorf("mock error"),
			wantErr: true, wantDeviceCalls: 1,
		},
		"installation client error": {
			givenType: "device", givenID: 7, givenInstallationID: 42, givenInstallationErr: fmt.Errorf("mock error"),
			wantErr: true, wantDeviceCalls: 1, wantInstallationCalls: 1,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client := &v2ClientMock{
				GetFunctionFunc: func(installationID, functionID int64) (*lynx.Function, error) {
					assert.Equal(t, int64(42), installationID)
					assert.Equal(t, tt.givenID, functionID)
					return function, tt.givenFunctionErr
				},
				GetDeviceFunc: func(installationID, deviceID int64) (*lynx.Device, error) {
					assert.Equal(t, int64(42), installationID)
					assert.Equal(t, tt.givenID, deviceID)
					return device, tt.givenDeviceErr
				},
				GetInstallationRowFunc: func(installationID int64) (*lynx.InstallationRow, error) {
					assert.Equal(t, int64(42), installationID)
					return installation, tt.givenInstallationErr
				},
			}
			s := &Service{V2Client: client}

			gotResult, err := s.GetObject(tt.givenType, tt.givenID, tt.givenInstallationID, false)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantResult, gotResult)
			assert.Len(t, client.GetFunctionCalls(), tt.wantFunctionCalls)
			assert.Len(t, client.GetDeviceCalls(), tt.wantDeviceCalls)
			assert.Len(t, client.GetInstallationRowCalls(), tt.wantInstallationCalls)
		})
	}
}

func TestService_GetObject_Relationships(t *testing.T) {
	t.Parallel()

	installation := &lynx.InstallationRow{ID: 42}
	device := &lynx.Device{ID: 7}

	tests := map[string]struct {
		givenType          string
		givenMeta          lynx.Meta
		givenDeviceErr     error
		givenFunctions     lynx.FunctionList
		givenFunctionsErr  error
		wantDevice         *lynx.Device
		wantFunctions      []domain.FunctionSummary
		wantWarningCodes   []string
		wantDeviceCalls    int
		wantFunctionsCalls int
	}{
		"installation has no relationships to load": {givenType: "installation"},
		"missing device link":                       {givenType: "function"},
		"blank device link":                         {givenType: "function", givenMeta: lynx.Meta{"device_id": " \t"}},
		"invalid device link":                       {givenType: "function", givenMeta: lynx.Meta{"device_id": "invalid"}, wantWarningCodes: []string{"invalid_device_id"}},
		"zero device link":                          {givenType: "function", givenMeta: lynx.Meta{"device_id": "0"}, wantWarningCodes: []string{"invalid_device_id"}},
		"negative device link":                      {givenType: "function", givenMeta: lynx.Meta{"device_id": "-1"}, wantWarningCodes: []string{"invalid_device_id"}},
		"device link is trimmed and loaded": {
			givenType: "function", givenMeta: lynx.Meta{"device_id": " 7 "},
			wantDevice: device, wantDeviceCalls: 1,
		},
		"related device failure becomes a warning": {
			givenType: "function", givenMeta: lynx.Meta{"device_id": "7"}, givenDeviceErr: fmt.Errorf("mock error"),
			wantWarningCodes: []string{"related_device_unavailable"}, wantDeviceCalls: 1,
		},
		"empty related functions remain a loaded empty slice": {
			givenType: "device", wantDevice: device, wantDeviceCalls: 1,
			wantFunctions: []domain.FunctionSummary{}, wantFunctionsCalls: 1,
		},
		"related functions are summarized and sorted by ID": {
			givenType: "device", wantDevice: device, wantDeviceCalls: 1,
			givenFunctions: lynx.FunctionList{
				{ID: 3, Type: "humidity", Meta: lynx.Meta{"name": "Humidity"}},
				{ID: 1, Type: "temperature", Meta: lynx.Meta{"name": "Temperature"}},
			},
			wantFunctions: []domain.FunctionSummary{
				{ID: 1, Name: "Temperature", ModelType: "temperature"},
				{ID: 3, Name: "Humidity", ModelType: "humidity"},
			},
			wantFunctionsCalls: 1,
		},
		"related function failure becomes a warning": {
			givenType: "device", givenFunctionsErr: fmt.Errorf("mock error"),
			wantDevice: device, wantDeviceCalls: 1, wantFunctionsCalls: 1,
			wantWarningCodes: []string{"related_functions_unavailable"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			function := &lynx.Function{ID: 9, Meta: tt.givenMeta}
			client := &v2ClientMock{
				GetInstallationRowFunc: func(id int64) (*lynx.InstallationRow, error) {
					assert.Equal(t, int64(42), id)
					return installation, nil
				},
				GetFunctionFunc: func(installationID, functionID int64) (*lynx.Function, error) {
					assert.Equal(t, int64(42), installationID)
					assert.Equal(t, int64(9), functionID)
					return function, nil
				},
				GetDeviceFunc: func(installationID, deviceID int64) (*lynx.Device, error) {
					assert.Equal(t, int64(42), installationID)
					assert.Equal(t, int64(7), deviceID)
					return device, tt.givenDeviceErr
				},
				GetFunctionsFunc: func(installationID int64, filter lynx.Filter) (lynx.FunctionList, error) {
					assert.Equal(t, int64(42), installationID)
					assert.Equal(t, lynx.Filter{"device_id": "7"}, filter)
					return tt.givenFunctions, tt.givenFunctionsErr
				},
			}
			s := &Service{V2Client: client}
			objectID := map[string]int64{"installation": 42, "function": 9, "device": 7}[tt.givenType]

			gotResult, err := s.GetObject(tt.givenType, objectID, 42, true)
			require.NoError(t, err)
			require.NotNil(t, gotResult)
			assert.Equal(t, tt.givenType, gotResult.Type)
			assert.Equal(t, objectID, gotResult.ID)
			assert.Equal(t, installation, gotResult.Installation)
			if tt.givenType == "function" {
				assert.Equal(t, function, gotResult.Function)
			} else {
				assert.Nil(t, gotResult.Function)
			}
			assert.Equal(t, tt.wantDevice, gotResult.Device)
			assert.Equal(t, tt.wantFunctions, gotResult.Functions)
			var gotWarningCodes []string
			for _, warning := range gotResult.Warnings {
				gotWarningCodes = append(gotWarningCodes, warning.Code)
			}
			assert.Equal(t, tt.wantWarningCodes, gotWarningCodes)
			assert.Len(t, client.GetDeviceCalls(), tt.wantDeviceCalls)
			assert.Len(t, client.GetFunctionsCalls(), tt.wantFunctionsCalls)
		})
	}
}
