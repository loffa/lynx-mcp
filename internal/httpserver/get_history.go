package httpserver

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	lynx "github.com/IoTOpen/go-lynx"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type getHistoryInput struct {
	InstallationID      int64   `json:"installation_id" jsonschema:"Lynx installation ID containing the topics."`
	FunctionIDs         []int64 `json:"function_ids,omitempty" jsonschema:"Optional Lynx function IDs whose configured topics to retrieve. Omit or provide an empty list to retrieve all installation topics."`
	From                string  `json:"from,omitempty" jsonschema:"Optional inclusive RFC 3339 start timestamp. Defaults to 24 hours before the end time."`
	To                  string  `json:"to,omitempty" jsonschema:"Optional inclusive RFC 3339 end timestamp. Defaults to now."`
	Limit               int64   `json:"limit,omitempty" jsonschema:"Optional maximum number of entries, from 1 to 10000. Defaults to 500."`
	AggregationMethod   string  `json:"aggregation_method,omitempty" jsonschema:"Optional aggregation: count, first, last, avg, mean, spread, max, min, sum, stddev, median, percentile(X), or integral(X)."`
	AggregationInterval string  `json:"aggregation_interval,omitempty" jsonschema:"Optional positive aggregation interval as a Go duration, for example 1h or 10m. Lynx selects an interval when omitted."`
}

func (i getHistoryInput) logOptions() (lynx.LogOptionsV3, error) {
	options := lynx.LogOptionsV3{
		Limit:      i.Limit,
		AggrMethod: i.AggregationMethod,
	}

	if i.From != "" {
		from, err := time.Parse(time.RFC3339, i.From)
		if err != nil {
			return lynx.LogOptionsV3{}, fmt.Errorf("parse from timestamp: %w", err)
		}

		options.From = from
	}

	if i.To != "" {
		to, err := time.Parse(time.RFC3339, i.To)
		if err != nil {
			return lynx.LogOptionsV3{}, fmt.Errorf("parse to timestamp: %w", err)
		}

		options.To = to
	}

	if !isAggregationMethod(i.AggregationMethod) {
		return lynx.LogOptionsV3{}, fmt.Errorf("unsupported aggregation_method %q", i.AggregationMethod)
	}

	if i.AggregationInterval != "" {
		if i.AggregationMethod == "" {
			return lynx.LogOptionsV3{}, fmt.Errorf("aggregation_interval requires aggregation_method")
		}

		interval, err := time.ParseDuration(i.AggregationInterval)
		if err != nil || interval <= 0 {
			return lynx.LogOptionsV3{}, fmt.Errorf("parse aggregation_interval as a positive duration: %q", i.AggregationInterval)
		}

		options.AggrInterval = interval
	}

	return options, nil
}

func isAggregationMethod(method string) bool {
	if method == "" {
		return true
	}

	switch method {
	case "count", "first", "last", "avg", "mean", "spread", "max", "min", "sum", "stddev", "median":
		return true
	}

	for _, name := range []string{"percentile", "integral"} {
		value, ok := strings.CutPrefix(method, name+"(")
		if !ok || !strings.HasSuffix(value, ")") {
			continue
		}

		valueNumber, err := strconv.ParseFloat(strings.TrimSuffix(value, ")"), 64)

		return err == nil && !math.IsNaN(valueNumber) && !math.IsInf(valueNumber, 0)
	}

	return false
}

func (s *Server) newGetHistoryTool() (*mcp.Tool, mcp.ToolHandlerFor[getHistoryInput, lynx.V3Log]) {
	return &mcp.Tool{
		Name:        "iot.get_history",
		Title:       "Get function value history",
		Description: "Get historical values for selected Lynx function topics, or all installation topics when function_ids is empty. Defaults to the last 24 hours and 500 newest entries.",
		Annotations: &mcp.ToolAnnotations{
			ReadOnlyHint:   true,
			IdempotentHint: true,
		},
	}, func(_ context.Context, _ *mcp.CallToolRequest, input getHistoryInput) (*mcp.CallToolResult, lynx.V3Log, error) {
		options, err := input.logOptions()
		if err != nil {
			return nil, lynx.V3Log{}, err
		}

		output, err := s.Service.GetHistory(input.InstallationID, input.FunctionIDs, options)
		if err != nil {
			return nil, lynx.V3Log{}, err
		}

		return nil, *output, nil
	}
}
