// Package files holds the domain types and the capability definitions of the
// files module. Types are owned by the domain, not by the transport layer.
package files

type ContentPayload struct {
	ID int64 `json:"id"`
}
type DriversPayload struct {
}
type FileContent struct {
	Name    string `json:"name"`
	Mime    string `json:"mime"`
	Content []byte `json:"content"`
}
type ListPayload struct {
}
type ManagedFile struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Mime           string `json:"mime"`
	Size           int64  `json:"size"`
	Purpose        string `json:"purpose"`
	CreatedAt      string `json:"createdAt"`
	OwnerID        int64  `json:"ownerId"`
	DepartmentID   int64  `json:"departmentId"`
	StorageVersion int64  `json:"storageVersion"`
	Link           string `json:"link"`
}
type RemovePayload struct {
	ID int64 `json:"id"`
}
type StorageDriver struct {
	Code       string   `json:"code"`
	Writable   bool     `json:"writable"`
	ConfigKeys []string `json:"configKeys"`
}
type UploadPayload struct {
	Name    string `json:"name"`
	Purpose string `json:"purpose"`
	Content []byte `json:"content"`
}

// EffectiveConfiguration is the public branding projection: only allowlisted
// display keys of a published deployment workspace.
type EffectiveConfiguration struct {
	Entries  []*ConfigEntry `json:"entries"`
	Revision string         `json:"revision"`
}

// ConfigEntry is one effective configuration value with its source scope.
type ConfigEntry struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	Source     string `json:"source"`
	Overridden bool   `json:"overridden"`
	Configured bool   `json:"configured"`
}
