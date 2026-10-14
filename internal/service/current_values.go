package service

import (
	"fmt"
	"maps"
	"slices"

	lynx "github.com/IoTOpen/go-lynx"

	"lynx-mcp/internal/domain"
)

// GetCurrentValues retrieves the latest status for each configured topic on the requested functions.
func (s *Service) GetCurrentValues(installationID int64, functionIDs []int64) (*domain.CurrentValuesResult, error) {
	if installationID <= 0 {
		return nil, fmt.Errorf("installation_id must be greater than zero")
	}

	functions, err := s.loadFunctionTopics(installationID, functionIDs)
	if err != nil {
		return nil, err
	}

	status, err := s.V2Client.Status(installationID, functions.uniqueTopics())
	if err != nil {
		return nil, fmt.Errorf("load current values in installation %d: %w", installationID, err)
	}

	statuses := status.Map()
	result := &domain.CurrentValuesResult{
		Functions: make([]domain.CurrentValueResult, len(functions)),
	}

	for i, function := range functions {
		result.Functions[i] = function.currentValueResult(statuses)
	}

	return result, nil
}

func (f functionTopics) currentValueResult(statuses map[string]*lynx.LogEntry) domain.CurrentValueResult {
	result := domain.CurrentValueResult{
		Function: f.function,
		Topics:   make([]domain.CurrentValueTopic, len(f.topics)),
	}

	for i, key := range slices.Sorted(maps.Keys(f.topics)) {
		topic := f.topics[key]
		result.Topics[i] = domain.CurrentValueTopic{
			Key:    key,
			Topic:  topic,
			Status: statuses[topic],
		}
	}

	return result
}
