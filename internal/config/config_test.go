package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"lynx-mcp/internal/config"
)

func TestConfig_Validate(t *testing.T) {
	t.Parallel()

	http := config.HTTPConfig{
		Listen:            ":8080",
		ReadTimeout:       time.Second,
		ReadHeaderTimeout: time.Second,
		WriteTimeout:      config.LynxRequestTimeout,
		IdleTimeout:       time.Minute,
		ShutdownTimeout:   time.Second,
	}
	lynx := config.LynxConfig{
		APIBase: "https://lynx.example.com",
		APIKey:  "test-key",
		MQTT:    config.MQTTConfig{Broker: "tcp://localhost:1883", ClientID: "test-client"},
	}

	tests := map[string]struct {
		givenConfig config.Config
		wantErr     bool
	}{
		"valid with control disabled": {
			givenConfig: config.Config{HTTP: http, Lynx: lynx},
		},
		"valid with control enabled": {
			givenConfig: config.Config{HTTP: http, Lynx: lynx, Permissions: config.PermissionsConfig{AllowControl: true}},
		},
		"invalid HTTP config": {
			givenConfig: config.Config{Lynx: lynx},
			wantErr:     true,
		},
		"invalid Lynx config": {
			givenConfig: config.Config{HTTP: http},
			wantErr:     true,
		},
		"invalid nested MQTT config": {
			givenConfig: config.Config{HTTP: http, Lynx: config.LynxConfig{APIBase: lynx.APIBase, APIKey: lynx.APIKey}},
			wantErr:     true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := tt.givenConfig.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHTTPConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		givenChange func(*config.HTTPConfig)
		wantErr     bool
	}{
		"valid config": {},
		"empty listen address": {
			givenChange: func(c *config.HTTPConfig) { c.Listen = "" },
			wantErr:     true,
		},
		"zero read timeout": {
			givenChange: func(c *config.HTTPConfig) { c.ReadTimeout = 0 },
			wantErr:     true,
		},
		"negative read timeout": {
			givenChange: func(c *config.HTTPConfig) { c.ReadTimeout = -time.Second },
			wantErr:     true,
		},
		"zero read header timeout": {
			givenChange: func(c *config.HTTPConfig) { c.ReadHeaderTimeout = 0 },
			wantErr:     true,
		},
		"negative read header timeout": {
			givenChange: func(c *config.HTTPConfig) { c.ReadHeaderTimeout = -time.Second },
			wantErr:     true,
		},
		"zero write timeout": {
			givenChange: func(c *config.HTTPConfig) { c.WriteTimeout = 0 },
			wantErr:     true,
		},
		"negative write timeout": {
			givenChange: func(c *config.HTTPConfig) { c.WriteTimeout = -time.Second },
			wantErr:     true,
		},
		"write timeout below Lynx request timeout": {
			givenChange: func(c *config.HTTPConfig) { c.WriteTimeout = config.LynxRequestTimeout - time.Nanosecond },
			wantErr:     true,
		},
		"write timeout equals Lynx request timeout": {
			givenChange: func(c *config.HTTPConfig) { c.WriteTimeout = config.LynxRequestTimeout },
		},
		"zero idle timeout": {
			givenChange: func(c *config.HTTPConfig) { c.IdleTimeout = 0 },
			wantErr:     true,
		},
		"negative idle timeout": {
			givenChange: func(c *config.HTTPConfig) { c.IdleTimeout = -time.Second },
			wantErr:     true,
		},
		"zero shutdown timeout": {
			givenChange: func(c *config.HTTPConfig) { c.ShutdownTimeout = 0 },
			wantErr:     true,
		},
		"negative shutdown timeout": {
			givenChange: func(c *config.HTTPConfig) { c.ShutdownTimeout = -time.Second },
			wantErr:     true,
		},
		"positive timeout boundaries": {
			givenChange: func(c *config.HTTPConfig) {
				c.ReadTimeout = time.Nanosecond
				c.ReadHeaderTimeout = time.Nanosecond
				c.WriteTimeout = config.LynxRequestTimeout
				c.IdleTimeout = time.Nanosecond
				c.ShutdownTimeout = time.Nanosecond
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			givenConfig := config.HTTPConfig{
				Listen:            ":8080",
				ReadTimeout:       time.Second,
				ReadHeaderTimeout: time.Second,
				WriteTimeout:      time.Minute,
				IdleTimeout:       time.Minute,
				ShutdownTimeout:   time.Second,
			}
			if tt.givenChange != nil {
				tt.givenChange(&givenConfig)
			}

			err := givenConfig.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLynxConfig_Validate(t *testing.T) {
	t.Parallel()

	mqtt := config.MQTTConfig{Broker: "tcp://localhost:1883", ClientID: "test-client"}
	tests := map[string]struct {
		givenConfig config.LynxConfig
		wantErr     bool
	}{
		"HTTPS API": {
			givenConfig: config.LynxConfig{APIBase: "https://lynx.example.com", APIKey: "test-key", MQTT: mqtt},
		},
		"HTTP API with port and path": {
			givenConfig: config.LynxConfig{APIBase: "http://localhost:8080/api", APIKey: "test-key", MQTT: mqtt},
		},
		"missing API key": {
			givenConfig: config.LynxConfig{APIBase: "https://lynx.example.com", MQTT: mqtt},
			wantErr:     true,
		},
		"empty API base": {
			givenConfig: config.LynxConfig{APIKey: "test-key", MQTT: mqtt},
			wantErr:     true,
		},
		"malformed API URL": {
			givenConfig: config.LynxConfig{APIBase: "https://lynx.example.com/%zz", APIKey: "test-key", MQTT: mqtt},
			wantErr:     true,
		},
		"relative API path": {
			givenConfig: config.LynxConfig{APIBase: "/api", APIKey: "test-key", MQTT: mqtt},
			wantErr:     true,
		},
		"API URL without host": {
			givenConfig: config.LynxConfig{APIBase: "https:///api", APIKey: "test-key", MQTT: mqtt},
			wantErr:     true,
		},
		"unsupported API scheme": {
			givenConfig: config.LynxConfig{APIBase: "ftp://lynx.example.com", APIKey: "test-key", MQTT: mqtt},
			wantErr:     true,
		},
		"invalid MQTT config": {
			givenConfig: config.LynxConfig{APIBase: "https://lynx.example.com", APIKey: "test-key"},
			wantErr:     true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := tt.givenConfig.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMQTTConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		givenBroker   string
		givenClientID string
		wantErr       bool
	}{
		"TCP broker":                {givenBroker: "tcp://localhost:1883", givenClientID: "test-client"},
		"MQTT broker":               {givenBroker: "mqtt://localhost:1883", givenClientID: "test-client"},
		"SSL broker":                {givenBroker: "ssl://localhost:8883", givenClientID: "test-client"},
		"TLS broker":                {givenBroker: "tls://localhost:8883", givenClientID: "test-client"},
		"MQTTS broker":              {givenBroker: "mqtts://localhost:8883", givenClientID: "test-client"},
		"MQTT over SSL":             {givenBroker: "mqtt+ssl://localhost:8883", givenClientID: "test-client"},
		"TCPS broker":               {givenBroker: "tcps://localhost:8883", givenClientID: "test-client"},
		"WebSocket broker":          {givenBroker: "ws://localhost:8080/mqtt", givenClientID: "test-client"},
		"secure WebSocket broker":   {givenBroker: "wss://localhost:8443/mqtt", givenClientID: "test-client"},
		"empty broker":              {givenClientID: "test-client", wantErr: true},
		"malformed broker":          {givenBroker: "tcp://localhost/%zz", givenClientID: "test-client", wantErr: true},
		"relative broker path":      {givenBroker: "/mqtt", givenClientID: "test-client", wantErr: true},
		"broker without host":       {givenBroker: "tcp:///mqtt", givenClientID: "test-client", wantErr: true},
		"unsupported broker scheme": {givenBroker: "http://localhost:1883", givenClientID: "test-client", wantErr: true},
		"empty client ID":           {givenBroker: "tcp://localhost:1883", wantErr: true},
		"whitespace client ID":      {givenBroker: "tcp://localhost:1883", givenClientID: " \t\n", wantErr: true},
		"client ID with null":       {givenBroker: "tcp://localhost:1883", givenClientID: "test\x00client", wantErr: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			givenConfig := config.MQTTConfig{Broker: tt.givenBroker, ClientID: tt.givenClientID}
			err := givenConfig.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
