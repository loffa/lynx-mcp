package service

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	lynx "github.com/IoTOpen/go-lynx"

	"lynx-mcp/internal/domain"
)

type functionTopics struct {
	function *lynx.Function
	topics   map[string]string
}

type functionTopicList []functionTopics

func (s *Service) loadFunctionTopics(installationID int64, functionIDs []int64) (functionTopicList, error) {
	functionIDs, err := uniqueFunctionIDs(functionIDs)
	if err != nil {
		return nil, err
	}

	functions, err := s.V2Client.GetFunctions(installationID, lynx.Filter{})
	if err != nil {
		return nil, fmt.Errorf("load functions in installation %d: %w", installationID, err)
	}

	functionsByID := functions.MapByID()

	selected := make(functionTopicList, len(functionIDs))
	for i, functionID := range functionIDs {
		function, ok := functionsByID[functionID]
		if !ok {
			return nil, fmt.Errorf("function %d is not available in installation %d", functionID, installationID)
		}

		topics := allFunctionTopics(function)
		if len(topics) == 0 {
			return nil, fmt.Errorf("function %d in installation %d has no configured topics", functionID, installationID)
		}

		selected[i] = functionTopics{function: function, topics: topics}
	}

	return selected, nil
}

func (functions functionTopicList) uniqueTopics() []string {
	unique := make(map[string]struct{})

	for _, function := range functions {
		for topic := range maps.Values(function.topics) {
			unique[topic] = struct{}{}
		}
	}

	return slices.Sorted(maps.Keys(unique))
}

func uniqueFunctionIDs(functionIDs []int64) ([]int64, error) {
	if len(functionIDs) == 0 {
		return nil, fmt.Errorf("function_ids cannot be empty")
	}

	seen := make(map[int64]struct{}, len(functionIDs))

	unique := make([]int64, 0, len(functionIDs))
	for _, functionID := range functionIDs {
		if functionID <= 0 {
			return nil, fmt.Errorf("function_ids must contain only values greater than zero")
		}

		if _, ok := seen[functionID]; ok {
			continue
		}

		seen[functionID] = struct{}{}
		unique = append(unique, functionID)
	}

	return unique, nil
}

func allFunctionTopics(function *lynx.Function) map[string]string {
	topics := make(map[string]string)

	for metaKey, topic := range function.Meta {
		key, ok := strings.CutPrefix(metaKey, "topic_")
		if !ok || key == "" || topic == "" {
			continue
		}

		topics[key] = topic
	}

	return topics
}

func summarizeFunction(function *lynx.Function) domain.FunctionSummary {
	return domain.FunctionSummary{
		ID:        function.ID,
		Name:      function.Meta["name"],
		ModelType: function.Type,
	}
}
