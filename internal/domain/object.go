package domain

import "github.com/IoTOpen/go-lynx"

const (
	// ObjectTypeInstallation identifies a Lynx installation.
	ObjectTypeInstallation = "installation"
	// ObjectTypeDevice identifies a Lynx device.
	ObjectTypeDevice = "device"
	// ObjectTypeFunction identifies a Lynx function.
	ObjectTypeFunction = "function"
)

const (
	// InstallationIDProperty names the installation identifier in tool inputs.
	InstallationIDProperty = "installation_id"
	// LimitProperty names the maximum result count in paginated tool inputs.
	LimitProperty = "limit"
	// OffsetProperty names the number of results to skip in paginated tool inputs.
	OffsetProperty = "offset"
)

// GetObjectResult contains a Lynx object, its installation, and any requested relationships.
type GetObjectResult struct {
	Type         string                `json:"type"`
	ID           int64                 `json:"id"`
	Installation *lynx.InstallationRow `json:"installation"`
	Function     *lynx.Function        `json:"function,omitempty"`
	Device       *lynx.Device          `json:"device"`
	// A nil slice is omitted; a loaded relationship with no functions is [].
	Functions []FunctionSummary `json:"functions,omitzero"`
	Warnings  []ObjectWarning   `json:"warnings,omitempty"`
}

// FunctionSummary identifies a function without including its full metadata.
type FunctionSummary struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	ModelType string `json:"model_type"`
}

// ObjectWarning reports an optional enrichment failure without failing the operation.
type ObjectWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
