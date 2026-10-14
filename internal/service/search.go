package service

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	lynx "github.com/IoTOpen/go-lynx"

	"lynx-mcp/internal/domain"
)

// SearchObjects searches accessible objects, or lists objects within a supplied installation.
func (s *Service) SearchObjects(installationID int64, options lynx.SearchOptions) (*domain.SearchObjectsResult, error) {
	options.Query = strings.TrimSpace(options.Query)

	if err := validateSearchTypes(options.Types); err != nil {
		return nil, err
	}

	limit, err := normalizePagination(options.Limit, options.Offset)
	if err != nil {
		return nil, err
	}

	options.Limit = limit

	if installationID == 0 {
		if options.Query == "" {
			return nil, fmt.Errorf("query cannot be empty")
		}

		return s.searchGlobalObjects(options)
	}

	return s.searchInstallationObjects(installationID, options)
}

func (s *Service) searchGlobalObjects(options lynx.SearchOptions) (*domain.SearchObjectsResult, error) {
	result, err := s.V3Client.Search(&options)
	if err != nil {
		return nil, fmt.Errorf("search Lynx objects: %w", err)
	}

	output := &domain.SearchObjectsResult{
		SearchResult: *result,
		Results:      make([]domain.SearchObjectEntry, len(result.Results)),
	}

	installations := make(map[int64]*lynx.InstallationRow)

	type modelKey struct {
		installationID int64
		entityType     string
	}

	modelTypes := make(map[modelKey]map[int64]string)

	for i, entry := range result.Results {
		installationID, err := getInstallationID(entry)
		if err != nil {
			return nil, err
		}

		installation, ok := installations[installationID]
		if !ok {
			installation, err = s.V2Client.GetInstallationRow(installationID)
			if err != nil {
				return nil, fmt.Errorf("load installation %d for search result %q %d: %w", installationID, entry.Type, entry.ID, err)
			}

			installations[installationID] = installation
		}

		var modelType string

		if entry.Type != domain.ObjectTypeInstallation {
			key := modelKey{installationID: installationID, entityType: entry.Type}

			types, ok := modelTypes[key]
			if !ok {
				types, err = s.loadSearchModelTypes(installationID, entry.Type)
				if err != nil {
					return nil, err
				}

				modelTypes[key] = types
			}

			modelType, ok = types[entry.ID]
			if !ok {
				return nil, fmt.Errorf("%s %d from search is not available in installation %d", entry.Type, entry.ID, installationID)
			}
		}

		output.Results[i] = domain.SearchObjectEntry{
			SearchEntry:  entry,
			ModelType:    modelType,
			Installation: installation,
		}
	}

	return output, nil
}

func (s *Service) loadSearchModelTypes(installationID int64, entityType string) (map[int64]string, error) {
	modelTypes := make(map[int64]string)

	switch entityType {
	case domain.ObjectTypeDevice:
		devices, err := s.V2Client.GetDevices(installationID, lynx.Filter{})
		if err != nil {
			return nil, fmt.Errorf("load device model types in installation %d for search: %w", installationID, err)
		}

		for _, device := range devices {
			modelTypes[device.ID] = device.Type
		}
	case domain.ObjectTypeFunction:
		functions, err := s.V2Client.GetFunctions(installationID, lynx.Filter{})
		if err != nil {
			return nil, fmt.Errorf("load function model types in installation %d for search: %w", installationID, err)
		}

		for _, function := range functions {
			modelTypes[function.ID] = function.Type
		}
	default:
		return nil, fmt.Errorf("unsupported search result type %q", entityType)
	}

	return modelTypes, nil
}

func (s *Service) searchInstallationObjects(installationID int64, options lynx.SearchOptions) (*domain.SearchObjectsResult, error) {
	if installationID < 0 {
		return nil, fmt.Errorf("installation_id must be greater than zero")
	}

	if options.Query != "" && options.Query != "*" {
		return nil, fmt.Errorf("query must be omitted or '*' when installation_id is supplied")
	}

	if slices.Contains(options.Types, domain.ObjectTypeInstallation) {
		return nil, fmt.Errorf("types cannot include installation when installation_id is supplied")
	}

	installation, err := s.V2Client.GetInstallationRow(installationID)
	if err != nil {
		return nil, fmt.Errorf("load installation %d for scoped search: %w", installationID, err)
	}

	filter := lynx.Filter(options.Metadata)

	entries := make([]domain.SearchObjectEntry, 0)

	if len(options.Types) == 0 || slices.Contains(options.Types, domain.ObjectTypeDevice) {
		devices, err := s.V2Client.GetDevices(installationID, filter)
		if err != nil {
			return nil, fmt.Errorf("list devices in installation %d: %w", installationID, err)
		}

		for _, device := range devices {
			entries = append(entries, deviceSearchEntry(device))
		}
	}

	if len(options.Types) == 0 || slices.Contains(options.Types, domain.ObjectTypeFunction) {
		functions, err := s.V2Client.GetFunctions(installationID, filter)
		if err != nil {
			return nil, fmt.Errorf("list functions in installation %d: %w", installationID, err)
		}

		for _, function := range functions {
			entries = append(entries, functionSearchEntry(function))
		}
	}

	slices.SortFunc(entries, func(a, b domain.SearchObjectEntry) int {
		if result := strings.Compare(a.Type, b.Type); result != 0 {
			return result
		}

		return cmp.Compare(a.ID, b.ID)
	})

	total := len(entries)
	start, end := pageBounds(total, options.Limit, options.Offset)

	entries = entries[start:end]
	output := &domain.SearchObjectsResult{
		SearchResult: lynx.SearchResult{
			Total:   int64(total),
			Limit:   int64(options.Limit),
			Offset:  int64(options.Offset),
			Results: make([]lynx.SearchEntry, len(entries)),
		},
		Results: entries,
	}

	for i, entry := range entries {
		output.SearchResult.Results[i] = entry.SearchEntry
		output.Results[i].Installation = installation
	}

	return output, nil
}

func deviceSearchEntry(device *lynx.Device) domain.SearchObjectEntry {
	return domain.SearchObjectEntry{
		Type:           domain.ObjectTypeDevice,
		ID:             device.ID,
		InstallationID: device.InstallationID,
		Created:        device.Created,
		Updated:        device.Updated,
		Meta:           device.Meta,
		ModelType:      device.Type,
	}
}

func functionSearchEntry(function *lynx.Function) domain.SearchObjectEntry {
	return domain.SearchObjectEntry{
		Type:           domain.ObjectTypeFunction,
		ID:             function.ID,
		InstallationID: function.InstallationID,
		Created:        function.Created,
		Updated:        function.Updated,
		Meta:           function.Meta,
		ModelType:      function.Type,
	}
}

func getInstallationID(entry lynx.SearchEntry) (int64, error) {
	switch entry.Type {
	case domain.ObjectTypeInstallation:
		return entry.ID, nil
	case domain.ObjectTypeDevice, domain.ObjectTypeFunction:
		return entry.InstallationID, nil
	default:
		return 0, fmt.Errorf("unsupported search result type %q", entry.Type)
	}
}

func validateSearchTypes(types []string) error {
	for _, objectType := range types {
		switch objectType {
		case domain.ObjectTypeInstallation, domain.ObjectTypeDevice, domain.ObjectTypeFunction:
		default:
			return fmt.Errorf("unsupported object type %q; use installation, device, or function", objectType)
		}
	}

	return nil
}
