package service

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/IoTOpen/go-lynx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_ListAutomations_Validation(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		givenInstallationID int64
		givenOptions        AutomationOptions
	}{
		"zero installation":     {},
		"negative installation": {givenInstallationID: -1},
		"negative instance":     {givenInstallationID: 42, givenOptions: AutomationOptions{InstanceID: -1}},
		"negative app":          {givenInstallationID: 42, givenOptions: AutomationOptions{AppID: -1}},
		"negative limit":        {givenInstallationID: 42, givenOptions: AutomationOptions{Limit: -1}},
		"excessive limit":       {givenInstallationID: 42, givenOptions: AutomationOptions{Limit: 10_001}},
		"negative offset":       {givenInstallationID: 42, givenOptions: AutomationOptions{Offset: -1}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := &Service{}
			gotResult, err := s.ListAutomations(tt.givenInstallationID, tt.givenOptions)
			require.Error(t, err)
			assert.Nil(t, gotResult)
		})
	}
}

func TestService_ListAutomations(t *testing.T) {
	t.Parallel()

	installation := &lynx.InstallationRow{ID: 42}
	instances := []*lynx.EdgeAppConfig{
		{ID: 3, AppID: 8, Version: "v1"},
		{ID: 1, AppID: 7, Version: "v1"},
		{ID: 2, AppID: 7, Version: "v1"},
		{ID: 4, AppID: 7, Version: "v2"},
	}

	tests := map[string]struct {
		givenOptions         AutomationOptions
		givenEmpty           bool
		givenInstallationErr error
		givenInstancesErr    error
		givenAppErr          error
		givenConfigErr       error
		wantIDs              []int64
		wantTotal            int
		wantLimit            int
		wantErr              bool
		wantListCalls        int
		wantInstanceCalls    int
		wantAppIDs           []int64
		wantConfigKeys       []string
		wantWarningCodes     []string
	}{
		"instances are sorted and shared app details are cached": {
			wantIDs: []int64{1, 2, 3, 4}, wantTotal: 4, wantLimit: 100, wantListCalls: 1, wantAppIDs: []int64{7, 8},
		},
		"config cache is keyed by app and version": {
			givenOptions: AutomationOptions{IncludeConfigOptions: true}, wantIDs: []int64{1, 2, 3, 4}, wantTotal: 4, wantLimit: 100,
			wantListCalls: 1, wantAppIDs: []int64{7, 8}, wantConfigKeys: []string{"7:v1", "8:v1", "7:v2"},
		},
		"app filter is applied before pagination and enrichment": {
			givenOptions: AutomationOptions{AppID: 7, Limit: 1, Offset: 1}, wantIDs: []int64{2}, wantTotal: 3, wantLimit: 1,
			wantListCalls: 1, wantAppIDs: []int64{7},
		},
		"single instance uses its dedicated client method": {
			givenOptions: AutomationOptions{InstanceID: 3}, wantIDs: []int64{3}, wantTotal: 1, wantLimit: 100,
			wantInstanceCalls: 1, wantAppIDs: []int64{8},
		},
		"single instance still respects the app filter": {
			givenOptions: AutomationOptions{InstanceID: 3, AppID: 7}, wantIDs: []int64{}, wantLimit: 100, wantInstanceCalls: 1,
		},
		"offset beyond results skips enrichment": {
			givenOptions: AutomationOptions{Offset: 99, IncludeConfigOptions: true}, wantIDs: []int64{}, wantTotal: 4, wantLimit: 100, wantListCalls: 1,
		},
		"empty installation": {givenEmpty: true, wantIDs: []int64{}, wantLimit: 100, wantListCalls: 1},
		"failed app lookups are cached and reported as warnings": {
			givenAppErr: fmt.Errorf("mock error"), wantIDs: []int64{1, 2, 3, 4}, wantTotal: 4, wantLimit: 100, wantListCalls: 1,
			wantAppIDs: []int64{7, 8}, wantWarningCodes: []string{"app_unavailable"},
		},
		"failed config lookups are cached and reported as warnings": {
			givenOptions: AutomationOptions{IncludeConfigOptions: true}, givenConfigErr: fmt.Errorf("mock error"),
			wantIDs: []int64{1, 2, 3, 4}, wantTotal: 4, wantLimit: 100, wantListCalls: 1, wantAppIDs: []int64{7, 8},
			wantConfigKeys: []string{"7:v1", "8:v1", "7:v2"}, wantWarningCodes: []string{"config_options_unavailable"},
		},
		"app failure does not prevent config enrichment": {
			givenOptions: AutomationOptions{InstanceID: 3, IncludeConfigOptions: true}, givenAppErr: fmt.Errorf("mock error"),
			wantIDs: []int64{3}, wantTotal: 1, wantLimit: 100, wantInstanceCalls: 1, wantAppIDs: []int64{8},
			wantConfigKeys: []string{"8:v1"}, wantWarningCodes: []string{"app_unavailable"},
		},
		"installation client error": {givenInstallationErr: fmt.Errorf("mock error"), wantErr: true},
		"instance list error":       {givenInstancesErr: fmt.Errorf("mock error"), wantErr: true, wantListCalls: 1},
		"single instance error":     {givenOptions: AutomationOptions{InstanceID: 3}, givenInstancesErr: fmt.Errorf("mock error"), wantErr: true, wantInstanceCalls: 1},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			client := &v2ClientMock{
				GetInstallationRowFunc: func(id int64) (*lynx.InstallationRow, error) {
					assert.Equal(t, int64(42), id)
					return installation, tt.givenInstallationErr
				},
				GetConfiguredEdgeAppsFunc: func(id int64) ([]*lynx.EdgeAppConfig, error) {
					assert.Equal(t, int64(42), id)
					if tt.givenEmpty {
						return nil, nil
					}
					return instances, tt.givenInstancesErr
				},
				GetEdgeAppInstanceFunc: func(installationID, instanceID int64) (*lynx.EdgeAppConfig, error) {
					assert.Equal(t, int64(42), installationID)
					assert.Equal(t, int64(3), instanceID)
					return instances[0], tt.givenInstancesErr
				},
				GetEdgeAppFunc: func(id int64) (*lynx.EdgeApp, error) {
					return &lynx.EdgeApp{ID: id, Name: "App"}, tt.givenAppErr
				},
				GetEdgeAppConfigOptionsFunc: func(int64, string) (json.RawMessage, error) {
					return json.RawMessage(`{"value":9007199254740993}`), tt.givenConfigErr
				},
			}
			s := &Service{V2Client: client}
			gotResult, err := s.ListAutomations(42, tt.givenOptions)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, gotResult)
			} else {
				require.NoError(t, err)
				require.NotNil(t, gotResult)
				assert.Equal(t, installation, gotResult.Installation)
				assert.Equal(t, tt.wantTotal, gotResult.Total)
				assert.Equal(t, tt.wantLimit, gotResult.Limit)
				assert.Equal(t, tt.givenOptions.Offset, gotResult.Offset)
				require.Len(t, gotResult.Automations, len(tt.wantIDs))
				for i, entry := range gotResult.Automations {
					assert.Equal(t, tt.wantIDs[i], entry.Instance.ID)
					if tt.givenAppErr != nil {
						assert.Nil(t, entry.App)
					} else {
						assert.Equal(t, json.Number(fmt.Sprint(entry.Instance.AppID)), entry.App["id"])
						assert.Equal(t, "App", entry.App["name"])
					}
					if tt.givenOptions.IncludeConfigOptions && tt.givenConfigErr == nil {
						assert.Equal(t, map[string]any{"value": json.Number("9007199254740993")}, entry.ConfigOptions)
					} else {
						assert.Nil(t, entry.ConfigOptions)
					}
					var gotWarningCodes []string
					for _, warning := range entry.Warnings {
						gotWarningCodes = append(gotWarningCodes, warning.Code)
					}
					assert.Equal(t, tt.wantWarningCodes, gotWarningCodes)
				}
			}
			assert.Len(t, client.GetInstallationRowCalls(), 1)
			assert.Len(t, client.GetConfiguredEdgeAppsCalls(), tt.wantListCalls)
			assert.Len(t, client.GetEdgeAppInstanceCalls(), tt.wantInstanceCalls)
			var gotAppIDs []int64
			for _, call := range client.GetEdgeAppCalls() {
				gotAppIDs = append(gotAppIDs, call.AppID)
			}
			assert.Equal(t, tt.wantAppIDs, gotAppIDs)
			var gotConfigKeys []string
			for _, call := range client.GetEdgeAppConfigOptionsCalls() {
				gotConfigKeys = append(gotConfigKeys, fmt.Sprintf("%d:%s", call.AppID, call.Version))
			}
			assert.Equal(t, tt.wantConfigKeys, gotConfigKeys)
		})
	}
}

func TestService_LoadAutomationApp(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		givenAppID int64
		givenErr   error
		wantErr    bool
		wantCalls  int
	}{
		"zero ID":      {wantErr: true},
		"negative ID":  {givenAppID: -1, wantErr: true},
		"client error": {givenAppID: 7, givenErr: fmt.Errorf("mock error"), wantErr: true, wantCalls: 1},
		"native fields and integer precision are preserved": {givenAppID: 9_007_199_254_740_993, wantCalls: 1},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client := &v2ClientMock{GetEdgeAppFunc: func(id int64) (*lynx.EdgeApp, error) {
				assert.Equal(t, tt.givenAppID, id)
				return &lynx.EdgeApp{ID: id, Name: "App", Publisher: &lynx.Publisher{ID: 2, Name: "Publisher"}}, tt.givenErr
			}}
			s := &Service{V2Client: client}
			gotApp, err := s.loadAutomationApp(tt.givenAppID)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, gotApp)
			} else {
				require.NoError(t, err)
				assert.Equal(t, json.Number("9007199254740993"), gotApp["id"])
				assert.Equal(t, "App", gotApp["name"])
				assert.Equal(t, map[string]any{"id": json.Number("2"), "name": "Publisher"}, gotApp["publisher"])
			}
			assert.Len(t, client.GetEdgeAppCalls(), tt.wantCalls)
		})
	}
}

func TestService_LoadAutomationConfigOptions(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		givenAppID   int64
		givenVersion string
		givenConfig  json.RawMessage
		givenErr     error
		wantConfig   any
		wantVersion  string
		wantErr      bool
		wantCalls    int
	}{
		"zero app ID":      {givenVersion: "v1", wantErr: true},
		"negative app ID":  {givenAppID: -1, givenVersion: "v1", wantErr: true},
		"blank version":    {givenAppID: 7, givenVersion: " \t", wantErr: true},
		"client error":     {givenAppID: 7, givenVersion: "v1", givenErr: fmt.Errorf("mock error"), wantVersion: "v1", wantErr: true, wantCalls: 1},
		"invalid response": {givenAppID: 7, givenVersion: "v1", givenConfig: json.RawMessage(`{`), wantVersion: "v1", wantErr: true, wantCalls: 1},
		"null response":    {givenAppID: 7, givenVersion: "v1", givenConfig: json.RawMessage(" \nnull\t"), wantVersion: "v1", wantErr: true, wantCalls: 1},
		"version is escaped and numeric precision is retained": {
			givenAppID: 7, givenVersion: "v1+beta &test", givenConfig: json.RawMessage(`[{"max":9007199254740993}]`),
			wantConfig: []any{map[string]any{"max": json.Number("9007199254740993")}}, wantVersion: "v1%2Bbeta+%26test", wantCalls: 1,
		},
		"empty options are accepted": {givenAppID: 7, givenVersion: "v1", givenConfig: json.RawMessage(`[]`), wantConfig: []any{}, wantVersion: "v1", wantCalls: 1},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client := &v2ClientMock{GetEdgeAppConfigOptionsFunc: func(id int64, version string) (json.RawMessage, error) {
				assert.Equal(t, tt.givenAppID, id)
				assert.Equal(t, tt.wantVersion, version)
				return tt.givenConfig, tt.givenErr
			}}
			s := &Service{V2Client: client}
			gotConfig, err := s.loadAutomationConfigOptions(tt.givenAppID, tt.givenVersion)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantConfig, gotConfig)
			assert.Len(t, client.GetEdgeAppConfigOptionsCalls(), tt.wantCalls)
		})
	}
}
