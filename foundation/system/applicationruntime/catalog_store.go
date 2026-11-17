package applicationruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

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
	d := contracts.Definition{Name: "application.runtime.catalog.reconcile", Version: "1.0.0", Title: "Reconcile runtime deployment", Description: "Register trusted compiled definitions, keep platform base applications enabled at the deployed baseline, and preserve published business customizations.", Domain: "runtime", Entity: "runtime_application_head", Entities: []string{"runtime_application_head", "runtime_application_version", "runtime_deployment_snapshot", "runtime_application_change"}, Layer: contracts.LayerAtomic, Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, Input: contracts.Object(nil), Output: contracts.Object(map[string]*contracts.Schema{"initialized": contracts.Int(0, 1<<31)}, "initialized"), Tags: []string{"system-only"}, Idempotency: contracts.Idempotency{Required: true, Scope: "tenant+operation+key"}, SuccessEvents: []string{"application.runtime.catalog.reconciled"}, Executor: "go:application.runtime.catalog.reconcile", AllowedOrigins: []contracts.Origin{contracts.OriginEvent}}
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
		latestBase := map[string]string{}
		for _, source := range m.contributions() {
			if !m.isBasePlugin(source.Source) {
				continue
			}
			if current, ok := latestBase[source.Source]; !ok || ar.NewerVersion(source.PluginVersion, current) {
				latestBase[source.Source] = source.PluginVersion
			}
		}
		for _, source := range m.contributions() {
			source, digest, e := m.registerDeploymentSnapshot(ctx, q, source)
			if e != nil {
				return contracts.Output{}, e
			}
			if m.isBasePlugin(source.Source) {
				if source.PluginVersion != latestBase[source.Source] {
					continue
				}
				wrote, e := m.reconcileBaseApplication(ctx, q, p.TenantID, source, digest)
				if e != nil {
					return contracts.Output{}, e
				}
				if wrote {
					count++
				}
				continue
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
			raw, e := json.Marshal(source)
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

// reconcileBaseApplication keeps a platform-managed package installed and
// enabled at the deployed version. Its published definition always follows the
// deployment baseline: tenant customizations of base applications are not part
// of the product surface and are replaced on the next reconciliation. The
// returned boolean reports whether a definition baseline was written.
func (m *Module) reconcileBaseApplication(ctx context.Context, q *dbgen.Queries, tenant string, source ar.ApplicationV2, digest string) (bool, error) {
	pluginID := source.Source
	installed, err := q.RuntimeInstallationLock(ctx, dbgen.RuntimeInstallationLockParams{TenantID: tenant, PluginID: pluginID})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if err = q.RuntimeInstall(ctx, dbgen.RuntimeInstallParams{TenantID: tenant, PluginID: pluginID, Version: source.PluginVersion, Digest: digest}); err != nil {
			return false, err
		}
		changed, err := q.RuntimeChangeInstallation(ctx, dbgen.RuntimeChangeInstallationParams{TenantID: tenant, PluginID: pluginID, Revision: 1, Version: source.PluginVersion, Status: "enabled", Digest: digest})
		if err != nil {
			return false, err
		}
		if changed != 1 {
			return false, contracts.Problem("conflict")
		}
		if err = q.RuntimeRecordHistory(ctx, dbgen.RuntimeRecordHistoryParams{TenantID: tenant, PluginID: pluginID, AccountID: 0, Action: "provision", Version: source.PluginVersion, Revision: 2}); err != nil {
			return false, err
		}
	case err != nil:
		return false, err
	case installed.Version != source.PluginVersion || installed.Status != "enabled":
		previous := installed.PreviousVersion
		action := "auto_enable"
		if installed.Version != source.PluginVersion {
			previous = installed.Version
			action = "auto_upgrade"
		}
		changed, err := q.RuntimeChangeInstallation(ctx, dbgen.RuntimeChangeInstallationParams{TenantID: tenant, PluginID: pluginID, Revision: installed.Revision, Version: source.PluginVersion, Status: "enabled", Digest: digest, PreviousVersion: previous})
		if err != nil {
			return false, err
		}
		if changed != 1 {
			return false, contracts.Problem("conflict")
		}
		if err = q.RuntimeRecordHistory(ctx, dbgen.RuntimeRecordHistoryParams{TenantID: tenant, PluginID: pluginID, AccountID: 0, Action: action, Version: source.PluginVersion, Revision: installed.Revision + 1}); err != nil {
			return false, err
		}
	}
	state, err := q.RuntimeApplication(ctx, dbgen.RuntimeApplicationParams{TenantID: tenant, ApplicationID: source.ID})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if errors.Is(err, pgx.ErrNoRows) || !state.Enabled {
		if err = q.RuntimeSetApplication(ctx, dbgen.RuntimeSetApplicationParams{TenantID: tenant, ApplicationID: source.ID, PluginID: pluginID, Enabled: true}); err != nil {
			return false, err
		}
	}
	head, err := q.RuntimeDefinitionHeadLock(ctx, dbgen.RuntimeDefinitionHeadLockParams{TenantID: tenant, ApplicationID: source.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err = q.RuntimeInitializeHead(ctx, dbgen.RuntimeInitializeHeadParams{TenantID: tenant, ApplicationID: source.ID, PluginID: pluginID, BaseDigest: digest, RouteRoot: source.Route}); err != nil {
			return false, err
		}
		if err = m.publishDeploymentBaseline(ctx, q, tenant, source, digest); err != nil {
			return false, err
		}
		return m.touchBaseApplication(ctx, q, tenant, source)
	}
	if err != nil {
		return false, err
	}
	if head.PluginID != pluginID {
		return false, contracts.Problem("conflict")
	}
	if head.PublishedVersion != "" {
		current, e := q.RuntimeDefinitionGet(ctx, dbgen.RuntimeDefinitionGetParams{TenantID: tenant, ApplicationID: source.ID, Version: head.PublishedVersion})
		if e != nil {
			return false, e
		}
		if deploymentBaselineMatches(current.Definition, source) {
			if head.Compatibility != "compatible" {
				if _, e = q.RuntimeDefinitionSetCompatibility(ctx, dbgen.RuntimeDefinitionSetCompatibilityParams{TenantID: tenant, ApplicationID: source.ID, Compatibility: "compatible"}); e != nil {
					return false, e
				}
			}
			return false, nil
		}
	}
	if err = m.publishDeploymentBaseline(ctx, q, tenant, source, digest); err != nil {
		return false, err
	}
	return m.touchBaseApplication(ctx, q, tenant, source)
}

// touchBaseApplication advances the effective catalog revision so browser
// catalogs observe a republished platform definition.
func (m *Module) touchBaseApplication(ctx context.Context, q *dbgen.Queries, tenant string, source ar.ApplicationV2) (bool, error) {
	if err := q.RuntimeSetApplication(ctx, dbgen.RuntimeSetApplicationParams{TenantID: tenant, ApplicationID: source.ID, PluginID: source.Source, Enabled: true}); err != nil {
		return false, err
	}
	return true, nil
}

// publishDeploymentBaseline writes a new immutable baseline version and makes
// it the published pointer with the system account. The caller holds the
// tenant advisory lock and the application head lock.
func (m *Module) publishDeploymentBaseline(ctx context.Context, q *dbgen.Queries, tenant string, source ar.ApplicationV2, digest string) error {
	head, err := q.RuntimeDefinitionHeadLock(ctx, dbgen.RuntimeDefinitionHeadLockParams{TenantID: tenant, ApplicationID: source.ID})
	if err != nil {
		return err
	}
	next := head.NextConfiguration
	if head.PublishedVersion == "" {
		next = 0
	}
	version := ""
	for {
		if next > 4294967295 || next < 0 {
			return contracts.Problem("conflict")
		}
		version = fmt.Sprintf("%s.%d", source.Version, next)
		_, e := q.RuntimeDefinitionGet(ctx, dbgen.RuntimeDefinitionGetParams{TenantID: tenant, ApplicationID: source.ID, Version: version})
		if errors.Is(e, pgx.ErrNoRows) {
			break
		}
		if e != nil {
			return e
		}
		next++
	}
	source.ApplicationVersion = version
	raw, err := json.Marshal(source)
	if err != nil {
		return err
	}
	definitionDigest := contracts.Digest(contracts.Input{"definition": source})
	if err = q.RuntimeDefinitionInsert(ctx, dbgen.RuntimeDefinitionInsertParams{TenantID: tenant, ApplicationID: source.ID, Version: version, ParentVersion: head.PublishedVersion, BaseDigest: digest, Definition: raw, Digest: definitionDigest, Source: "deployment", CreatedBy: 0}); err != nil {
		return err
	}
	action := "upgrade"
	if head.PublishedVersion == "" {
		action = "provision"
		if _, err = q.RuntimeDefinitionInitializePublished(ctx, dbgen.RuntimeDefinitionInitializePublishedParams{TenantID: tenant, ApplicationID: source.ID, PublishedVersion: version}); err != nil {
			return err
		}
	} else {
		changed, e := q.RuntimeDefinitionPropose(ctx, dbgen.RuntimeDefinitionProposeParams{TenantID: tenant, ApplicationID: source.ID, Revision: head.Revision, ProposalVersion: version})
		if e != nil {
			return e
		}
		if changed != 1 {
			return contracts.Problem("conflict")
		}
		changed, e = q.RuntimeDefinitionPublish(ctx, dbgen.RuntimeDefinitionPublishParams{TenantID: tenant, ApplicationID: source.ID, Revision: head.Revision + 1, PublishedVersion: version})
		if e != nil {
			return e
		}
		if changed != 1 {
			return contracts.Problem("conflict")
		}
	}
	return q.RuntimeDefinitionChange(ctx, dbgen.RuntimeDefinitionChangeParams{TenantID: tenant, ApplicationID: source.ID, Action: action, Version: version, Digest: definitionDigest, AccountID: 0})
}

// deploymentBaselineMatches reports whether a stored definition is the
// unmodified deployment baseline. The fourth configuration octet is assigned
// when a definition version is persisted and is not part of the comparison.
func deploymentBaselineMatches(raw []byte, source ar.ApplicationV2) bool {
	stored, err := decodeDefinition(raw)
	if err != nil {
		return false
	}
	stored.ApplicationVersion = ""
	expected := source
	expected.ApplicationVersion = ""
	return reflect.DeepEqual(ar.Canonical(stored), ar.Canonical(expected))
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
	const reconcileBehaviorVersion = "8-platform-base-applications"
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
		if e == nil && result.Status != contracts.StatusSucceeded && result.Replayed {
			// A failed attempt is recorded under the same key forever, which would
			// turn one transient database conflict into a permanent startup failure.
			// Reconciliation is idempotent and system-only, so retry once under a
			// fresh key instead of replaying the stored failure.
			retryKey := contracts.Digest(contracts.Input{"deployment": deployment, "installations": versions, "retry": fmt.Sprint(time.Now().UnixNano())})
			result, e = g.Invoke(identity.WithPrincipal(ctx, p), gateway.Request{Name: "application.runtime.catalog.reconcile", Version: "1.0.0", Input: contracts.Input{}, Origin: contracts.OriginEvent, IdempotencyKey: retryKey})
		}
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
