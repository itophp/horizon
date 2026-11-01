package identity

import "baseline/services/api/internal/platform/capability/contracts"

// sessionEntity mirrors iam_sessions. Sessions are business objects because
// publishing a security setting clamps or expires them; they are never editable
// from a form, which is why every attribute is system or reference.
func sessionEntity() contracts.Entity {
	return contracts.Entity{
		Name:        "session",
		Domain:      "identity",
		Table:       "iam_sessions",
		Description: "A signed in browser session for one account in one department.",
		Key:         "tokenHash",
		Attributes: []contracts.Attribute{
			{Name: "tokenHash", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true,
				Sensitive: true, Description: "SHA-256 of the cookie token; the raw token never leaves the client."},
			{Name: "accountId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Required: true},
			{Name: "csrfToken", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true,
				Sensitive: true, Description: "Nonce required on unsafe methods; held in memory by the console."},
			{Name: "expiresAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "lastSeenAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
			{Name: "activeDepartmentId", Kind: contracts.AttributeReference, Type: contracts.TypeReference, Nullable: true,
				Description: "Department the session is currently acting in; falls back to the primary one."},
		},
		Relations: []contracts.Relation{
			{Name: "account", Target: "account", Kind: "one", Via: "account_id", Required: true},
		},
	}
}
