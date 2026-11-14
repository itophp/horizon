package applicationruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	genidentity "horizon/foundation/gen/identity"
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
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
		if app.Protocol == nil {
			continue
		}
		m.supplyPublishedImportTemplates(&app)
		app.ApplicationVersion = row.Version
		app.DefinitionDigest = row.Digest
		app.DefinitionRevision = row.Revision
		app.CatalogRevision = revision
		apps = append(apps, app)
	}
	return apps, nil
}

// Published snapshots can predate the templatePath requirement. Use the
// matching deployment's template for the same component without rewriting the snapshot.
func (m *Module) supplyPublishedImportTemplates(app *ar.ApplicationV2) {
	if app.Protocol == nil {
		return
	}
	for _, source := range m.contributions() {
		if source.Protocol == nil || source.ID != app.ID || source.Source != app.Source ||
			source.PluginVersion != app.PluginVersion || source.Version != app.Version {
			continue
		}
		for _, page := range app.Protocol.Pages {
			for _, baseline := range source.Protocol.Pages {
				if baseline.ID == page.ID {
					supplyImportTemplates(page.Root.Layout, baseline.Root)
					break
				}
			}
		}
		return
	}
}

func supplyImportTemplates(layout *ar.LayoutSpec, baseline ar.Node) {
	if layout == nil {
		return
	}
	for _, child := range layout.Children {
		supplyImportTemplates(child.Layout, baseline)
		component := child.Component
		if component == nil {
			continue
		}
		supplyImportTemplates(component.Content, baseline)
		if component.Kind != "batch-import" || component.Props["templatePath"] != nil {
			continue
		}
		configured, _ := ar.FindAIComponent(baseline, component.ID)
		if configured == nil || configured.Kind != component.Kind || configured.Version != component.Version {
			continue
		}
		if templatePath, ok := configured.Props["templatePath"].(string); ok && templatePath != "" {
			if component.Props == nil {
				component.Props = map[string]any{}
			}
			component.Props["templatePath"] = templatePath
		}
	}
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
			// New applications joining an already-installed plugin inherit the
			// install default (enabled); an existing tenant choice is preserved.
			stateEnabled := true
			if state, e := q.RuntimeApplication(ctx, dbgen.RuntimeApplicationParams{TenantID: p.TenantID, ApplicationID: source.ID}); e == nil {
				stateEnabled = state.Enabled
			} else if !errors.Is(e, pgx.ErrNoRows) {
				return contracts.Output{}, e
			}
			if e = q.RuntimeSetApplication(ctx, dbgen.RuntimeSetApplicationParams{TenantID: p.TenantID, ApplicationID: source.ID, PluginID: source.Source, Enabled: stateEnabled}); e != nil {
				return contracts.Output{}, e
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
					m.supplyPublishedImportTemplates(&current)
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
	queries := dbgen.New(m.Pool)
	tenants, err := queries.RuntimeDefinitionTenants(ctx)
	if err != nil {
		return err
	}
	// reconcileBehaviorVersion busts the idempotency key only when reconciler
	// behavior changes; catalog/contract content changes already rotate it.
	const reconcileBehaviorVersion = "5-import-template-compatibility"
	deployment := contracts.Digest(contracts.Input{"catalog": m.contributions(), "contracts": m.Definitions, "reconciler": reconcileBehaviorVersion})
	for _, tenant := range tenants {
		// A migration can add installation rows after this deployment was
		// reconciled. Replaying the old result would leave those apps unpublished.
		installations, e := queries.RuntimeInstallations(ctx, tenant)
		if e != nil {
			return e
		}
		versions := make(map[string]string, len(installations))
		for _, installation := range installations {
			versions[installation.PluginID] = installation.Version
		}
		key := contracts.Digest(contracts.Input{"deployment": deployment, "installations": versions})
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
