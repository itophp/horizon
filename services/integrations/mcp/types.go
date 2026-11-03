// Package mcp holds the domain types and the capability definitions of the
// mcp module. Types are owned by the domain, not by the transport layer.
package mcp

type AdminRevokePayload struct {
	Password string `json:"password"`
	ID       int64  `json:"id"`
}
type AuditPayload struct {
}
type CapabilitiesPayload struct {
	DepartmentID *int64 `json:"departmentId"`
}
type CreatePayload struct {
	Password     string   `json:"password"`
	Name         string   `json:"name"`
	DepartmentID int64    `json:"departmentId"`
	Days         int      `json:"days"`
	Tools        []string `json:"tools"`
}
type ListPayload struct {
}
type MCPAudit struct {
	ID           int64  `json:"id"`
	TokenID      int64  `json:"tokenId"`
	DepartmentID int64  `json:"departmentId"`
	Tool         string `json:"tool"`
	ResultCode   string `json:"resultCode"`
	DurationMs   int64  `json:"durationMs"`
	RequestID    string `json:"requestId"`
	CreatedAt    string `json:"createdAt"`
}
type MCPToken struct {
	ID           int64    `json:"id"`
	DepartmentID int64    `json:"departmentId"`
	Name         string   `json:"name"`
	Prefix       string   `json:"prefix"`
	Tools        []string `json:"tools"`
	CreatedAt    string   `json:"createdAt"`
	ExpiresAt    string   `json:"expiresAt"`
	RevokedAt    string   `json:"revokedAt"`
	LastUsedAt   string   `json:"lastUsedAt"`
	RevokedBy    int64    `json:"revokedBy"`
}
type MCPTokenCreated struct {
	Token  *MCPToken `json:"token"`
	Secret string    `json:"secret"`
}
type RevokePayload struct {
	Password string `json:"password"`
	ID       int64  `json:"id"`
}
type RotatePayload struct {
	Password string `json:"password"`
	ID       int64  `json:"id"`
}
