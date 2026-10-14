package service

import (
	"context"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	lynx "github.com/IoTOpen/go-lynx"

	"lynx-mcp/internal/domain"
)

// ControlFunction validates all requested writes before submitting them to MQTT.
// A submission failure returns the partial result alongside the error.
func (s *Service) ControlFunction(ctx context.Context, installationID, functionID int64, writes []domain.ControlWrite) (*domain.ControlFunctionResult, error) {
	if !s.Permissions.AllowControl {
		return nil, fmt.Errorf("control is disabled by permissions.allow_control")
	}

	if s.MQTTClient == nil {
		return nil, fmt.Errorf("MQTT publisher is not configured")
	}

	if installationID <= 0 || functionID <= 0 {
		return nil, fmt.Errorf("installation_id and function_id must be greater than zero")
	}

	if len(writes) == 0 {
		return nil, fmt.Errorf("writes cannot be empty")
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	object, err := s.GetObject(domain.ObjectTypeFunction, functionID, installationID, false)
	if err != nil {
		return nil, err
	}

	result := &domain.ControlFunctionResult{
		Installation: object.Installation,
		Function:     object.Function,
		Writes:       make([]domain.ControlWriteResult, len(writes)),
	}

	seenTopics := make(map[string]struct{}, len(writes))
	for i, write := range writes {
		topic, err := resolveControlTopic(object.Function.Meta, write.TopicKey, object.Installation.ClientID)
		if err != nil {
			return nil, fmt.Errorf("writes[%d]: %w", i, err)
		}

		if _, ok := seenTopics[topic]; ok {
			return nil, fmt.Errorf("writes[%d]: duplicate destination topic %q", i, topic)
		}

		seenTopics[topic] = struct{}{}

		validation, err := validateControlValue(object.Function.Meta, write.TopicKey, write.Value)
		if err != nil {
			return nil, fmt.Errorf("writes[%d] (%s): %w", i, write.TopicKey, err)
		}

		result.Writes[i] = domain.ControlWriteResult{
			ControlWrite: write,
			Topic:        topic,
			Status:       "not_attempted",
			Validation:   validation,
		}
	}

	// All metadata and values are validated before the first side effect.
	for i := range result.Writes {
		write := &result.Writes[i]
		if err := ctx.Err(); err != nil {
			write.Error = err.Error()

			return result, fmt.Errorf("control interrupted before writes[%d]: %w", i, err)
		}

		if err := s.MQTTClient.Publish(write.Topic, lynx.Message{Value: write.Value}, 1); err != nil {
			write.Status = "failed"
			write.Error = err.Error()

			return result, fmt.Errorf("publish writes[%d] to %q: %w; delivery may be uncertain", i, write.Topic, err)
		}

		// The helper's success is submission, not confirmation of device state.
		write.Status = "submitted"
	}

	result.Complete = true

	return result, nil
}

func resolveControlTopic(meta lynx.Meta, topicKey string, clientID int64) (string, error) {
	if topicKey != "topic_write" && (!strings.HasPrefix(topicKey, "topic_write_") || topicKey == "topic_write_") {
		return "", fmt.Errorf("topic_key must be topic_write or a topic_write_* metadata key")
	}

	topic := meta[topicKey]
	if strings.TrimSpace(topic) == "" || strings.ContainsAny(topic, "+#\x00") || !utf8.ValidString(topic) {
		return "", fmt.Errorf("%s must contain a nonempty MQTT topic without wildcards or null characters", topicKey)
	}

	if clientID <= 0 {
		return "", fmt.Errorf("installation has no valid MQTT client ID")
	}

	prefix := strconv.FormatInt(clientID, 10)
	first, rest, hasSlash := strings.Cut(topic, "/")

	if first == "" {
		return "", fmt.Errorf("%s must be relative to the installation or prefixed with its client ID", topicKey)
	}

	if hasSlash && strings.Trim(first, "0123456789") == "" {
		if first != prefix {
			return "", fmt.Errorf("%s targets client ID %q instead of installation client ID %s", topicKey, first, prefix)
		}

		if rest == "" {
			return "", fmt.Errorf("%s contains no topic after the MQTT client ID", topicKey)
		}
	} else {
		topic = prefix + "/" + topic
	}

	if len(topic) > 65_535 {
		return "", fmt.Errorf("%s exceeds the MQTT topic length limit", topicKey)
	}

	return topic, nil
}

func validateControlValue(meta lynx.Meta, topicKey string, value float64) (domain.ControlValueValidation, error) {
	validation := domain.ControlValueValidation{Mode: "unconstrained"}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return validation, fmt.Errorf("value must be a finite number")
	}

	channel := strings.TrimPrefix(topicKey, "topic_write_")
	if topicKey == "topic_write" {
		channel = ""
	}

	var err error

	validation.Min, validation.MinKey, err = controlBound(meta, "min", channel)
	if err != nil {
		return validation, err
	}

	validation.Max, validation.MaxKey, err = controlBound(meta, "max", channel)
	if err != nil {
		return validation, err
	}

	if validation.Min != nil && validation.Max != nil && *validation.Min > *validation.Max {
		return validation, fmt.Errorf("%s must not exceed %s", validation.MinKey, validation.MaxKey)
	}

	statePrefix := "state_"

	if channel != "" {
		channelPrefix := "state_" + channel + "_"

		for key := range meta {
			if strings.HasPrefix(key, channelPrefix) && key != channelPrefix {
				statePrefix = channelPrefix

				break
			}
		}
	}

	states := make(map[string]float64)

	for _, key := range slices.Sorted(maps.Keys(meta)) {
		if !strings.HasPrefix(key, statePrefix) || key == statePrefix {
			continue
		}

		state, err := parseControlNumber(meta, key)
		if err != nil {
			return validation, err
		}

		states[key] = state
		if state == value {
			validation.StateKeys = append(validation.StateKeys, key)
		}
	}

	if len(validation.StateKeys) > 0 {
		validation.Mode = "state"

		return validation, nil
	}

	if validation.Min != nil || validation.Max != nil {
		validation.Mode = "range"

		if validation.Min != nil && value < *validation.Min {
			return validation, fmt.Errorf("value %g is below %s=%g and matches no declared state", value, validation.MinKey, *validation.Min)
		}

		if validation.Max != nil && value > *validation.Max {
			return validation, fmt.Errorf("value %g exceeds %s=%g and matches no declared state", value, validation.MaxKey, *validation.Max)
		}

		return validation, nil
	}

	if len(states) > 0 {
		return validation, fmt.Errorf("value %g must match one of the declared states: %v", value, states)
	}

	return validation, nil
}

func controlBound(meta lynx.Meta, bound, channel string) (*float64, string, error) {
	key := bound
	if channel != "" {
		if _, ok := meta[bound+"_"+channel]; ok {
			key = bound + "_" + channel
		}
	}

	if _, ok := meta[key]; !ok {
		return nil, "", nil
	}

	value, err := parseControlNumber(meta, key)
	if err != nil {
		return nil, "", err
	}

	return &value, key, nil
}

func parseControlNumber(meta lynx.Meta, key string) (float64, error) {
	value, err := strconv.ParseFloat(strings.TrimSpace(meta[key]), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("metadata %s must contain a finite number, got %q", key, meta[key])
	}

	return value, nil
}
