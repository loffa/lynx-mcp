package service

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/IoTOpen/go-lynx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"lynx-mcp/internal/config"
	"lynx-mcp/internal/domain"
)

func TestService_ControlFunction_Validation(t *testing.T) {
	t.Parallel()

	writes := []domain.ControlWrite{{TopicKey: "topic_write", Value: 1}}
	tests := map[string]struct {
		givenAllowControl   bool
		givenMissingMQTT    bool
		givenInstallationID int64
		givenFunctionID     int64
		givenWrites         []domain.ControlWrite
		givenCanceled       bool
	}{
		"control is disabled":   {givenInstallationID: 42, givenFunctionID: 7, givenWrites: writes},
		"publisher is missing":  {givenAllowControl: true, givenMissingMQTT: true, givenInstallationID: 42, givenFunctionID: 7, givenWrites: writes},
		"zero installation":     {givenAllowControl: true, givenFunctionID: 7, givenWrites: writes},
		"negative installation": {givenAllowControl: true, givenInstallationID: -1, givenFunctionID: 7, givenWrites: writes},
		"zero function":         {givenAllowControl: true, givenInstallationID: 42, givenWrites: writes},
		"negative function":     {givenAllowControl: true, givenInstallationID: 42, givenFunctionID: -1, givenWrites: writes},
		"no writes":             {givenAllowControl: true, givenInstallationID: 42, givenFunctionID: 7},
		"canceled context":      {givenAllowControl: true, givenInstallationID: 42, givenFunctionID: 7, givenWrites: writes, givenCanceled: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tt.givenCanceled {
				cancel()
			}
			publisher := &mqttPublisherMock{}
			s := &Service{V2Client: &v2ClientMock{}, MQTTClient: publisher, Permissions: config.PermissionsConfig{AllowControl: tt.givenAllowControl}}
			if tt.givenMissingMQTT {
				s.MQTTClient = nil
			}

			gotResult, err := s.ControlFunction(ctx, tt.givenInstallationID, tt.givenFunctionID, tt.givenWrites)
			require.Error(t, err)
			assert.Nil(t, gotResult)
			assert.Empty(t, publisher.PublishCalls())
		})
	}
}

func TestService_ControlFunction(t *testing.T) {
	t.Parallel()

	meta := lynx.Meta{"topic_write": "actuator/value", "topic_write_mode": "actuator/mode"}
	writes := []domain.ControlWrite{{TopicKey: "topic_write", Value: 1}, {TopicKey: "topic_write_mode", Value: 2}}

	tests := map[string]struct {
		givenMeta               lynx.Meta
		givenWrites             []domain.ControlWrite
		givenFunctionErr        error
		givenInstallationErr    error
		givenCancelOnLoad       bool
		givenCancelAfterPublish bool
		givenPublishErrAt       int
		wantErr                 bool
		wantResult              bool
		wantComplete            bool
		wantStatuses            []string
		wantWriteErrors         []int
		wantPublishCalls        int
	}{
		"all writes are submitted in order": {
			givenMeta: meta, givenWrites: writes, wantResult: true, wantComplete: true,
			wantStatuses: []string{"submitted", "submitted"}, wantPublishCalls: 2,
		},
		"function loading error":     {givenMeta: meta, givenWrites: writes, givenFunctionErr: fmt.Errorf("mock error"), wantErr: true},
		"installation loading error": {givenMeta: meta, givenWrites: writes, givenInstallationErr: fmt.Errorf("mock error"), wantErr: true},
		"invalid second topic prevents every publish": {
			givenMeta: meta, givenWrites: []domain.ControlWrite{writes[0], {TopicKey: "topic_read", Value: 2}}, wantErr: true,
		},
		"invalid second value prevents every publish": {
			givenMeta: meta, givenWrites: []domain.ControlWrite{writes[0], {TopicKey: "topic_write_mode", Value: math.NaN()}}, wantErr: true,
		},
		"duplicate normalized destinations prevent every publish": {
			givenMeta: lynx.Meta{"topic_write": "actuator/value", "topic_write_mode": "100/actuator/value"}, givenWrites: writes, wantErr: true,
		},
		"first publish failure leaves remaining writes unattempted": {
			givenMeta: meta, givenWrites: writes, givenPublishErrAt: 1, wantErr: true, wantResult: true,
			wantStatuses: []string{"failed", "not_attempted"}, wantWriteErrors: []int{0}, wantPublishCalls: 1,
		},
		"second publish failure retains the first submission": {
			givenMeta: meta, givenWrites: writes, givenPublishErrAt: 2, wantErr: true, wantResult: true,
			wantStatuses: []string{"submitted", "failed"}, wantWriteErrors: []int{1}, wantPublishCalls: 2,
		},
		"cancellation during loading prevents publishing": {
			givenMeta: meta, givenWrites: writes, givenCancelOnLoad: true, wantErr: true, wantResult: true,
			wantStatuses: []string{"not_attempted", "not_attempted"}, wantWriteErrors: []int{0},
		},
		"cancellation between writes retains the first submission": {
			givenMeta: meta, givenWrites: writes, givenCancelAfterPublish: true, wantErr: true, wantResult: true,
			wantStatuses: []string{"submitted", "not_attempted"}, wantWriteErrors: []int{1}, wantPublishCalls: 1,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			function := &lynx.Function{ID: 7, Meta: tt.givenMeta}
			installation := &lynx.InstallationRow{ID: 42, ClientID: 100}
			client := &v2ClientMock{
				GetFunctionFunc: func(installationID, functionID int64) (*lynx.Function, error) {
					assert.Equal(t, int64(42), installationID)
					assert.Equal(t, int64(7), functionID)
					return function, tt.givenFunctionErr
				},
				GetInstallationRowFunc: func(id int64) (*lynx.InstallationRow, error) {
					assert.Equal(t, int64(42), id)
					if tt.givenCancelOnLoad {
						cancel()
					}
					return installation, tt.givenInstallationErr
				},
			}
			publishCount := 0
			publisher := &mqttPublisherMock{PublishFunc: func(string, any, byte) error {
				publishCount++
				if tt.givenCancelAfterPublish {
					cancel()
				}
				if publishCount == tt.givenPublishErrAt {
					return fmt.Errorf("mock error")
				}
				return nil
			}}
			s := &Service{V2Client: client, MQTTClient: publisher, Permissions: config.PermissionsConfig{AllowControl: true}}

			gotResult, err := s.ControlFunction(ctx, 42, 7, tt.givenWrites)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if tt.wantResult {
				require.NotNil(t, gotResult)
				assert.Equal(t, installation, gotResult.Installation)
				assert.Equal(t, function, gotResult.Function)
				assert.Equal(t, tt.wantComplete, gotResult.Complete)
				require.Len(t, gotResult.Writes, len(tt.wantStatuses))
				for i, write := range gotResult.Writes {
					assert.Equal(t, tt.givenWrites[i], write.ControlWrite)
					assert.Equal(t, []string{"100/actuator/value", "100/actuator/mode"}[i], write.Topic)
					assert.Equal(t, tt.wantStatuses[i], write.Status)
					assert.Equal(t, domain.ControlValueValidation{Mode: "unconstrained"}, write.Validation)
					if slices.Contains(tt.wantWriteErrors, i) {
						assert.NotEmpty(t, write.Error)
					} else {
						assert.Empty(t, write.Error)
					}
				}
			} else {
				assert.Nil(t, gotResult)
			}
			require.Len(t, publisher.PublishCalls(), tt.wantPublishCalls)
			for i, call := range publisher.PublishCalls() {
				assert.Equal(t, []string{"100/actuator/value", "100/actuator/mode"}[i], call.Topic)
				assert.Equal(t, lynx.Message{Value: tt.givenWrites[i].Value}, call.Payload)
				assert.Equal(t, byte(1), call.Qos)
			}
		})
	}
}

func TestResolveControlTopic(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		givenMeta     lynx.Meta
		givenTopicKey string
		givenClientID int64
		wantTopic     string
		wantErr       bool
	}{
		"relative topic":                            {givenMeta: lynx.Meta{"topic_write": "actuator/set"}, givenTopicKey: "topic_write", givenClientID: 100, wantTopic: "100/actuator/set"},
		"single level topic":                        {givenMeta: lynx.Meta{"topic_write": "set"}, givenTopicKey: "topic_write", givenClientID: 100, wantTopic: "100/set"},
		"channel topic":                             {givenMeta: lynx.Meta{"topic_write_mode": "100/actuator/mode"}, givenTopicKey: "topic_write_mode", givenClientID: 100, wantTopic: "100/actuator/mode"},
		"read key is not writable":                  {givenMeta: lynx.Meta{"topic_read": "sensor/read"}, givenTopicKey: "topic_read", givenClientID: 100, wantErr: true},
		"empty channel key":                         {givenTopicKey: "topic_write_", givenClientID: 100, wantErr: true},
		"missing metadata":                          {givenTopicKey: "topic_write", givenClientID: 100, wantErr: true},
		"blank topic":                               {givenMeta: lynx.Meta{"topic_write": " \t"}, givenTopicKey: "topic_write", givenClientID: 100, wantErr: true},
		"single level wildcard":                     {givenMeta: lynx.Meta{"topic_write": "actuator/+"}, givenTopicKey: "topic_write", givenClientID: 100, wantErr: true},
		"multi level wildcard":                      {givenMeta: lynx.Meta{"topic_write": "actuator/#"}, givenTopicKey: "topic_write", givenClientID: 100, wantErr: true},
		"null character":                            {givenMeta: lynx.Meta{"topic_write": "actuator/\x00"}, givenTopicKey: "topic_write", givenClientID: 100, wantErr: true},
		"invalid UTF8":                              {givenMeta: lynx.Meta{"topic_write": "actuator/\xff"}, givenTopicKey: "topic_write", givenClientID: 100, wantErr: true},
		"missing client ID":                         {givenMeta: lynx.Meta{"topic_write": "set"}, givenTopicKey: "topic_write", wantErr: true},
		"negative client ID":                        {givenMeta: lynx.Meta{"topic_write": "set"}, givenTopicKey: "topic_write", givenClientID: -1, wantErr: true},
		"leading slash":                             {givenMeta: lynx.Meta{"topic_write": "/actuator/set"}, givenTopicKey: "topic_write", givenClientID: 100, wantErr: true},
		"different client prefix":                   {givenMeta: lynx.Meta{"topic_write": "101/actuator/set"}, givenTopicKey: "topic_write", givenClientID: 100, wantErr: true},
		"client prefix without topic":               {givenMeta: lynx.Meta{"topic_write": "100/"}, givenTopicKey: "topic_write", givenClientID: 100, wantErr: true},
		"maximum length includes the added prefix":  {givenMeta: lynx.Meta{"topic_write": strings.Repeat("a", 65_531)}, givenTopicKey: "topic_write", givenClientID: 100, wantTopic: "100/" + strings.Repeat("a", 65_531)},
		"topic exceeds maximum after adding prefix": {givenMeta: lynx.Meta{"topic_write": strings.Repeat("a", 65_532)}, givenTopicKey: "topic_write", givenClientID: 100, wantErr: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			gotTopic, err := resolveControlTopic(tt.givenMeta, tt.givenTopicKey, tt.givenClientID)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantTopic, gotTopic)
		})
	}
}

func TestValidateControlValue(t *testing.T) {
	t.Parallel()

	zero, ten, twenty := 0.0, 10.0, 20.0
	tests := map[string]struct {
		givenMeta      lynx.Meta
		givenTopicKey  string
		givenValue     float64
		wantValidation domain.ControlValueValidation
		wantErr        bool
	}{
		"unconstrained value": {givenTopicKey: "topic_write", givenValue: -123, wantValidation: domain.ControlValueValidation{Mode: "unconstrained"}},
		"NaN":                 {givenTopicKey: "topic_write", givenValue: math.NaN(), wantErr: true},
		"positive infinity":   {givenTopicKey: "topic_write", givenValue: math.Inf(1), wantErr: true},
		"negative infinity":   {givenTopicKey: "topic_write", givenValue: math.Inf(-1), wantErr: true},
		"inclusive minimum": {
			givenTopicKey: "topic_write", givenMeta: lynx.Meta{"min": "0", "max": "10"}, givenValue: 0,
			wantValidation: domain.ControlValueValidation{Mode: "range", Min: &zero, Max: &ten, MinKey: "min", MaxKey: "max"},
		},
		"inclusive maximum": {
			givenTopicKey: "topic_write", givenMeta: lynx.Meta{"min": "0", "max": "10"}, givenValue: 10,
			wantValidation: domain.ControlValueValidation{Mode: "range", Min: &zero, Max: &ten, MinKey: "min", MaxKey: "max"},
		},
		"below minimum":           {givenTopicKey: "topic_write", givenMeta: lynx.Meta{"min": "0"}, givenValue: -1, wantErr: true},
		"above maximum":           {givenTopicKey: "topic_write", givenMeta: lynx.Meta{"max": "10"}, givenValue: 11, wantErr: true},
		"minimum exceeds maximum": {givenTopicKey: "topic_write", givenMeta: lynx.Meta{"min": "10", "max": "0"}, givenValue: 5, wantErr: true},
		"invalid minimum":         {givenTopicKey: "topic_write", givenMeta: lynx.Meta{"min": "invalid"}, wantErr: true},
		"invalid maximum":         {givenTopicKey: "topic_write", givenMeta: lynx.Meta{"max": "NaN"}, wantErr: true},
		"channel bounds override global bounds": {
			givenTopicKey: "topic_write_mode", givenMeta: lynx.Meta{"min": "0", "max": "10", "min_mode": "10", "max_mode": "20"}, givenValue: 15,
			wantValidation: domain.ControlValueValidation{Mode: "range", Min: &ten, Max: &twenty, MinKey: "min_mode", MaxKey: "max_mode"},
		},
		"missing channel bound falls back independently": {
			givenTopicKey: "topic_write_mode", givenMeta: lynx.Meta{"min": "0", "max": "10", "max_mode": "20"}, givenValue: 15,
			wantValidation: domain.ControlValueValidation{Mode: "range", Min: &zero, Max: &twenty, MinKey: "min", MaxKey: "max_mode"},
		},
		"invalid channel bound does not fall back": {givenTopicKey: "topic_write_mode", givenMeta: lynx.Meta{"min": "0", "min_mode": ""}, wantErr: true},
		"matching state keys are sorted": {
			givenTopicKey: "topic_write", givenMeta: lynx.Meta{"state_on": "1", "state_active": "1", "state_off": "0", "state_": "ignored"}, givenValue: 1,
			wantValidation: domain.ControlValueValidation{Mode: "state", StateKeys: []string{"state_active", "state_on"}},
		},
		"state can allow a value outside the range": {
			givenTopicKey: "topic_write", givenMeta: lynx.Meta{"min": "0", "max": "10", "state_auto": "-1"}, givenValue: -1,
			wantValidation: domain.ControlValueValidation{Mode: "state", StateKeys: []string{"state_auto"}, Min: &zero, Max: &ten, MinKey: "min", MaxKey: "max"},
		},
		"unmatched state without a range": {givenTopicKey: "topic_write", givenMeta: lynx.Meta{"state_on": "1"}, givenValue: 2, wantErr: true},
		"range allows values between declared states": {
			givenTopicKey: "topic_write", givenMeta: lynx.Meta{"min": "0", "max": "10", "state_on": "1"}, givenValue: 5,
			wantValidation: domain.ControlValueValidation{Mode: "range", Min: &zero, Max: &ten, MinKey: "min", MaxKey: "max"},
		},
		"channel states exclude global states": {
			givenTopicKey: "topic_write_mode", givenMeta: lynx.Meta{"state_on": "invalid", "state_mode_on": "1", "state_mode_": "ignored"}, givenValue: 1,
			wantValidation: domain.ControlValueValidation{Mode: "state", StateKeys: []string{"state_mode_on"}},
		},
		"channel without its own states uses global states": {
			givenTopicKey: "topic_write_mode", givenMeta: lynx.Meta{"state_on": "1"}, givenValue: 1,
			wantValidation: domain.ControlValueValidation{Mode: "state", StateKeys: []string{"state_on"}},
		},
		"invalid state metadata": {givenTopicKey: "topic_write", givenMeta: lynx.Meta{"state_on": "invalid"}, wantErr: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			gotValidation, err := validateControlValue(tt.givenMeta, tt.givenTopicKey, tt.givenValue)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantValidation, gotValidation)
			}
		})
	}
}

func TestParseControlNumber(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		givenValue string
		wantValue  float64
		wantErr    bool
	}{
		"trimmed decimal":     {givenValue: " \t-1.25 ", wantValue: -1.25},
		"scientific notation": {givenValue: "1e2", wantValue: 100},
		"empty":               {wantErr: true},
		"invalid":             {givenValue: "invalid", wantErr: true},
		"NaN":                 {givenValue: "NaN", wantErr: true},
		"positive infinity":   {givenValue: "+Inf", wantErr: true},
		"negative infinity":   {givenValue: "-Inf", wantErr: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			gotValue, err := parseControlNumber(lynx.Meta{"min": tt.givenValue}, "min")
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantValue, gotValue)
		})
	}
}
