package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/IoTOpen/go-lynx"
	"github.com/spf13/viper"

	"lynx-mcp/internal/config"
	"lynx-mcp/internal/httpserver"
	"lynx-mcp/internal/service"
)

func main() {
	var configPath string

	flag.StringVar(&configPath, "config", "", "Path to the config file (default: search /etc/iot-open and the current directory)")
	flag.StringVar(&configPath, "c", "", "Shorthand for -config")
	flag.Parse()

	if err := run(configPath); err != nil {
		slog.Error("Lynx MCP stopped", "error", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mqttConfig := viper.New()
	mqttConfig.Set("broker", cfg.Lynx.MQTT.Broker)
	mqttConfig.Set("client_id", cfg.Lynx.MQTT.ClientID)
	mqttConfig.Set("connection_log", true)

	lynxClient := lynx.NewClient(&lynx.Options{
		Authenticator: lynx.AuthAPIKey{Key: cfg.Lynx.APIKey},
		APIBase:       cfg.Lynx.APIBase,
		MqttOptions:   lynx.NewMqttOptions(mqttConfig, nil, nil),
		HTTPClient: &http.Client{
			Timeout: config.LynxRequestTimeout,
		},
	})

	defer lynxClient.MQTTDisconnect()

	if err := lynxClient.MQTTConnect(); err != nil {
		return fmt.Errorf("connect to Lynx MQTT: %w", err)
	}

	var (
		mcpService = &service.Service{
			V3Client:    lynxClient.V3(),
			V2Client:    lynxClient,
			MQTTClient:  lynxClient,
			Permissions: cfg.Permissions,
		}
		server = httpserver.Server{
			Service: mcpService,
		}
	)

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Open(ctx, cfg.HTTP)
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		return <-serverErr
	}
}
