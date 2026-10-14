package httpserver

import (
	"context"

	"github.com/IoTOpen/go-lynx"

	"lynx-mcp/internal/domain"
	"lynx-mcp/internal/service"
)

//go:generate go tool moq -out service_mock_test.go . mcpService
type mcpService interface {
	SearchObjects(installationID int64, options lynx.SearchOptions) (*domain.SearchObjectsResult, error)
	GetObject(objectType string, objectID, installationID int64, includeRelated bool) (*domain.GetObjectResult, error)
	ListSchedules(installationID int64, options service.ScheduleOptions) (*domain.ListSchedulesResult, error)
	ListAutomations(installationID int64, options service.AutomationOptions) (*domain.ListAutomationsResult, error)
	ControlFunction(ctx context.Context, installationID, functionID int64, writes []domain.ControlWrite) (*domain.ControlFunctionResult, error)
	GetHistory(installationID int64, functionIDs []int64, options lynx.LogOptionsV3) (*lynx.V3Log, error)
	GetCurrentValues(installationID int64, functionIDs []int64) (*domain.CurrentValuesResult, error)
}
