package service

import "lynx-mcp/internal/config"

// Service provides Lynx object queries and permission-controlled device writes.
type Service struct {
	V3Client    v3Client
	V2Client    v2Client
	MQTTClient  mqttPublisher
	Permissions config.PermissionsConfig
}
