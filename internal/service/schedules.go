package service

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	lynx "github.com/IoTOpen/go-lynx"

	"lynx-mcp/internal/domain"
)

// ScheduleOptions controls schedule filtering and pagination.
type ScheduleOptions struct {
	FunctionIDs []int64
	Topics      []string
	Active      *bool
	Executor    string
	Limit       int
	Offset      int
}

// ListSchedules lists matching schedules, sorted by ID, and the functions sharing their topics.
func (s *Service) ListSchedules(installationID int64, options ScheduleOptions) (*domain.ListSchedulesResult, error) {
	if installationID <= 0 {
		return nil, fmt.Errorf("installation_id must be greater than zero")
	}

	limit, err := normalizePagination(options.Limit, options.Offset)
	if err != nil {
		return nil, err
	}

	options.Limit = limit

	if len(options.FunctionIDs) > 0 {
		var err error

		options.FunctionIDs, err = uniqueFunctionIDs(options.FunctionIDs)
		if err != nil {
			return nil, err
		}
	}

	for _, topic := range options.Topics {
		if topic == "" || strings.ContainsAny(topic, "+#\x00") {
			return nil, fmt.Errorf("topics must contain nonempty exact MQTT topics without wildcards or null characters")
		}
	}

	installation, err := s.V2Client.GetInstallationRow(installationID)
	if err != nil {
		return nil, fmt.Errorf("load installation %d for schedules: %w", installationID, err)
	}

	functions, err := s.V2Client.GetFunctions(installationID, lynx.Filter{})
	if err != nil {
		return nil, fmt.Errorf("load functions in installation %d for schedules: %w", installationID, err)
	}

	functionsByID := functions.MapByID()

	filterTopics := make(map[string]struct{})
	for _, topic := range options.Topics {
		filterTopics[normalizeScheduleTopic(topic, installation.ClientID)] = struct{}{}
	}

	for _, functionID := range options.FunctionIDs {
		function, ok := functionsByID[functionID]
		if !ok {
			return nil, fmt.Errorf("function %d is not available in installation %d", functionID, installationID)
		}

		for _, topic := range allFunctionTopics(function) {
			filterTopics[normalizeScheduleTopic(topic, installation.ClientID)] = struct{}{}
		}
	}

	hasTopicFilter := len(options.Topics) > 0 || len(options.FunctionIDs) > 0

	schedules, err := s.V2Client.GetSchedules(installationID, options.Executor)
	if err != nil {
		return nil, fmt.Errorf("load schedules in installation %d: %w", installationID, err)
	}

	selected := make([]*lynx.Schedule, 0, len(schedules))
	for _, schedule := range schedules {
		if options.Active != nil && schedule.Active != *options.Active {
			continue
		}

		if options.Executor != "" && schedule.Executor != options.Executor {
			continue
		}

		if hasTopicFilter {
			if _, ok := filterTopics[normalizeScheduleTopic(schedule.Topic, installation.ClientID)]; !ok {
				continue
			}
		}

		selected = append(selected, schedule)
	}

	slices.SortFunc(selected, func(a, b *lynx.Schedule) int {
		return cmp.Compare(a.ID, b.ID)
	})

	total := len(selected)
	start, end := pageBounds(total, options.Limit, options.Offset)

	result := &domain.ListSchedulesResult{
		Installation: installation,
		Total:        total,
		Limit:        options.Limit,
		Offset:       options.Offset,
		Schedules:    make([]domain.ScheduleResult, 0, end-start),
	}

	functionIndex := indexScheduleFunctions(functionsByID, installation.ClientID)
	for _, schedule := range selected[start:end] {
		matches := functionIndex[normalizeScheduleTopic(schedule.Topic, installation.ClientID)]

		if matches == nil {
			matches = make([]domain.ScheduleFunctionMatch, 0)
		}

		result.Schedules = append(result.Schedules, domain.ScheduleResult{
			Schedule:         schedule,
			MatchedFunctions: matches,
		})
	}

	return result, nil
}

func indexScheduleFunctions(functions map[int64]*lynx.Function, clientID int64) map[string][]domain.ScheduleFunctionMatch {
	index := make(map[string][]domain.ScheduleFunctionMatch)

	for _, functionID := range slices.Sorted(maps.Keys(functions)) {
		function := functions[functionID]
		topics := allFunctionTopics(function)
		keysByTopic := make(map[string][]string)

		for _, key := range slices.Sorted(maps.Keys(topics)) {
			topic := normalizeScheduleTopic(topics[key], clientID)
			keysByTopic[topic] = append(keysByTopic[topic], "topic_"+key)
		}

		for topic, keys := range keysByTopic {
			index[topic] = append(index[topic], domain.ScheduleFunctionMatch{
				FunctionSummary: summarizeFunction(function),
				TopicKeys:       keys,
			})
		}
	}

	return index
}

// Only the current installation's client ID can be treated as an MQTT prefix.
func normalizeScheduleTopic(topic string, clientID int64) string {
	if clientID <= 0 {
		return topic
	}

	return strings.TrimPrefix(topic, strconv.FormatInt(clientID, 10)+"/")
}
