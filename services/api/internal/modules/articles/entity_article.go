package articles

import "baseline/services/api/internal/platform/capability/contracts"

func Entities() []contracts.Entity { return []contracts.Entity{articleEntity()} }
func articleEntity() contracts.Entity {
	return contracts.Entity{Name: "article", Domain: "articles", Table: "articles", Key: "id", Attributes: []contracts.Attribute{
		{Name: "id", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "tenantId", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "createdBy", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "title", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true},
		{Name: "body", Kind: contracts.AttributeBusiness, Type: contracts.TypeString, Required: true, Sensitive: true},
		{Name: "revision", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "reviewCode", Kind: contracts.AttributeSystem, Type: contracts.TypeString, Required: true},
		{Name: "reviewedBy", Kind: contracts.AttributeSystem, Type: contracts.TypeInteger, Required: true},
		{Name: "createdAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		{Name: "updatedAt", Kind: contracts.AttributeSystem, Type: contracts.TypeTimestamp, Required: true},
		{Name: "status", Kind: contracts.AttributeState, Type: contracts.TypeEnum, Required: true, Enum: []string{"draft", "pending_ai", "pending_review", "published", "rejected"}}}, States: []contracts.State{{Name: "draft"}, {Name: "pending_ai"}, {Name: "pending_review"}, {Name: "published"}, {Name: "rejected"}}, Transitions: []contracts.Transition{
		{From: []string{}, To: "draft", Capability: "article.create"},
		{From: []string{"draft", "rejected"}, To: "draft", Capability: "article.update"},
		{From: []string{"draft"}, To: "pending_ai", Capability: "article.submit"},
		{From: []string{"pending_ai"}, To: "pending_review", Capability: "article.apply"},
		{From: []string{"pending_ai"}, To: "published", Capability: "article.apply"},
		{From: []string{"pending_review"}, To: "published", Capability: "article.decide"},
		{From: []string{"pending_review"}, To: "rejected", Capability: "article.decide"},
	}}
}
