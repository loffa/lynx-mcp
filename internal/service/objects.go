package service

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	lynx "github.com/IoTOpen/go-lynx"

	"lynx-mcp/internal/domain"
)

// GetObject loads an object and its installation, optionally enriching its immediate relationships.
func (s *Service) GetObject(objectType string, objectID, installationID int64, includeRelated bool) (*domain.GetObjectResult, error) {
	if err := validateSearchTypes([]string{objectType}); err != nil {
		return nil, err
	}

	if objectID <= 0 {
		return nil, fmt.Errorf("id must be greater than zero")
	}

	if objectType == domain.ObjectTypeInstallation {
		if installationID != 0 && installationID != objectID {
			return nil, fmt.Errorf("installation_id must be omitted or equal id for an installation")
		}

		installationID = objectID
	} else if installationID <= 0 {
		return nil, fmt.Errorf("installation_id must be greater than zero for a %s", objectType)
	}

	result := &domain.GetObjectResult{Type: objectType, ID: objectID}

	switch objectType {
	case domain.ObjectTypeFunction:
		function, err := s.V2Client.GetFunction(installationID, objectID)
		if err != nil {
			return nil, fmt.Errorf("load function %d in installation %d: %w", objectID, installationID, err)
		}

		result.Function = function
	case domain.ObjectTypeDevice:
		device, err := s.V2Client.GetDevice(installationID, objectID)
		if err != nil {
			return nil, fmt.Errorf("load device %d in installation %d: %w", objectID, installationID, err)
		}

		result.Device = device
	}

	installation, err := s.V2Client.GetInstallationRow(installationID)
	if err != nil {
		return nil, fmt.Errorf("load installation %d for %s %d: %w", installationID, objectType, objectID, err)
	}

	result.Installation = installation

	if includeRelated {
		switch objectType {
		case domain.ObjectTypeFunction:
			s.populateObjectDevice(result)
		case domain.ObjectTypeDevice:
			s.populateObjectFunctions(result)
		}
	}

	return result, nil
}

func (s *Service) populateObjectDevice(result *domain.GetObjectResult) {
	value := strings.TrimSpace(result.Function.Meta["device_id"])
	if value == "" {
		return
	}

	deviceID, err := strconv.ParseInt(value, 10, 64)
	if err != nil || deviceID <= 0 {
		result.Warnings = append(result.Warnings, domain.ObjectWarning{
			Code:    "invalid_device_id",
			Message: fmt.Sprintf("Function %d has an invalid device_id metadata value %q.", result.ID, value),
		})

		return
	}

	installationID := result.Installation.ID

	device, err := s.V2Client.GetDevice(installationID, deviceID)
	if err != nil {
		result.Warnings = append(result.Warnings, domain.ObjectWarning{
			Code:    "related_device_unavailable",
			Message: fmt.Sprintf("Could not load linked device %d in installation %d: %v", deviceID, installationID, err),
		})

		return
	}

	result.Device = device
}

func (s *Service) populateObjectFunctions(result *domain.GetObjectResult) {
	installationID := result.Installation.ID
	deviceID := strconv.FormatInt(result.ID, 10)

	functions, err := s.V2Client.GetFunctions(installationID, lynx.Filter{"device_id": deviceID})
	if err != nil {
		result.Warnings = append(result.Warnings, domain.ObjectWarning{
			Code:    "related_functions_unavailable",
			Message: fmt.Sprintf("Could not load functions linked to device %d in installation %d: %v", result.ID, installationID, err),
		})

		return
	}

	result.Functions = make([]domain.FunctionSummary, 0, len(functions))

	for _, function := range functions {
		result.Functions = append(result.Functions, summarizeFunction(function))
	}

	slices.SortFunc(result.Functions, func(a, b domain.FunctionSummary) int {
		return cmp.Compare(a.ID, b.ID)
	})
}
