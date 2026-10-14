package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// LynxRequestTimeout bounds individual requests to the Lynx API.
const LynxRequestTimeout = 30 * time.Second

// Config holds the server, Lynx connection, and permission settings.
type Config struct {
	HTTP        HTTPConfig        `mapstructure:"http"`
	Lynx        LynxConfig        `mapstructure:"lynx"`
	Permissions PermissionsConfig `mapstructure:"permissions"`
}

// PermissionsConfig controls access to operations that change objects or states.
type PermissionsConfig struct {
	AllowControl bool `mapstructure:"allow_control"`
}

// HTTPConfig defines the listening address and HTTP server timeouts.
type HTTPConfig struct {
	Listen            string        `mapstructure:"listen"`
	ReadTimeout       time.Duration `mapstructure:"read_timeout"`
	ReadHeaderTimeout time.Duration `mapstructure:"read_header_timeout"`
	WriteTimeout      time.Duration `mapstructure:"write_timeout"`
	IdleTimeout       time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout   time.Duration `mapstructure:"shutdown_timeout"`
}

// LynxConfig configures authentication and connections to Lynx.
type LynxConfig struct {
	APIBase string     `mapstructure:"api_base"`
	APIKey  string     `mapstructure:"api_key"`
	MQTT    MQTTConfig `mapstructure:"mqtt"`
}

// MQTTConfig identifies the broker and this server's MQTT client.
type MQTTConfig struct {
	Broker   string `mapstructure:"broker"`
	ClientID string `mapstructure:"client_id"`
}

// Validate checks the HTTP and Lynx configuration.
func (c *Config) Validate() error {
	if err := c.HTTP.Validate(); err != nil {
		return fmt.Errorf("http config invalid: %w", err)
	}

	if err := c.Lynx.Validate(); err != nil {
		return fmt.Errorf("lynx config invalid: %w", err)
	}

	return nil
}

// Validate checks the API credentials, base URL, and MQTT configuration.
func (c *LynxConfig) Validate() error {
	if c.APIKey == "" {
		return fmt.Errorf("api_key can not be empty")
	}

	apiBase, err := url.ParseRequestURI(c.APIBase)
	if err != nil || apiBase.Scheme == "" || apiBase.Host == "" {
		return fmt.Errorf("api_base must be an absolute URL")
	}

	switch apiBase.Scheme {
	case "http", "https":
	default:
		return fmt.Errorf("api_base must use HTTP or HTTPS")
	}

	if err := c.MQTT.Validate(); err != nil {
		return fmt.Errorf("MQTT config invalid: %w", err)
	}

	return nil
}

// Validate checks the broker URL scheme and MQTT client ID.
func (c *MQTTConfig) Validate() error {
	broker, err := url.ParseRequestURI(c.Broker)
	if err != nil || broker.Host == "" {
		return fmt.Errorf("broker must be an absolute MQTT broker URL")
	}

	switch broker.Scheme {
	case "tcp", "mqtt", "ssl", "tls", "mqtts", "mqtt+ssl", "tcps", "ws", "wss":
	default:
		return fmt.Errorf("broker must use a supported MQTT or WebSocket scheme")
	}

	if strings.TrimSpace(c.ClientID) == "" || strings.ContainsRune(c.ClientID, '\x00') {
		return fmt.Errorf("client_id must be nonempty and contain no null characters")
	}

	return nil
}

// Validate checks the listening address and timeout constraints.
func (c *HTTPConfig) Validate() error {
	if c.Listen == "" {
		return fmt.Errorf("listen can not be empty")
	}

	if c.ReadTimeout <= 0 {
		return fmt.Errorf("read_timeout must be greater than zero")
	}

	if c.ReadHeaderTimeout <= 0 {
		return fmt.Errorf("read_header_timeout must be greater than zero")
	}

	if c.WriteTimeout <= 0 {
		return fmt.Errorf("write_timeout must be greater than zero")
	}

	if c.WriteTimeout < LynxRequestTimeout {
		return fmt.Errorf("write_timeout must be at least %s to support Lynx requests", LynxRequestTimeout)
	}

	if c.IdleTimeout <= 0 {
		return fmt.Errorf("idle_timeout must be greater than zero")
	}

	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown_timeout must be greater than zero")
	}

	return nil
}

// Load reads and validates a configuration file. An empty path uses the default search locations.
func Load(filePath string) (*Config, error) {
	v := viper.New()

	if filePath != "" {
		v.SetConfigFile(filePath)
	} else {
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/iot-open")
		v.SetConfigName("lynx-mcp")
		v.SetConfigType("yaml")
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("could not load config: %w", err)
	}

	slog.Info("Using config file", "file", v.ConfigFileUsed())

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("could not unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}
