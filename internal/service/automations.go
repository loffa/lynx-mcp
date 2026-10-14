package service

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"

	lynx "github.com/IoTOpen/go-lynx"

	"lynx-mcp/internal/domain"
)

// AutomationOptions controls instance filtering, enrichment, and pagination.
type AutomationOptions struct {
	InstanceID           int64
	AppID                int64
	IncludeConfigOptions bool
	Limit                int
	Offset               int
}

// ListAutomations lists configured EdgeApp instances, sorted by ID, with optional enrichment.
func (s *Service) ListAutomations(installationID int64, options AutomationOptions) (*domain.ListAutomationsResult, error) {
	if installationID <= 0 {
		return nil, fmt.Errorf("installation_id must be greater than zero")
	}

	if options.InstanceID < 0 {
		return nil, fmt.Errorf("instance_id must be greater than zero when supplied")
	}

	if options.AppID < 0 {
		return nil, fmt.Errorf("app_id must be greater than zero when supplied")
	}

	limit, err := normalizePagination(options.Limit, options.Offset)
	if err != nil {
		return nil, err
	}

	options.Limit = limit

	installation, err := s.V2Client.GetInstallationRow(installationID)
	if err != nil {
		return nil, fmt.Errorf("load installation %d for automations: %w", installationID, err)
	}

	instances, err := s.loadAutomationInstances(installationID, options.InstanceID)
	if err != nil {
		return nil, err
	}

	selected := make([]*lynx.EdgeAppConfig, 0, len(instances))
	for _, instance := range instances {
		if options.AppID != 0 && instance.AppID != options.AppID {
			continue
		}

		selected = append(selected, instance)
	}

	slices.SortFunc(selected, func(a, b *lynx.EdgeAppConfig) int {
		return cmp.Compare(a.ID, b.ID)
	})

	total := len(selected)
	start, end := pageBounds(total, options.Limit, options.Offset)

	result := &domain.ListAutomationsResult{
		Installation: installation,
		Total:        total,
		Limit:        options.Limit,
		Offset:       options.Offset,
		Automations:  make([]domain.AutomationResult, 0, end-start),
	}

	// Cache failures too, so shared apps and versions are requested only once.
	type appResult struct {
		app map[string]any
		err error
	}

	type configKey struct {
		appID   int64
		version string
	}

	type configResult struct {
		config any
		err    error
	}

	apps := make(map[int64]appResult)
	configs := make(map[configKey]configResult)

	for _, instance := range selected[start:end] {
		entry := domain.AutomationResult{Instance: instance}

		app, ok := apps[instance.AppID]
		if !ok {
			app.app, app.err = s.loadAutomationApp(instance.AppID)
			apps[instance.AppID] = app
		}

		if app.err != nil {
			entry.Warnings = append(entry.Warnings, domain.ObjectWarning{
				Code:    "app_unavailable",
				Message: fmt.Sprintf("Could not load EdgeApp %d for instance %d: %v", instance.AppID, instance.ID, app.err),
			})
		} else {
			entry.App = app.app
		}

		if options.IncludeConfigOptions {
			key := configKey{appID: instance.AppID, version: instance.Version}

			config, ok := configs[key]
			if !ok {
				config.config, config.err = s.loadAutomationConfigOptions(instance.AppID, instance.Version)
				configs[key] = config
			}

			if config.err != nil {
				entry.Warnings = append(entry.Warnings, domain.ObjectWarning{
					Code:    "config_options_unavailable",
					Message: fmt.Sprintf("Could not load configuration options for EdgeApp %d version %q: %v", instance.AppID, instance.Version, config.err),
				})
			} else {
				entry.ConfigOptions = config.config
			}
		}

		result.Automations = append(result.Automations, entry)
	}

	return result, nil
}

func (s *Service) loadAutomationInstances(installationID, instanceID int64) ([]*lynx.EdgeAppConfig, error) {
	if instanceID != 0 {
		instance, err := s.V2Client.GetEdgeAppInstance(installationID, instanceID)
		if err != nil {
			return nil, fmt.Errorf("load EdgeApp instance %d in installation %d: %w", instanceID, installationID, err)
		}

		return []*lynx.EdgeAppConfig{instance}, nil
	}

	instances, err := s.V2Client.GetConfiguredEdgeApps(installationID)
	if err != nil {
		return nil, fmt.Errorf("load EdgeApp instances in installation %d: %w", installationID, err)
	}

	return instances, nil
}

func (s *Service) loadAutomationApp(appID int64) (map[string]any, error) {
	if appID <= 0 {
		return nil, fmt.Errorf("app ID must be greater than zero")
	}

	app, err := s.V2Client.GetEdgeApp(appID)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(app)
	if err != nil {
		return nil, fmt.Errorf("encode app details: %w", err)
	}

	// Preserve all native fields and integer precision while avoiding recursive
	// EdgeApp/Publisher types in the inferred MCP result schema.
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	var details map[string]any
	if err := decoder.Decode(&details); err != nil {
		return nil, fmt.Errorf("decode app details: %w", err)
	}

	return details, nil
}

func (s *Service) loadAutomationConfigOptions(appID int64, version string) (any, error) {
	if appID <= 0 || strings.TrimSpace(version) == "" {
		return nil, fmt.Errorf("a valid app ID and configured version are required")
	}

	// The pinned client interpolates the version into the query without escaping.
	config, err := s.V2Client.GetEdgeAppConfigOptions(appID, url.QueryEscape(version))
	if err != nil {
		return nil, err
	}

	if !json.Valid(config) || bytes.Equal(bytes.TrimSpace(config), []byte("null")) {
		return nil, fmt.Errorf("configuration options response must contain valid, non-null JSON")
	}

	// Decode JSON values so schema inference does not treat RawMessage as bytes.
	decoder := json.NewDecoder(bytes.NewReader(config))
	decoder.UseNumber()

	var options any
	if err := decoder.Decode(&options); err != nil {
		return nil, fmt.Errorf("decode configuration options: %w", err)
	}

	return options, nil
}
