package applicationruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
	"io"
	"slices"
	"time"
)

func (m *Module) definitionEntries() []registry.Entry {
	entries := []registry.Entry{}
	for _, operation := range []string{"list", "get", "history", "validate", "diff", "preview", "inspect", "contracts", "baselines", "rebase.check", "propose", "publish", "discard", "rollback", "rebase"} {
		op := operation
		read := slices.Contains([]string{"list", "get", "history", "validate", "diff", "preview", "inspect", "contracts", "baselines", "rebase.check"}, op)
		manual := op == "publish" || op == "rollback"
		props := map[string]*contracts.Schema{"applicationId": contracts.Str(1, 100), "version": contracts.Str(1, 50), "baseVersion": contracts.Str(1, 50), "revision": contracts.Int(1, 1<<53), "digest": contracts.Str(1, 100), "targetDigest": contracts.Str(1, 100), "reason": contracts.Str(0, 1000), "changes": {Type: "array", Items: ar.Schema(ar.DefinitionChange{})}}
		if op == "inspect" {
			props["section"] = &contracts.Schema{Type: "string", Enum: []string{"summary", "page", "provider"}}
			props["itemId"] = contracts.Str(1, 100)
			props["pageId"] = contracts.Str(1, 100)
		}
		required := []string{"applicationId"}
		if op == "list" {
			required = nil
			props = map[string]*contracts.Schema{}
		}
		if op == "contracts" {
			required = []string{"kind"}
			props = map[string]*contracts.Schema{"kind": {Type: "string", Enum: []string{"capability", "entity", "component", "layout", "element", "interaction"}}, "name": contracts.Str(1, 200), "offset": contracts.Int(0, 100000)}
		}
		if !read {
			required = append(required, "revision")
		}
		if op == "rebase" || op == "rebase.check" {
			required = append(required, "targetDigest")
		}
		if op == "propose" {
			required = append(required, "baseVersion", "changes")
		}
		if manual {
			required = append(required, "version", "digest")
		}
		permission := "applications:design"
		if manual {
			permission = "applications:manage"
		}
		if op == "list" || op == "get" || op == "history" {
			permission = "applications:read"
		}
		d := contracts.Definition{Name: "application.runtime.definition." + op, Version: "1.0.0", Title: "Runtime definition " + op, Description: "Read and revise immutable tenant application definitions.", Domain: "runtime", Entity: "runtime_application_head", Entities: []string{"runtime_application_head", "runtime_application_version", "runtime_application_change"}, Layer: contracts.LayerAtomic, Permission: permission, Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: read, TransientResult: read, Input: contracts.Object(props, required...), Output: &contracts.Schema{Type: "object", AdditionalProperties: true}, Loggable: []string{"applicationId", "version", "baseVersion", "revision", "digest"}, Executor: "go:application.runtime.definition." + op}
		if op == "get" || op == "preview" {
			d.Output = ar.Schema(ar.DefinitionView{})
		}
		if op == "rebase.check" {
			d.Output = ar.Schema(ar.DefinitionMergeReport{})
		}
		if op == "baselines" {
			d.Output = ar.Schema(ar.DefinitionBaselineList{})
		}
		if op == "history" {
			d.Output = ar.Schema(ar.DefinitionHistory{})
		}
		if op == "diff" {
			d.Output = ar.Schema(ar.DefinitionDiff{})
		}
		if op == "validate" {
			d.Output = ar.Schema(ar.DefinitionValidation{})
		}
		if op == "list" {
			d.Output = ar.Schema(ar.DefinitionList{})
		}
		if !read {
			d.Risk = contracts.RiskMedium
			d.Idempotency = contracts.Idempotency{Required: true, Scope: "tenant+operation+key"}
			d.SuccessEvents = []string{"application.runtime.definition.changed"}
		}
		if manual {
			d.Risk = contracts.RiskHigh
			d.Tags = []string{"manual-only"}
			d.AllowedOrigins = []contracts.Origin{contracts.OriginHumanUI}
		}
		h := contracts.Handler{}
		if read {
			h.Execute = func(ctx context.Context, in contracts.Input) (contracts.Output, error) {
				return m.definitionRead(ctx, op, in)
			}
		} else {
			h.ExecuteTx = func(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
				return m.definitionWrite(ctx, dbgen.New(tx), op, in)
			}
		}
		entries = append(entries, registry.Entry{Definition: d, Handler: h})
	}
	return entries
}
func decodeDefinition(raw []byte) (ar.ApplicationV2, error) {
	var app ar.ApplicationV2
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if len(raw) > 1048576 || decoder.Decode(&app) != nil {
		return app, contracts.Problem("invalid")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return app, contracts.Problem("invalid")
	}
	return app, nil
}
func (m *Module) definitionRead(ctx context.Context, op string, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(m.Pool)
	if op == "contracts" {
		return m.designContracts(p, in)
	}
	if op == "list" {
		apps, err := m.designCatalog(ctx, q, p.TenantID)
		if err != nil {
			return contracts.Output{}, err
		}
		items := []map[string]any{}
		for _, app := range apps {
			if p.Can(app.Permission) {
				labelEn, labelZhCN := "", ""
				for _, message := range app.Messages {
					if message.Key == app.LabelKey {
						labelEn, labelZhCN = message.En, message.ZhCN
					}
				}
				items = append(items, map[string]any{"labelKey": app.LabelKey, "labelEn": labelEn, "labelZhCN": labelZhCN, "applicationId": app.ID, "version": app.ApplicationVersion, "digest": app.DefinitionDigest, "revision": app.DefinitionRevision})
			}
		}
		return contracts.Output{Data: map[string]any{"items": items}}, nil
	}
	id := in["applicationId"].(string)
	head, err := q.RuntimeDefinitionHead(ctx, dbgen.RuntimeDefinitionHeadParams{TenantID: p.TenantID, ApplicationID: id})
	if err != nil {
		return contracts.Output{}, err
	}
	version, _ := in["version"].(string)
	if version == "" {
		version = head.ProposalVersion
		if version == "" {
			version = head.PublishedVersion
		}
	}
	row, err := q.RuntimeDefinitionGet(ctx, dbgen.RuntimeDefinitionGetParams{TenantID: p.TenantID, ApplicationID: id, Version: version})
	if err != nil {
		return contracts.Output{}, err
	}
	app, err := decodeDefinition(row.Definition)
	if err != nil {
		return contracts.Output{}, err
	}
	if !p.Can(app.Permission) {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	if version != head.PublishedVersion && !p.CanPermanent("applications:design") && !p.CanPermanent("applications:manage") {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	if app.Protocol == nil {
		return contracts.Output{}, contracts.Problem("schema_version_unsupported")
	}
	if op == "inspect" {
		section, _ := in["section"].(string)
		id, _ := in["itemId"].(string)
		var value any
		switch section {
		case "", "summary":
			pages := []map[string]any{}
			providers := []map[string]any{}
			for _, page := range app.Protocol.Pages {
				pages = append(pages, map[string]any{"id": page.ID, "route": page.Route, "titleKey": page.TitleKey, "permission": page.Permission})
			}
			for _, provider := range app.Protocol.Providers {
				providers = append(providers, map[string]any{"id": provider.ID, "capability": provider.Capability})
			}
			value = map[string]any{"pages": pages, "providers": providers}
		case "page":
			for _, page := range app.Protocol.Pages {
				if page.ID == id {
					value = map[string]any{"page": page}
				}
			}
		case "provider":
			for _, provider := range app.Protocol.Providers {
				if provider.ID == id {
					value = provider
				}
			}
		}
		if value == nil {
			return contracts.Output{}, contracts.Problem("not_found")
		}
		result := map[string]any{"applicationId": app.ID, "version": version, "revision": head.Revision, "content": value}
		raw, e := json.Marshal(result)
		if e != nil {
			return contracts.Output{}, e
		}
		if len(raw) > 10000 {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		return contracts.Output{Data: result}, nil
	}
	if op == "rebase.check" {
		target, e := q.RuntimeDefinitionSnapshot(ctx, in["targetDigest"].(string))
		if e != nil {
			return contracts.Output{}, e
		}
		if target.ApplicationID != id || target.PluginID != head.PluginID {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		trusted := false
		for _, source := range m.contributions() {
			if contracts.Digest(contracts.Input{"definition": source}) == target.Digest {
				trusted = true
			}
		}
		if !trusted {
			return contracts.Output{}, contracts.Problem("unavailable")
		}
		original, e := q.RuntimeDefinitionSnapshot(ctx, row.BaseDigest)
		if e != nil {
			return contracts.Output{}, e
		}
		base, e := decodeDefinition(original.Definition)
		if e != nil {
			return contracts.Output{}, e
		}
		next, e := decodeDefinition(target.Definition)
		if e != nil {
			return contracts.Output{}, e
		}
		_, conflicts, e := ar.RebaseDefinition(base, app, next)
		if e != nil {
			return contracts.Output{}, e
		}
		if conflicts == nil {
			conflicts = []string{}
		}
		return projection(ar.DefinitionMergeReport{Compatible: len(conflicts) == 0, TargetDigest: target.Digest, Conflicts: conflicts})
	}
	if op == "baselines" {
		items := []map[string]any{}
		for _, source := range m.contributions() {
			if source.ID == id && source.Source == head.PluginID {
				digest := contracts.Digest(contracts.Input{"definition": source})
				snapshot, e := q.RuntimeDefinitionSnapshot(ctx, digest)
				if e != nil {
					return contracts.Output{}, e
				}
				items = append(items, map[string]any{"digest": snapshot.Digest, "pluginVersion": snapshot.PluginVersion, "version": snapshot.BaseVersion})
			}
		}
		return contracts.Output{Data: map[string]any{"items": items}}, nil
	}
	if op == "history" {
		history, err := q.RuntimeDefinitionHistory(ctx, dbgen.RuntimeDefinitionHistoryParams{TenantID: p.TenantID, ApplicationID: id})
		if err != nil {
			return contracts.Output{}, err
		}
		items := []ar.DefinitionHistoryItem{}
		for _, item := range history {
			items = append(items, ar.DefinitionHistoryItem{Version: item.Version, ParentVersion: item.ParentVersion, Digest: item.Digest, Source: item.Source, CreatedAt: item.CreatedAt.Time.UTC().Format(time.RFC3339)})
		}
		return projection(ar.DefinitionHistory{Items: items})
	}
	if op == "validate" {
		_, err = m.validateCandidate(ctx, q, p, app)
		if err != nil {
			return projection(ar.DefinitionValidation{Valid: false, Code: contracts.Code(err)})
		}
		return projection(ar.DefinitionValidation{Valid: true})
	}
	if op == "diff" {
		base, err := q.RuntimeDefinitionGet(ctx, dbgen.RuntimeDefinitionGetParams{TenantID: p.TenantID, ApplicationID: id, Version: head.PublishedVersion})
		if err != nil {
			return contracts.Output{}, err
		}
		baseApp, e := decodeDefinition(base.Definition)
		if e != nil {
			return contracts.Output{}, e
		}
		return projection(ar.DefinitionDiff{BaseVersion: head.PublishedVersion, CandidateVersion: version, BaseDefinition: baseApp, CandidateDefinition: app, Changes: ar.DefinitionImpacts(baseApp, app)})
	}
	state, e := q.RuntimeApplication(ctx, dbgen.RuntimeApplicationParams{TenantID: p.TenantID, ApplicationID: id})
	if e != nil {
		return contracts.Output{}, e
	}
	return contracts.Output{Data: map[string]any{"enabled": state.Enabled, "application": app, "version": version, "digest": row.Digest, "revision": head.Revision, "publishedVersion": head.PublishedVersion, "proposalVersion": head.ProposalVersion, "readOnly": op == "preview"}}, nil
}
func (m *Module) validateCandidate(ctx context.Context, q *dbgen.Queries, p *identity.Principal, app ar.ApplicationV2) (ar.ApplicationV2, error) {
	if app.Protocol == nil {
		return app, contracts.Problem("schema_version_unsupported")
	}
	bound, err := ar.BindApplicationMessages(app)
	if err != nil {
		return app, contracts.Problem("invalid")
	}
	app = bound
	messageKeys := map[string]bool{}
	for _, source := range m.contributions() {
		for _, key := range ar.MessageReferences(source) {
			messageKeys[key] = true
		}
	}
	for _, message := range app.Messages {
		messageKeys[message.Key] = true
	}
	for _, key := range ar.MessageReferences(app) {
		if !messageKeys[key] {
			return app, contracts.Problem("invalid")
		}
	}
	trustedAudience := slices.ContainsFunc(m.contributions(), func(source ar.ApplicationV2) bool {
		return source.ID == app.ID && source.Source == app.Source &&
			source.Permission == ar.AuthenticatedPermission
	})
	canConfigure := func(permission string) bool {
		return (trustedAudience && permission == ar.AuthenticatedPermission) || p.CanPermanent(permission)
	}
	if !canConfigure(app.Permission) || !canConfigure(app.Protocol.Permission) {
		return app, contracts.Problem("forbidden")
	}
	refs := ar.ProtocolCapabilityRefs(*app.Protocol)
	for _, ref := range refs {
		d, ok := m.Definitions[ref.Name+"@"+ref.Version]
		authenticatedProvider := trustedAudience && d.ReadOnly && d.Permission == "" &&
			slices.Contains(d.Tags, contracts.AuthenticatedProviderTag)
		authenticatedAction := trustedAudience && isAuthenticatedAction(d)
		if !ok || (!authenticatedProvider && !authenticatedAction && !p.CanPermanent(d.Permission)) || slices.Contains(d.Tags, "internal-only") || slices.Contains(d.Tags, "system-only") {
			return app, contracts.Problem("forbidden")
		}
	}
	app.MetadataVersion = ""
	app.DefinitionDigest = ""
	app.DefinitionRevision = 0
	app.InstallationRevision = 0
	app.ApplicationRevision = 0
	app.CatalogRevision = 0
	existing, err := m.publishedCatalogWith(ctx, q, p.TenantID)
	if err != nil {
		return app, err
	}
	sources := []ar.ApplicationV2{app}
	for _, other := range existing {
		if other.ID != app.ID {
			sources = append(sources, other)
		}
	}
	definitions := []contracts.Definition{}
	for _, d := range m.Definitions {
		definitions = append(definitions, d)
	}
	result, err := ar.Assemble(sources, m.Entities, definitions)
	if err != nil {
		return app, contracts.Problem("invalid")
	}
	for _, candidate := range result {
		if candidate.ID == app.ID {
			return candidate, nil
		}
	}
	return app, contracts.Problem("invalid")
}
func (m *Module) definitionWrite(ctx context.Context, q *dbgen.Queries, op string, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	permission := "applications:design"
	if op == "publish" || op == "rollback" {
		permission = "applications:manage"
	}
	if !p.CanPermanent(permission) {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	if err = q.RuntimeTenantLock(ctx, "runtime-installation:"+p.TenantID); err != nil {
		return contracts.Output{}, err
	}
	id := in["applicationId"].(string)
	head, err := q.RuntimeDefinitionHeadLock(ctx, dbgen.RuntimeDefinitionHeadLockParams{TenantID: p.TenantID, ApplicationID: id})
	if err != nil {
		return contracts.Output{}, err
	}
	if head.Revision != inputInt(in["revision"]) {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	version, _ := in["version"].(string)
	if op == "propose" {
		version = in["baseVersion"].(string)
	}
	if version == "" {
		version = head.ProposalVersion
		if version == "" {
			version = head.PublishedVersion
		}
	}
	row, err := q.RuntimeDefinitionGet(ctx, dbgen.RuntimeDefinitionGetParams{TenantID: p.TenantID, ApplicationID: id, Version: version})
	if err != nil {
		return contracts.Output{}, err
	}
	app, err := decodeDefinition(row.Definition)
	if err != nil {
		return contracts.Output{}, err
	}
	if !p.CanPermanent(app.Permission) {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	if op == "publish" || op == "rollback" {
		installed, e := q.RuntimeInstallation(ctx, dbgen.RuntimeInstallationParams{TenantID: p.TenantID, PluginID: head.PluginID})
		if e != nil || installed.Status == "uninstalled" || installed.Version != app.PluginVersion {
			return contracts.Output{}, contracts.Problem("conflict")
		}
	}
	if head.NextConfiguration > 4294967295 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	if op == "discard" {
		if head.ProposalVersion == "" || version != head.ProposalVersion {
			return contracts.Output{}, contracts.Problem("conflict")
		}
		_, err = q.RuntimeDefinitionDiscard(ctx, dbgen.RuntimeDefinitionDiscardParams{TenantID: p.TenantID, ApplicationID: id, Revision: head.Revision})
	} else if op == "publish" {
		if version != head.ProposalVersion || row.Digest != in["digest"] {
			return contracts.Output{}, contracts.Problem("conflict")
		}
		if _, err = m.validateCandidate(ctx, q, p, app); err != nil {
			return contracts.Output{}, err
		}
		_, err = q.RuntimeDefinitionPublish(ctx, dbgen.RuntimeDefinitionPublishParams{TenantID: p.TenantID, ApplicationID: id, Revision: head.Revision, PublishedVersion: version})
	} else {
		baseDigest := row.BaseDigest
		if op == "rebase" {
			target, e := q.RuntimeDefinitionSnapshot(ctx, in["targetDigest"].(string))
			if e != nil {
				return contracts.Output{}, e
			}
			if target.ApplicationID != id || target.PluginID != head.PluginID {
				return contracts.Output{}, contracts.Problem("invalid")
			}
			trusted := false
			for _, source := range m.contributions() {
				if contracts.Digest(contracts.Input{"definition": source}) == target.Digest {
					trusted = true
				}
			}
			if !trusted {
				return contracts.Output{}, contracts.Problem("unavailable")
			}
			original, e := q.RuntimeDefinitionSnapshot(ctx, row.BaseDigest)
			if e != nil {
				return contracts.Output{}, e
			}
			oldBase, e := decodeDefinition(original.Definition)
			if e != nil {
				return contracts.Output{}, e
			}
			newBase, e := decodeDefinition(target.Definition)
			if e != nil {
				return contracts.Output{}, e
			}
			merged, conflicts, e := ar.RebaseDefinition(oldBase, app, newBase)
			if e != nil {
				return contracts.Output{}, e
			}
			if len(conflicts) > 0 {
				return contracts.Output{}, contracts.Problem("conflict")
			}
			app = merged
			baseDigest = target.Digest
		} else if op == "propose" {
			expectedBase := head.ProposalVersion
			if expectedBase == "" {
				expectedBase = head.PublishedVersion
			}
			if version != expectedBase {
				return contracts.Output{}, contracts.Problem("conflict")
			}
			raw, e := json.Marshal(in["changes"])
			if e != nil {
				return contracts.Output{}, e
			}
			changes := []ar.DefinitionChange{}
			if json.Unmarshal(raw, &changes) != nil {
				return contracts.Output{}, contracts.Problem("invalid")
			}
			app, e = ar.ApplyDefinitionChanges(app, changes)
			if e != nil {
				return contracts.Output{}, contracts.Problem("invalid")
			}
		} else if op != "rebase" && row.Digest != in["digest"] {
			return contracts.Output{}, contracts.Problem("conflict")
		}
		app.ApplicationVersion = fmt.Sprintf("%s.%d", app.Version, head.NextConfiguration)
		app, err = m.validateCandidate(ctx, q, p, app)
		if err != nil {
			return contracts.Output{}, err
		}
		raw, e := json.Marshal(app)
		if e != nil {
			return contracts.Output{}, e
		}
		if len(raw) > 1048576 {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		source := "human"
		meta, present := gateway.CurrentInvocation(ctx)
		if present && meta.Origin != contracts.OriginHumanUI {
			source = "agent"
		}
		if op == "rebase" {
			source = "rebase"
		}
		if op == "rollback" {
			source = "rollback"
		}
		digest := contracts.Digest(contracts.Input{"definition": app})
		version = app.ApplicationVersion
		runRef, e := definitionTask(ctx, q, p)
		if e != nil {
			return contracts.Output{}, e
		}
		if err = q.RuntimeDefinitionInsert(ctx, dbgen.RuntimeDefinitionInsertParams{TenantID: p.TenantID, ApplicationID: id, Version: version, ParentVersion: row.Version, BaseDigest: baseDigest, Definition: raw, Digest: digest, Source: source, CreatedBy: p.Account.ID, AgentRunID: runRef, CorrelationID: meta.CorrelationID}); err != nil {
			return contracts.Output{}, err
		}
		if _, err = q.RuntimeDefinitionPropose(ctx, dbgen.RuntimeDefinitionProposeParams{TenantID: p.TenantID, ApplicationID: id, Revision: head.Revision, ProposalVersion: version}); err != nil {
			return contracts.Output{}, err
		}
		if op == "rollback" {
			_, err = q.RuntimeDefinitionPublish(ctx, dbgen.RuntimeDefinitionPublishParams{TenantID: p.TenantID, ApplicationID: id, Revision: head.Revision + 1, PublishedVersion: version})
		}
	}
	if err != nil {
		return contracts.Output{}, err
	}
	meta, _ := gateway.CurrentInvocation(ctx)
	written, err := q.RuntimeDefinitionGet(ctx, dbgen.RuntimeDefinitionGetParams{TenantID: p.TenantID, ApplicationID: id, Version: version})
	if err != nil {
		return contracts.Output{}, err
	}
	if err = q.RuntimeDefinitionChange(ctx, dbgen.RuntimeDefinitionChangeParams{TenantID: p.TenantID, ApplicationID: id, Action: op, Version: version, Digest: written.Digest, AccountID: p.Account.ID, CorrelationID: meta.CorrelationID}); err != nil {
		return contracts.Output{}, err
	}
	if op == "publish" || op == "rollback" {
		state, e := q.RuntimeApplication(ctx, dbgen.RuntimeApplicationParams{TenantID: p.TenantID, ApplicationID: id})
		if e != nil {
			return contracts.Output{}, e
		}
		if err = q.RuntimeSetApplication(ctx, dbgen.RuntimeSetApplicationParams{TenantID: p.TenantID, ApplicationID: id, PluginID: head.PluginID, Enabled: state.Enabled}); err != nil {
			return contracts.Output{}, err
		}
	}
	catalogRevision, err := q.RuntimeCatalogRevision(ctx, p.TenantID)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"applicationId": id, "version": version}, Events: []contracts.Event{{Type: "application.runtime.definition.changed", AggregateType: "runtime_application_head", AggregateID: id, Payload: map[string]any{"applicationId": id, "version": version, "digest": written.Digest, "action": op, "catalogRevision": catalogRevision}}}}, nil
}
