package applicationruntime

import (
	"baseline/services/api/internal/repository/dbgen"
	ar "baseline/services/api/kernel/applicationruntime"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/gateway"
	"baseline/services/api/kernel/capability/registry"
	"baseline/services/api/system/identity"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"regexp"
	"strings"
)

const runtimeHost = "builtin:runtime-host"

var slugPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,49}$`)

func (m *Module) createDefinitionEntry() registry.Entry {
	d := contracts.Definition{Name: "application.runtime.definition.create", Version: "1.0.0", Title: "Create runtime application", Description: "Create a disabled declarative application candidate hosted by the trusted Runtime.", Domain: "runtime", Entity: "runtime_application_head", Entities: []string{"runtime_application_head", "runtime_application_version", "runtime_application_state", "runtime_installation", "runtime_deployment_snapshot", "runtime_application_change"}, Layer: contracts.LayerAtomic, Permission: "applications:design", Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto, Input: contracts.Object(map[string]*contracts.Schema{"slug": contracts.Str(1, 50), "definition": contracts.Str(1, 1048576)}, "slug", "definition"), Output: contracts.Object(map[string]*contracts.Schema{"applicationId": contracts.Str(1, 100), "version": contracts.Str(1, 50)}, "applicationId", "version"), Idempotency: contracts.Idempotency{Required: true, Scope: "tenant+operation+key"}, SuccessEvents: []string{"application.runtime.definition.changed"}, Executor: "go:application.runtime.definition.create"}
	return registry.Entry{Definition: d, Handler: contracts.Handler{ExecuteTx: func(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
		p, err := identity.Current(ctx)
		if err != nil {
			return contracts.Output{}, err
		}
		if !p.CanPermanent("applications:design") {
			return contracts.Output{}, contracts.Problem("forbidden")
		}
		slug := in["slug"].(string)
		if !slugPattern.MatchString(slug) {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		app, err := decodeDefinition([]byte(in["definition"].(string)))
		if err != nil {
			return contracts.Output{}, err
		}
		id := make([]byte, 16)
		if _, err = rand.Read(id); err != nil {
			return contracts.Output{}, err
		}
		app.ID = "app_" + hex.EncodeToString(id)
		app.Source = runtimeHost
		app.Version = "0.1.0"
		app.PluginVersion = "1.0.0"
		app.ApplicationVersion = "0.1.0.0"
		app.PreviewVersion = ""
		app.AgentTools = nil
		app.Components = nil
		app.PluginDependencies = nil
		app.Route = "/apps/" + slug
		app, err = ar.BindApplicationMessages(app)
		if err != nil {
			return contracts.Output{}, err
		}
		for _, page := range app.Pages {
			if page.Route != app.Route && !strings.HasPrefix(page.Route, app.Route+"/") {
				return contracts.Output{}, contracts.Problem("invalid")
			}
		}
		rootPage := ""
		for i, page := range app.Pages {
			if page.Type == "list" && !strings.Contains(page.Route, ":") {
				rootPage = page.ID
				app.Pages[i].Route = app.Route
				break
			}
		}
		if rootPage == "" {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		if len(app.Navigation) == 0 {
			app.Navigation = []ar.Navigation{{PageID: rootPage, LabelKey: app.LabelKey, Icon: "boxes"}}
		}
		q := dbgen.New(tx)
		if err = q.RuntimeTenantLock(ctx, "runtime-installation:"+p.TenantID); err != nil {
			return contracts.Output{}, err
		}
		existing, err := m.publishedCatalogWith(ctx, q, p.TenantID)
		if err != nil {
			return contracts.Output{}, err
		}
		for _, other := range existing {
			if other.Route == app.Route {
				return contracts.Output{}, contracts.Problem("conflict")
			}
		}
		app, err = m.validateCandidate(ctx, q, p, app)
		if err != nil {
			return contracts.Output{}, err
		}
		raw, err := json.Marshal(app)
		if err != nil {
			return contracts.Output{}, err
		}
		digest := contracts.Digest(contracts.Input{"definition": app})
		if _, err = q.RuntimeRegisterSnapshot(ctx, dbgen.RuntimeRegisterSnapshotParams{Digest: digest, PluginID: runtimeHost, PluginVersion: "1.0.0", ApplicationID: app.ID, BaseVersion: "0.1.0", Definition: raw, ContractsDigest: contracts.Digest(contracts.Input{"definitions": m.Definitions})}); err != nil {
			return contracts.Output{}, err
		}
		host, e := q.RuntimeInstallation(ctx, dbgen.RuntimeInstallationParams{TenantID: p.TenantID, PluginID: runtimeHost})
		if e == pgx.ErrNoRows {
			if err = q.RuntimeInstall(ctx, dbgen.RuntimeInstallParams{TenantID: p.TenantID, PluginID: runtimeHost, Version: "1.0.0", Digest: "runtime-host"}); err != nil {
				return contracts.Output{}, err
			}
			if _, err = q.RuntimeChangeInstallation(ctx, dbgen.RuntimeChangeInstallationParams{TenantID: p.TenantID, PluginID: runtimeHost, Revision: 1, Version: "1.0.0", Status: "enabled", Digest: "runtime-host"}); err != nil {
				return contracts.Output{}, err
			}
		} else if e != nil {
			return contracts.Output{}, e
		} else if host.Status != "enabled" {
			return contracts.Output{}, contracts.Problem("unavailable")
		}
		if _, err = q.RuntimeInitializeHead(ctx, dbgen.RuntimeInitializeHeadParams{TenantID: p.TenantID, ApplicationID: app.ID, PluginID: runtimeHost, BaseDigest: digest, RouteRoot: app.Route}); err != nil {
			return contracts.Output{}, err
		}
		if err = q.RuntimeSetApplication(ctx, dbgen.RuntimeSetApplicationParams{TenantID: p.TenantID, ApplicationID: app.ID, PluginID: runtimeHost, Enabled: false}); err != nil {
			return contracts.Output{}, err
		}
		meta, _ := gateway.CurrentInvocation(ctx)
		runRef, e := definitionTask(ctx, q, p)
		if e != nil {
			return contracts.Output{}, e
		}
		sourceType := "human"
		if meta.Origin != contracts.OriginHumanUI {
			sourceType = "agent"
		}
		if err = q.RuntimeDefinitionInsert(ctx, dbgen.RuntimeDefinitionInsertParams{TenantID: p.TenantID, ApplicationID: app.ID, Version: app.ApplicationVersion, BaseDigest: digest, Definition: raw, Digest: digest, Source: "deployment", CreatedBy: p.Account.ID, CorrelationID: meta.CorrelationID}); err != nil {
			return contracts.Output{}, err
		}
		app.ApplicationVersion = "0.1.0.1"
		app, err = m.validateCandidate(ctx, q, p, app)
		if err != nil {
			return contracts.Output{}, err
		}
		raw, err = json.Marshal(app)
		if err != nil {
			return contracts.Output{}, err
		}
		if err = q.RuntimeDefinitionInsert(ctx, dbgen.RuntimeDefinitionInsertParams{TenantID: p.TenantID, ApplicationID: app.ID, Version: app.ApplicationVersion, ParentVersion: "0.1.0.0", BaseDigest: digest, Definition: raw, Digest: contracts.Digest(contracts.Input{"definition": app}), Source: sourceType, CreatedBy: p.Account.ID, AgentRunID: runRef, CorrelationID: meta.CorrelationID}); err != nil {
			return contracts.Output{}, err
		}
		if _, err = q.RuntimeDefinitionPropose(ctx, dbgen.RuntimeDefinitionProposeParams{TenantID: p.TenantID, ApplicationID: app.ID, Revision: 1, ProposalVersion: app.ApplicationVersion}); err != nil {
			return contracts.Output{}, err
		}
		return contracts.Output{Data: map[string]any{"applicationId": app.ID, "version": app.ApplicationVersion}, Events: []contracts.Event{{Type: "application.runtime.definition.changed", AggregateType: "runtime_application_head", AggregateID: app.ID, Payload: map[string]any{"applicationId": app.ID, "version": app.ApplicationVersion, "action": "create"}}}}, nil
	}}}
}
