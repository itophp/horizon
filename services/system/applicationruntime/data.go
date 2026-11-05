package applicationruntime

import (
	"baseline/services/api/internal/repository/dbgen"
	ar "baseline/services/api/kernel/applicationruntime"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/gateway"
	"baseline/services/api/kernel/capability/registry"
	"baseline/services/api/system/identity"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (m *Module) dataEntries() []registry.Entry {
	out := []registry.Entry{}
	obj := func() *contracts.Schema { return &contracts.Schema{Type: "object", AdditionalProperties: true} }
	for _, kind := range []string{"data.read", "context.resolve", "action.invoke"} {
		write := kind == "action.invoke"
		props := map[string]*contracts.Schema{"applicationId": contracts.Str(1, 80), "pageId": contracts.Str(1, 100), "metadataVersion": contracts.Str(0, 100), "providerId": contracts.Str(0, 100), "input": obj(), "filter": ar.Schema(ar.FilterExpression{}), "select": {Type: "array", Items: contracts.Str(1, 100)}, "runId": contracts.Int(1, 1<<53), "recordId": contracts.Int(1, 1<<53), "selection": {Type: "array", Items: contracts.Int(1, 1<<53)}, "actionId": contracts.Str(1, 100), "form": obj(), "query": obj(), "route": obj()}
		if kind == "data.read" {
			props["previewVersion"] = contracts.Str(1, 50)
		}
		required := []string{"applicationId", "pageId"}
		if kind == "data.read" {
			required = append(required, "providerId", "input")
		}
		if write {
			required = append(required, "actionId", "metadataVersion")
		}
		d := contracts.Definition{Name: "application.runtime." + kind, Version: "1.0.0", Title: "Runtime " + kind, Description: "Resolve a trusted active definition and invoke fixed authorized providers or actions.", Domain: "runtime", Entity: "runtime_installation", Layer: contracts.LayerOrchestration, Input: contracts.Object(props, required...), Output: obj(), Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: !write, TransientResult: !write, Loggable: []string{"applicationId", "pageId", "providerId"}, Executor: "go:application.runtime." + kind}
		if kind == "context.resolve" {
			d.Output = ar.Schema(ar.SemanticContext{})
		}
		if write {
			d.Idempotency = contracts.Idempotency{Required: true, Scope: "tenant+operation+key"}
			d.SuccessEvents = []string{"application.runtime.action.dispatched"}
		}
		k := kind
		out = append(out, registry.Entry{Definition: d, Handler: contracts.Handler{Execute: func(ctx context.Context, in contracts.Input) (contracts.Output, error) {
			return m.dispatchData(ctx, k, in)
		}}})
	}
	return out
}
func (m *Module) activePage(ctx context.Context, p *identity.Principal, in contracts.Input) (ar.ApplicationV2, ar.PageV2, error) {
	apps, loadErr := m.publishedCatalog(ctx)
	if loadErr != nil {
		return ar.ApplicationV2{}, ar.PageV2{}, loadErr
	}
	if preview, ok := in["previewVersion"].(string); ok && preview != "" {
		if !p.CanPermanent("applications:design") {
			return ar.ApplicationV2{}, ar.PageV2{}, contracts.Problem("forbidden")
		}
		row, e := dbgen.New(m.Pool).RuntimeDefinitionGet(ctx, dbgen.RuntimeDefinitionGetParams{TenantID: p.TenantID, ApplicationID: in["applicationId"].(string), Version: preview})
		if e != nil {
			return ar.ApplicationV2{}, ar.PageV2{}, e
		}
		candidate, e := decodeDefinition(row.Definition)
		if e != nil {
			return candidate, ar.PageV2{}, e
		}
		available := candidate.Source == runtimeHost && candidate.PluginVersion == "1.0.0"
		for _, source := range m.contributions() {
			if source.ID == candidate.ID && source.Source == candidate.Source && source.PluginVersion == candidate.PluginVersion {
				available = true
			}
		}
		if !available {
			return candidate, ar.PageV2{}, contracts.Problem("unavailable")
		}
		apps = []ar.ApplicationV2{candidate}
	}
	for _, app := range apps {
		if app.ID != in["applicationId"] || !p.Can(app.Permission) {
			continue
		}
		if m.Pool != nil {
			install, err := dbgen.New(m.Pool).RuntimeInstallation(ctx, dbgen.RuntimeInstallationParams{TenantID: p.TenantID, PluginID: app.Source})
			state, se := dbgen.New(m.Pool).RuntimeApplication(ctx, dbgen.RuntimeApplicationParams{TenantID: p.TenantID, ApplicationID: app.ID})
			if err != nil || se != nil || install.Status != "enabled" || (install.Version != app.PluginVersion && in["previewVersion"] == nil) || (!state.Enabled && in["previewVersion"] == nil) {
				return app, ar.PageV2{}, contracts.Problem("unavailable")
			}
		}
		if v, ok := in["metadataVersion"].(string); ok && v != "" && v != app.MetadataVersion {
			return app, ar.PageV2{}, contracts.Problem("conflict")
		}
		for _, page := range app.Pages {
			if page.ID == in["pageId"] && p.Can(page.Permission) {
				return app, page, nil
			}
		}
	}
	return ar.ApplicationV2{}, ar.PageV2{}, contracts.Problem("not_found")
}
func (m *Module) providerCall(ctx context.Context, app ar.ApplicationV2, id string, input contracts.Input) (any, error) {
	if m.Invoke == nil {
		return nil, contracts.Problem("unavailable")
	}
	for _, provider := range app.Providers {
		if provider.ID != id {
			continue
		}
		d, ok := m.Definitions[provider.Capability.Name+"@"+provider.Capability.Version]
		if !ok || !d.ReadOnly || d.Input.Validate(input) != nil {
			return nil, contracts.Problem("invalid")
		}
		deadline := 5 * time.Second
		if provider.TimeoutMs > 0 {
			deadline = time.Duration(provider.TimeoutMs) * time.Millisecond
		}
		ctx, cancel := context.WithTimeout(ctx, deadline)
		defer cancel()
		result, err := m.Invoke(ctx, d.Name, d.Version, input, "")
		if err != nil {
			return nil, err
		}
		if result.Status != contracts.StatusSucceeded {
			return nil, contracts.Problem(result.ErrorCode)
		}
		return result.Data, nil
	}
	return nil, contracts.Problem("not_found")
}
func normalized(v any) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, contracts.Problem("invalid")
	}
	out := map[string]any{}
	if json.Unmarshal(raw, &out) != nil {
		return nil, contracts.Problem("invalid")
	}
	return out, nil
}
func (m *Module) record(ctx context.Context, app ar.ApplicationV2, page ar.PageV2, id any) (map[string]any, error) {
	// Resolve the declared detail provider for the same entity; never trust a client record body.
	for _, detail := range app.Pages {
		if detail.EntityRef == page.EntityRef && detail.Type == "detail" {
			p, err := identity.Current(ctx)
			if err != nil || !p.Can(detail.Permission) {
				return nil, contracts.Problem("forbidden")
			}
			for _, provider := range app.Providers {
				if provider.ID == detail.Provider {
					input, err := ar.Map(provider.Input, map[string]any{"route": map[string]any{"id": id}, "record": map[string]any{"id": id}})
					if err != nil {
						return nil, contracts.Problem("invalid")
					}
					v, err := m.providerCall(ctx, app, provider.ID, contracts.Input(input))
					if err != nil {
						return nil, err
					}
					return normalized(v)
				}
			}
		}
	}
	return nil, contracts.Problem("not_found")
}
func (m *Module) dispatchData(ctx context.Context, kind string, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	if kind == "context.resolve" {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	if raw, ok := in["runId"]; ok {
		run, e := dbgen.New(m.Pool).AIGetRun(ctx, dbgen.AIGetRunParams{TenantID: p.TenantID, AccountID: p.Account.ID, ID: inputInt(raw)})
		invocation, present := gateway.CurrentInvocation(ctx)
		if e != nil || !present || !p.Can("assistant:use") || run.DepartmentID != p.Account.DepartmentID || run.CorrelationID != invocation.CorrelationID || run.Status != "running" {
			return contracts.Output{}, contracts.Problem("forbidden")
		}
	}
	app, page, err := m.activePage(ctx, p, in)
	if err != nil {
		return contracts.Output{}, err
	}
	if kind == "data.read" {
		providerID := in["providerId"].(string)
		allowed := providerID == page.Provider
		for _, f := range page.Forms {
			allowed = allowed || f.Provider == providerID
			for _, fields := range [][]ar.FieldV2{f.Create.Fields, f.Edit.Fields} {
				for _, field := range fields {
					allowed = allowed || field.Reference == providerID
				}
			}
		}
		if !allowed {
			return contracts.Output{}, contracts.Problem("forbidden")
		}
		input, ok := in["input"].(map[string]any)
		if !ok {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		var binding ar.Provider
		for _, provider := range app.Providers {
			if provider.ID == providerID {
				binding = provider
			}
		}
		if filter, ok := in["filter"]; ok {
			raw, e := json.Marshal(filter)
			if e != nil {
				return contracts.Output{}, contracts.Problem("invalid")
			}
			var expr ar.FilterExpression
			if json.Unmarshal(raw, &expr) != nil {
				return contracts.Output{}, contracts.Problem("invalid")
			}
			mapped, e := ar.CompileFilter(expr, binding)
			if e != nil {
				return contracts.Output{}, contracts.Problem("invalid")
			}
			for key, value := range mapped {
				if _, exists := input[key]; exists {
					return contracts.Output{}, contracts.Problem("invalid")
				}
				input[key] = value
			}
		}
		for _, key := range []string{"pageSize", "limit"} {
			if value, ok := input[key]; ok && inputInt(value) > 100 {
				return contracts.Output{}, contracts.Problem("invalid")
			}
		}
		v, err := m.providerCall(ctx, app, providerID, contracts.Input(input))
		if err != nil {
			return contracts.Output{}, err
		}
		selected := append([]string{}, binding.Select...)
		if requested, ok := in["select"].([]any); ok {
			if len(requested) > 50 {
				return contracts.Output{}, contracts.Problem("invalid")
			}
			allowed := map[string]bool{}
			for _, field := range binding.Select {
				allowed[field] = true
			}
			if len(binding.Select) == 0 {
				for _, field := range append(append([]ar.FieldV2{}, page.Fields...), page.Table.Columns...) {
					if !field.Sensitive {
						allowed[field.Name] = true
					}
				}
			}
			selected = []string{}
			for _, raw := range requested {
				field, ok := raw.(string)
				if !ok || !allowed[field] {
					return contracts.Output{}, contracts.Problem("forbidden")
				}
				selected = append(selected, field)
			}
		}
		if len(selected) > 0 {
			v, err = projectProvider(v, binding, selected)
			if err != nil {
				return contracts.Output{}, err
			}
		}
		return contracts.Output{Data: v, Summary: fmt.Sprintf("provider=%s fields=%d", binding.ID, len(selected))}, nil
	}
	var record map[string]any
	if id, ok := in["recordId"]; ok {
		record, err = m.record(ctx, app, page, id)
		if err != nil {
			return contracts.Output{}, err
		}
	}
	selection := []ar.RecordReference{}
	if ids, ok := in["selection"].([]any); ok {
		limit := 20
		if kind == "action.invoke" {
			limit = 100
		}
		if len(ids) > limit {
			return contracts.Output{}, contracts.Problem("invalid")
		}
		for _, id := range ids {
			r, e := m.record(ctx, app, page, id)
			if e != nil {
				return contracts.Output{}, e
			}
			selection = append(selection, ar.RecordReference{EntityRef: page.EntityRef, RecordID: fmt.Sprint(r["id"])})
		}
	}
	scope := map[string]any{"record": record, "context": map[string]any{"accountId": float64(p.Account.ID), "canUseAssistant": p.Can("assistant:use"), "applicationId": app.ID, "pageId": page.ID}, "route": map[string]any{"id": in["recordId"]}, "query": in["query"], "form": in["form"], "selection": map[string]any{"ids": in["selection"]}}
	if kind == "action.invoke" {
		for _, action := range page.Actions {
			if action.ID != in["actionId"] {
				continue
			}
			if !p.Can(action.Permission) || !action.WebMCP || action.Capability.Name == "" || action.Type == "custom" {
				return contracts.Output{}, contracts.Problem("forbidden")
			}
			visible, e := ar.Evaluate(action.When, scope)
			if e != nil || visible != true {
				return contracts.Output{}, contracts.Problem("forbidden")
			}
			input, e := ar.Map(action.Input, scope)
			if e != nil {
				return contracts.Output{}, contracts.Problem("invalid")
			}
			result, e := m.Invoke(ctx, action.Capability.Name, action.Capability.Version, contracts.Input(input), gateway.ChildKey(ctx, action.ID))
			if e != nil {
				return contracts.Output{}, e
			}
			return contracts.Output{Data: result, Events: []contracts.Event{{Type: "application.runtime.action.dispatched", AggregateType: "runtime_installation", AggregateID: app.Source, Payload: map[string]any{"actionId": action.ID, "invocationId": result.InvocationID}}}}, nil
		}
		return contracts.Output{}, contracts.Problem("not_found")
	}
	semantic := ar.SemanticContext{ApplicationID: app.ID, ApplicationVersion: app.ApplicationVersion, MetadataVersion: app.MetadataVersion, PageID: page.ID, PageType: page.Type, EntityRef: page.EntityRef, Selection: selection, References: []ar.RecordReference{}, AvailableActions: []ar.SemanticAction{}, Source: app.Source, Locale: "", Timezone: "", Fields: []ar.SemanticField{}}
	if record != nil {
		semantic.RecordID = fmt.Sprint(record["id"])
	}
	for _, f := range append(append([]ar.FieldV2{}, page.Fields...), page.Table.Columns...) {
		if !f.Semantic || f.Permission != "" && !p.Can(f.Permission) || f.Sensitive || f.Hidden || f.Name == "body" || f.Name == "description" {
			continue
		}
		if v, ok := record[f.Name]; ok {
			raw, e := json.Marshal(v)
			if e == nil && len(raw) <= 300 {
				semantic.Fields = append(semantic.Fields, ar.SemanticField{Name: f.Name, Value: string(raw)})
			}
		}
	}
	for _, action := range page.Actions {
		if action.Permission != "" && !p.Can(action.Permission) {
			continue
		}
		if action.Type == "dialog" && action.Mode != "detail" {
			allowed := false
			for _, form := range page.Forms {
				if form.ID == action.Target {
					command := form.Create
					if action.Mode == "edit" {
						command = form.Edit
					}
					condition, e := ar.Evaluate(command.When, scope)
					if command.Capability.Name != "" && p.Can(command.Permission) && e == nil && condition == true {
						allowed = true
					}
				}
			}
			if !allowed {
				continue
			}
		}
		visible, e := ar.Evaluate(action.When, scope)
		if e != nil || visible != true {
			continue
		}
		semantic.AvailableActions = append(semantic.AvailableActions, ar.SemanticAction{ID: action.ID, LabelKey: action.LabelKey, Capability: action.Capability})
	}
	if record != nil {
		for _, binding := range page.RelationBindings {
			if len(semantic.References) >= 10 {
				semantic.Truncated = true
				break
			}
			var provider ar.Provider
			for _, candidate := range app.Providers {
				if candidate.ID == binding.ProviderID {
					provider = candidate
				}
			}
			input, e := ar.Map(provider.Input, map[string]any{"route": map[string]any{"id": record["id"]}, "record": record, "query": map[string]any{"page": 1, "pageSize": binding.MaxItems, "keyword": "", "referenceKeyword": ""}})
			if e != nil {
				return contracts.Output{}, contracts.Problem("invalid")
			}
			data, e := m.providerCall(ctx, app, provider.ID, contracts.Input(input))
			if e != nil {
				return contracts.Output{}, e
			}
			document, e := normalized(data)
			if e != nil {
				return contracts.Output{}, e
			}
			if total, ok := document["total"].(float64); ok && total > float64(binding.MaxItems) {
				semantic.Truncated = true
			}
			var value any = document
			if provider.ResultPath != "" {
				for _, part := range strings.Split(provider.ResultPath, ".") {
					object, ok := value.(map[string]any)
					if !ok {
						return contracts.Output{}, contracts.Problem("invalid")
					}
					value = object[part]
				}
			}
			rows, ok := value.([]any)
			if !ok {
				return contracts.Output{}, contracts.Problem("invalid")
			}
			for index, raw := range rows {
				if index >= binding.MaxItems || len(semantic.References) >= 10 {
					semantic.Truncated = true
					break
				}
				row, ok := raw.(map[string]any)
				if !ok {
					return contracts.Output{}, contracts.Problem("invalid")
				}
				id, ok := row[provider.KeyField]
				if !ok {
					return contracts.Output{}, contracts.Problem("invalid")
				}
				semantic.References = append(semantic.References, ar.RecordReference{EntityRef: binding.EntityRef, RecordID: fmt.Sprint(id)})
			}
		}
	}
	for len(semantic.Fields) > 0 {
		raw, e := json.Marshal(semantic)
		if e != nil {
			return contracts.Output{}, e
		}
		if len(raw) <= 8000 {
			break
		}
		semantic.Fields = semantic.Fields[:len(semantic.Fields)-1]
		semantic.Truncated = true
	}
	semantic.ContextVersion = "2.1"
	semantic.Revision = contracts.Digest(contracts.Input{"metadata": app.MetadataVersion, "record": semantic.RecordID, "fields": semantic.Fields, "selection": semantic.Selection, "references": semantic.References})
	semantic.CapturedAt = time.Now().UTC().Format(time.RFC3339)
	return projection(semantic)
}

func projectProvider(value any, provider ar.Provider, fields []string) (any, error) {
	normalized, err := normalized(value)
	if err != nil {
		return nil, err
	}
	var target any = normalized
	if provider.ResultPath != "" {
		parts := strings.Split(provider.ResultPath, ".")
		for _, part := range parts {
			m, ok := target.(map[string]any)
			if !ok {
				return nil, contracts.Problem("invalid")
			}
			target = m[part]
		}
	}
	keep := map[string]bool{provider.KeyField: true}
	for _, field := range fields {
		keep[field] = true
	}
	project := func(record map[string]any) {
		for field := range record {
			if !keep[field] {
				delete(record, field)
			}
		}
	}
	switch rows := target.(type) {
	case []any:
		for _, row := range rows {
			if record, ok := row.(map[string]any); ok {
				project(record)
			}
		}
	case map[string]any:
		project(rows)
	}
	return normalized, nil
}

// AgentProfile is derived from trusted, enabled contributions, not browser tool claims.
func (m *Module) AgentProfile(ctx context.Context) ([]string, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	out := []string{}
	seen := map[string]string{}
	apps, loadErr := m.publishedCatalog(ctx)
	if loadErr != nil {
		return nil, loadErr
	}
	for _, app := range apps {
		if !p.Can(app.Permission) {
			continue
		}
		row, err := dbgen.New(m.Pool).RuntimeInstallation(ctx, dbgen.RuntimeInstallationParams{TenantID: p.TenantID, PluginID: app.Source})
		state, se := dbgen.New(m.Pool).RuntimeApplication(ctx, dbgen.RuntimeApplicationParams{TenantID: p.TenantID, ApplicationID: app.ID})
		if err != nil || se != nil || row.Status != "enabled" || row.Version != app.PluginVersion || !state.Enabled {
			continue
		}
		for _, ref := range app.AgentTools {
			if version, exists := seen[ref.Name]; exists {
				if version != ref.Version {
					return nil, contracts.Problem("conflict")
				}
				continue
			}
			seen[ref.Name] = ref.Version
			out = append(out, ref.Name+"@"+ref.Version)
		}
	}
	return out, nil
}

// ValidateInvocation enforces optional UI freshness without treating UI context as authority.
func (m *Module) ValidateInvocation(ctx context.Context, ref *contracts.RuntimeReference, definition contracts.Definition, q *dbgen.Queries) error {
	p, err := identity.Current(ctx)
	if err != nil {
		return err
	}
	apps, loadErr := m.publishedCatalogWith(ctx, q, p.TenantID)
	if loadErr != nil {
		return loadErr
	}
	for _, app := range apps {
		if (ref.ApplicationVersion != "" && app.ApplicationVersion != ref.ApplicationVersion) || (ref.DefinitionDigest != "" && app.DefinitionDigest != ref.DefinitionDigest) {
			continue
		}
		if app.ID != ref.ApplicationID || app.MetadataVersion != ref.MetadataVersion || !p.Can(app.Permission) {
			continue
		}
		installed, e := q.RuntimeInstallation(ctx, dbgen.RuntimeInstallationParams{TenantID: p.TenantID, PluginID: app.Source})
		if e != nil || installed.Status != "enabled" || installed.Version != app.PluginVersion || installed.Revision != ref.InstallationRevision {
			return contracts.Problem("conflict")
		}
		state, e := q.RuntimeApplication(ctx, dbgen.RuntimeApplicationParams{TenantID: p.TenantID, ApplicationID: app.ID})
		if e != nil || !state.Enabled {
			return contracts.Problem("unavailable")
		}
		if ref.CatalogRevision > 0 && app.CatalogRevision != ref.CatalogRevision {
			return contracts.Problem("conflict")
		}
		for _, page := range app.Pages {
			if page.ID != ref.PageID || !p.Can(page.Permission) {
				continue
			}
			for _, action := range page.Actions {
				if action.Capability.Name == definition.Name && action.Capability.Version == definition.Version {
					return nil
				}
			}
			for _, form := range page.Forms {
				for _, command := range []ar.Command{form.Create, form.Edit} {
					if command.Capability.Name == definition.Name && command.Capability.Version == definition.Version {
						return nil
					}
				}
			}
		}
	}
	return contracts.Problem("conflict")
}
