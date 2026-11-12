package harness

import (
	"context"
	"encoding/json"
	aicontext "horizon/foundation/ai/context"
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
	"strings"
)

func (s *Service) resolveSemantic(ctx context.Context, page aicontext.Page) (*ar.SemanticContext, error) {
	if page.ApplicationID == "" {
		return nil, nil
	}
	input := contracts.Input{"applicationId": page.ApplicationID, "pageId": page.PageID, "metadataVersion": page.MetadataVersion}
	if page.EntityID > 0 {
		input["recordId"] = page.EntityID
	}
	if len(page.Selection) > 0 {
		ids := []any{}
		for _, id := range page.Selection {
			ids = append(ids, id)
		}
		input["selection"] = ids
	}
	result, err := s.gateway.Invoke(ctx, gateway.Request{Name: "application.runtime.context.resolve", Version: "1.0.0", Origin: contracts.OriginAgent, Input: input})
	if err != nil {
		return nil, err
	}
	if result.Status != contracts.StatusSucceeded {
		return nil, contracts.Problem(result.ErrorCode)
	}
	raw, err := json.Marshal(result.Data)
	if err != nil {
		return nil, err
	}
	var semantic ar.SemanticContext
	if err = json.Unmarshal(raw, &semantic); err != nil {
		return nil, err
	}
	return &semantic, nil
}

func (s *Service) toolProfile(ctx context.Context) ([]string, error) {
	keys := []string{"example.record.list@1.0.0", "example.record.get@1.0.0", "example.record.create@1.0.0", "example.record.archive@1.0.0", "knowledge.search@1.0.0"}
	if s.Profile != nil {
		extra, err := s.Profile(ctx)
		if err != nil {
			return nil, err
		}
		keys = append(keys, extra...)
	}
	// Test and reduced deployments may omit optional core modules; never invent a handler.
	out := []string{}
	for _, key := range keys {
		name, version, _ := strings.Cut(key, "@")
		if _, ok := s.registry.Resolve(name, version); ok {
			out = append(out, key)
		}
	}
	return out, nil
}

func (s *Service) authorityStamp(ctx context.Context) (string, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return "", err
	}
	fresh, err := s.Core.Identity.PrincipalForTask(ctx, p.TenantID, p.Account.ID, p.Account.DepartmentID)
	if err != nil {
		return "", err
	}
	if p.TokenHash != "" {
		session, e := dbgen.New(s.Core.Pool).IAMSession(ctx, p.TokenHash)
		if e != nil || session.ActiveDepartmentID != p.Account.DepartmentID {
			return "", contracts.Problem("unauthenticated")
		}
	}
	rows, err := dbgen.New(s.Core.Pool).RuntimeScopeStamp(ctx, p.TenantID)
	if err != nil {
		return "", err
	}
	return contracts.Digest(contracts.Input{"principal": identity.AuthorizationVersion(fresh), "applications": rows}), nil
}
