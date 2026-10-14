package domain

import "github.com/IoTOpen/go-lynx"

// SearchObjectsResult enriches search results with model types and installations.
type SearchObjectsResult struct {
	lynx.SearchResult

	Results []SearchObjectEntry `json:"results"`
}

// SearchObjectEntry describes a matching object with its model type and installation.
type SearchObjectEntry struct {
	lynx.SearchEntry

	// ModelType is the DeviceX or FunctionX type; installations use an empty string.
	ModelType    string                `json:"model_type"`
	Installation *lynx.InstallationRow `json:"installation"`
}
