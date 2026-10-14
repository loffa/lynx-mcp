package service

import (
	"fmt"

	"github.com/IoTOpen/go-lynx"
)

// GetHistory retrieves history for the requested functions, or the installation when no IDs are supplied.
func (s *Service) GetHistory(installationID int64, functionIDs []int64, options lynx.LogOptionsV3) (*lynx.V3Log, error) {
	if installationID <= 0 {
		return nil, fmt.Errorf("installation_id must be greater than zero")
	}

	if options.Limit < 0 || options.Limit > 10_000 {
		return nil, fmt.Errorf("limit must be between 1 and %d", 10_000)
	}

	if len(functionIDs) > 0 {
		functions, err := s.loadFunctionTopics(installationID, functionIDs)
		if err != nil {
			return nil, err
		}

		options.TopicFilter = functions.uniqueTopics()
	}

	log, err := s.V3Client.Log(installationID, &options)
	if err != nil {
		return nil, fmt.Errorf("load history in installation %d: %w", installationID, err)
	}

	return log, nil
}
