package gateway

import (
	"encoding/json"
	"slices"

	"horizon/foundation/kernel/approval"
	"horizon/foundation/kernel/capability/contracts"
)

type ApprovalCipher interface {
	Seal(tenant, scope, key, raw string) (string, error)
	Unseal(tenant, scope, key, sealed string) (string, error)
}

func (g *Gateway) approvalArguments(tenant, invocation string, def contracts.Definition, input contracts.Input) (json.RawMessage, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, contracts.Problem("invalid")
	}
	if !slices.Contains(def.Tags, contracts.TagSealedApprovalInput) {
		return raw, nil
	}
	if g.approvalCipher == nil {
		return nil, contracts.Problem("unavailable")
	}
	sealed, err := g.approvalCipher.Seal(tenant, "approval:"+invocation, def.Key(), string(raw))
	if err != nil {
		return nil, contracts.Problem("unavailable")
	}
	return json.Marshal(approval.SealedArguments{Ciphertext: sealed, Projection: def.Redact(input)})
}

func (g *Gateway) openApprovalArguments(card *approval.Approval, def contracts.Definition) (contracts.Input, error) {
	raw := card.Arguments
	sealed := slices.Contains(def.Tags, contracts.TagSealedApprovalInput)
	if sealed {
		var envelope approval.SealedArguments
		if g.approvalCipher == nil || json.Unmarshal(raw, &envelope) != nil || envelope.Ciphertext == "" {
			return nil, contracts.Problem("invalid")
		}
		opened, err := g.approvalCipher.Unseal(card.TenantID, "approval:"+card.InvocationID, def.Key(), envelope.Ciphertext)
		if err != nil {
			return nil, contracts.Problem("invalid")
		}
		raw = json.RawMessage(opened)
	}
	var input contracts.Input
	if err := json.Unmarshal(raw, &input); err != nil || (sealed && contracts.Digest(input) != card.Digest) {
		return nil, contracts.Problem("invalid")
	}
	return input, nil
}
