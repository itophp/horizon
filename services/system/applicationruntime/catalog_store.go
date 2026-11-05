package applicationruntime

import (
	genidentity "baseline/services/api/gen/identity"
	"baseline/services/api/internal/repository/dbgen"
	ar "baseline/services/api/kernel/applicationruntime"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/gateway"
	"baseline/services/api/kernel/capability/registry"
	"baseline/services/api/system/identity"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
)

// publishedCatalog is the only effective-definition reader. Deployment data is
// not a fallback for a missing or unavailable tenant catalog.
func (m *Module) publishedCatalog(ctx context.Context) ([]ar.ApplicationV2, error) {
	if m.Pool == nil {
		return m.contributions(), nil
	} // declaration-only/offline composition
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	return m.publishedCatalogWith(ctx, dbgen.New(m.Pool), p.TenantID)
}
func (m *Module) publishedCatalogWith(ctx context.Context, q *dbgen.Queries, tenant string) ([]ar.ApplicationV2, error) {
	rows, err := q.RuntimePublishedDefinitions(ctx, tenant)
	if err != nil {
		return nil, err
	}
	apps := []ar.ApplicationV2{}
	revision, err := q.RuntimeCatalogRevision(ctx, tenant)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.Compatibility != "compatible" {
			continue
		}
		var app ar.ApplicationV2
		if err = json.Unmarshal(row.Definition, &app); err != nil {
			return nil, contracts.Problem("invalid")
		}
		app.ApplicationVersion = row.Version
		app.DefinitionDigest = row.Digest
		app.DefinitionRevision = row.Revision
		app.CatalogRevision = revision
		apps = append(apps, app)
	}
	return apps, nil
}
func (m *Module) reconcileEntry() registry.Entry {
	d := contracts.Definition{Name: "application.runtime.catalog.reconcile", Version: "1.0.0", Title: "Reconcile runtime deployment", Description: "Register trusted compiled definitions without overwriting published tenant customizations.", Domain: "runtime", Entity: "runtime_application_head", Entities: []string{"runtime_application_head", "runtime_application_version", "runtime_deployment_snapshot", "runtime_application_change"}, Layer: contracts.LayerAtomic, Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, Input: contracts.Object(nil), Output: contracts.Object(map[string]*contracts.Schema{"initialized": contracts.Int(0, 1<<31)}, "initialized"), Tags: []string{"system-only"}, Idempotency: contracts.Idempotency{Required: true, Scope: "tenant+operation+key"}, SuccessEvents: []string{"application.runtime.catalog.reconciled"}, Executor: "go:application.runtime.catalog.reconcile", AllowedOrigins: []contracts.Origin{contracts.OriginEvent}}
	return registry.Entry{Definition: d, Handler: contracts.Handler{ExecuteTx: func(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
		p, err := identity.Current(ctx)
		if err != nil || p.Account.ID != 0 {
			return contracts.Output{}, contracts.Problem("forbidden")
		}
		q := dbgen.New(tx)
		if err = q.RuntimeTenantLock(ctx, "runtime-installation:"+p.TenantID); err != nil {
			return contracts.Output{}, err
		}
		count := 0
		for _, source := range m.contributions() {
			raw, e := json.Marshal(source)
			if e != nil {
				return contracts.Output{}, e
			}
			digest := contracts.Digest(contracts.Input{"definition": source})
			_, e = q.RuntimeRegisterSnapshot(ctx, dbgen.RuntimeRegisterSnapshotParams{Digest: digest, PluginID: source.Source, PluginVersion: source.PluginVersion, ApplicationID: source.ID, BaseVersion: source.Version, Definition: raw, ContractsDigest: contracts.Digest(contracts.Input{"definitions": m.Definitions})})
			if e != nil {
				return contracts.Output{}, e
			}
			installation, e := q.RuntimeInstallation(ctx, dbgen.RuntimeInstallationParams{TenantID: p.TenantID, PluginID: source.Source})
			if e != nil {
				if e == pgx.ErrNoRows {
					continue
				}
				return contracts.Output{}, e
			}
			if installation.Version != source.PluginVersion {
				continue
			}
			changed, e := q.RuntimeInitializeHead(ctx, dbgen.RuntimeInitializeHeadParams{TenantID: p.TenantID, ApplicationID: source.ID, PluginID: source.Source, BaseDigest: digest, RouteRoot: source.Route})
			if e != nil {
				return contracts.Output{}, e
			}
			if changed == 0 {
				head, e := q.RuntimeDefinitionHead(ctx, dbgen.RuntimeDefinitionHeadParams{TenantID: p.TenantID, ApplicationID: source.ID})
				if e != nil {
					return contracts.Output{}, e
				}
				if head.PublishedVersion != "" {
					stored, e := q.RuntimeDefinitionGet(ctx, dbgen.RuntimeDefinitionGetParams{TenantID: p.TenantID, ApplicationID: source.ID, Version: head.PublishedVersion})
					if e != nil {
						return contracts.Output{}, e
					}
					current, e := decodeDefinition(stored.Definition)
					if e != nil {
						return contracts.Output{}, e
					}
					compatibility := "compatible"
					candidates := []ar.ApplicationV2{current}
					for _, other := range m.Applications {
						if other.ID != current.ID {
							candidates = append(candidates, other)
						}
					}
					definitions := []contracts.Definition{}
					for _, definition := range m.Definitions {
						definitions = append(definitions, definition)
					}
					if _, e = ar.Assemble(candidates, m.Entities, definitions); e != nil {
						compatibility = "incompatible"
					}
					if current.PluginVersion != installation.Version {
						compatibility = "pending_rebase"
					}
					changed, e := q.RuntimeDefinitionSetCompatibility(ctx, dbgen.RuntimeDefinitionSetCompatibilityParams{TenantID: p.TenantID, ApplicationID: source.ID, Compatibility: compatibility})
					if e != nil {
						return contracts.Output{}, e
					}
					if changed > 0 {
						state, e := q.RuntimeApplication(ctx, dbgen.RuntimeApplicationParams{TenantID: p.TenantID, ApplicationID: source.ID})
						if e != nil {
							return contracts.Output{}, e
						}
						if e = q.RuntimeSetApplication(ctx, dbgen.RuntimeSetApplicationParams{TenantID: p.TenantID, ApplicationID: source.ID, PluginID: source.Source, Enabled: state.Enabled}); e != nil {
							return contracts.Output{}, e
						}
					}
				}
				continue
			}
			source.ApplicationVersion = source.Version + ".0"
			raw, e = json.Marshal(source)
			if e != nil {
				return contracts.Output{}, e
			}
			if e = q.RuntimeDefinitionInsert(ctx, dbgen.RuntimeDefinitionInsertParams{TenantID: p.TenantID, ApplicationID: source.ID, Version: source.ApplicationVersion, BaseDigest: digest, Definition: raw, Digest: contracts.Digest(contracts.Input{"definition": source}), Source: "deployment", CreatedBy: 0}); e != nil {
				return contracts.Output{}, e
			}
			if _, e = q.RuntimeDefinitionInitializePublished(ctx, dbgen.RuntimeDefinitionInitializePublishedParams{TenantID: p.TenantID, ApplicationID: source.ID, PublishedVersion: source.ApplicationVersion}); e != nil {
				return contracts.Output{}, e
			}
			count++
		}
		return contracts.Output{Data: map[string]any{"initialized": count}, Events: []contracts.Event{{Type: "application.runtime.catalog.reconciled", AggregateType: "runtime_application_head", AggregateID: p.TenantID, Payload: map[string]any{"initialized": count}}}}, nil
	}}}
}

// Reconcile is invoked before serving requests, through the same fixed Gateway.
func (m *Module) Reconcile(ctx context.Context, g *gateway.Gateway) error {
	tenants, err := dbgen.New(m.Pool).RuntimeDefinitionTenants(ctx)
	if err != nil {
		return err
	}
	key := contracts.Digest(contracts.Input{"catalog": m.contributions(), "contracts": m.Definitions})
	for _, tenant := range tenants {
		p, e := identity.NewPrincipal(&identityAccount, tenant, nil)
		if e != nil {
			return e
		}
		result, e := g.Invoke(identity.WithPrincipal(ctx, p), gateway.Request{Name: "application.runtime.catalog.reconcile", Version: "1.0.0", Input: contracts.Input{}, Origin: contracts.OriginEvent, IdempotencyKey: key})
		if e != nil {
			return e
		}
		if result.Status != contracts.StatusSucceeded {
			return fmt.Errorf("runtime reconcile: %s", result.ErrorCode)
		}
	}
	return nil
}

var identityAccount = genidentity.AccountRecord{ID: 0}

func (m *Module) designCatalog(ctx context.Context, q *dbgen.Queries, tenant string) ([]ar.ApplicationV2, error) {
	rows, err := q.RuntimeDesignDefinitions(ctx, tenant)
	if err != nil {
		return nil, err
	}
	out := []ar.ApplicationV2{}
	for _, row := range rows {
		app, e := decodeDefinition(row.Definition)
		if e != nil {
			return nil, e
		}
		app.ApplicationVersion = row.Version
		app.DefinitionDigest = row.Digest
		app.DefinitionRevision = row.Revision
		out = append(out, app)
	}
	return out, nil
}
