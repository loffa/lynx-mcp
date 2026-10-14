package service

import (
	"encoding/json"

	"github.com/IoTOpen/go-lynx"
)

//go:generate go tool moq -out v3_client_mock_test.go . v3Client
type v3Client interface {
	Search(opts *lynx.SearchOptions) (*lynx.SearchResult, error)
	Log(installationID int64, opts *lynx.LogOptionsV3) (*lynx.V3Log, error)
}

//go:generate go tool moq -out mqtt_publisher_mock_test.go . mqttPublisher
type mqttPublisher interface {
	Publish(topic string, payload any, qos byte) error
}

//go:generate go tool moq -out v2_client_mock_test.go . v2Client
type v2Client interface {
	GetInstallationRow(installationID int64) (*lynx.InstallationRow, error)
	GetDevice(installationID, deviceID int64) (*lynx.Device, error)
	GetDevices(installationID int64, filter lynx.Filter) (lynx.DeviceList, error)
	GetFunction(installationID, functionID int64) (*lynx.Function, error)
	GetFunctions(installationID int64, filter lynx.Filter) (lynx.FunctionList, error)
	GetSchedules(installationID int64, executor string) ([]*lynx.Schedule, error)
	GetConfiguredEdgeApps(installationID int64) ([]*lynx.EdgeAppConfig, error)
	GetEdgeAppInstance(installationID, instanceID int64) (*lynx.EdgeAppConfig, error)
	GetEdgeApp(appID int64) (*lynx.EdgeApp, error)
	GetEdgeAppConfigOptions(appID int64, version string) (json.RawMessage, error)
	Status(installationID int64, topicFilter []string) (lynx.Status, error)
}
