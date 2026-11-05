package applicationruntime

import (
	"baseline/services/api/internal/repository/dbgen"
	ar "baseline/services/api/kernel/applicationruntime"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/registry"
	"baseline/services/api/system/identity"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"strconv"
	"strings"
)

func (m *Module) candidates() []ar.Candidate { return ar.DeploymentCandidates(m.contributions()) }
func (m *Module) candidate(id, version string) (ar.Candidate, bool) {
	for _, c := range m.candidates() {
		if c.PluginID == id && (version == "" || c.Version == version) {
			return c, true
		}
	}
	return ar.Candidate{}, false
}
func (m *Module) lifecycleEntries() []registry.Entry {
	out := []registry.Entry{}
	object := func() *contracts.Schema { return &contracts.Schema{Type: "object", AdditionalProperties: true} }
	for _, action := range []string{"list", "get", "check", "install", "enable", "disable", "upgrade", "rollback", "uninstall", "application.enable", "application.disable"} {
		application := strings.HasPrefix(action, "application.")
		name := "application.runtime.installation." + action
		if application {
			name = "application.runtime." + action
		}
		read := action == "list" || action == "get" || action == "check"
		props := map[string]*contracts.Schema{"pluginId": contracts.Str(1, 100), "version": contracts.Str(1, 40), "revision": contracts.Int(1, 1<<53), "applicationId": contracts.Str(1, 100)}
		required := []string{}
		if action != "list" {
			required = append(required, "pluginId")
		}
		if !read && action != "install" {
			required = append(required, "revision")
		}
		if application {
			required = append(required, "applicationId")
		}
		if action == "install" || action == "upgrade" || action == "rollback" || action == "check" {
			required = append(required, "version")
		}
		if action == "list" {
			props = map[string]*contracts.Schema{}
		}
		permission := "applications:manage"
		if read {
			permission = "applications:read"
		}
		d := contracts.Definition{Name: name, Version: "1.0.0", Title: name, Description: "Manage a trusted deployment contribution for the current tenant.", Domain: "runtime", Entity: "runtime_installation", Entities: []string{"runtime_installation", "runtime_application_state", "runtime_installation_history"}, Layer: contracts.LayerAtomic, Permission: permission, Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: read, Input: contracts.Object(props, required...), Output: object(), Executor: "go:" + name, Loggable: []string{"pluginId", "version", "revision", "applicationId"}}
		if action == "list" {
			d.Output = ar.Schema(ar.InstallationCatalog{})
		}
		if action == "check" {
			d.Output = ar.Schema(ar.CompatibilityReport{})
		}
		if !read {
			d.Risk = contracts.RiskMedium
			d.Idempotency = contracts.Idempotency{Required: true, Scope: "tenant+operation+key"}
			d.SuccessEvents = []string{"application.runtime.installation.changed"}
		}
		d.AllowedOrigins = []contracts.Origin{contracts.OriginHumanUI}
		a := action
		h := contracts.Handler{}
		if read {
			h.Execute = func(ctx context.Context, in contracts.Input) (contracts.Output, error) {
				return m.lifecycleRead(ctx, a, in)
			}
		} else {
			h.ExecuteTx = func(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
				return m.lifecycleWrite(ctx, tx, a, in)
			}
		}
		out = append(out, registry.Entry{Definition: d, Handler: h})
	}
	return out
}
func (m *Module) lifecycleRead(ctx context.Context, action string, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(m.Pool)
	id, _ := in["pluginId"].(string)
	version, _ := in["version"].(string)
	if action == "list" {
		rows, err := q.RuntimeInstallations(ctx, p.TenantID)
		if err != nil {
			return contracts.Output{}, err
		}
		result := ar.InstallationCatalog{Items: []ar.Installation{}, Candidates: m.candidates(), Applications: []ar.ApplicationState{}}
		states, e := q.RuntimeApplications(ctx, p.TenantID)
		if e != nil {
			return contracts.Output{}, e
		}
		for _, state := range states {
			result.Applications = append(result.Applications, ar.ApplicationState{ApplicationID: state.ApplicationID, PluginID: state.PluginID, Enabled: state.Enabled, Revision: state.Revision})
		}
		for _, r := range rows {
			result.Items = append(result.Items, ar.Installation{PluginID: r.PluginID, Version: r.Version, Status: r.Status, Revision: r.Revision, Digest: r.Digest, PreviousVersion: r.PreviousVersion})
		}
		return projection(result)
	}
	if action == "check" {
		c, ok := m.candidate(id, version)
		issues := []string{}
		if !ok {
			issues = append(issues, "candidate_unavailable")
		}
		for _, dep := range c.DependencyVersions {
			r, e := q.RuntimeInstallation(ctx, dbgen.RuntimeInstallationParams{TenantID: p.TenantID, PluginID: dep.ID})
			if e != nil || r.Status != "enabled" || r.Version != dep.Version {
				issues = append(issues, "dependency_unavailable")
			}
		}
		for _, dep := range c.Dependencies {
			r, e := q.RuntimeInstallation(ctx, dbgen.RuntimeInstallationParams{TenantID: p.TenantID, PluginID: dep})
			if e != nil || r.Status != "enabled" {
				issues = append(issues, "dependency_unavailable")
			}
		}
		names := m.capabilityNames(id)
		active, e := q.RuntimeActiveReferences(ctx, dbgen.RuntimeActiveReferencesParams{TenantID: p.TenantID, Capabilities: names, PluginID: id})
		if e != nil {
			return contracts.Output{}, e
		}
		if active > 0 {
			issues = append(issues, "active_references")
		}
		return projection(ar.Canonical(ar.CompatibilityReport{Compatible: len(issues) == 0, Issues: issues, Candidate: c}))
	}
	r, err := q.RuntimeInstallation(ctx, dbgen.RuntimeInstallationParams{TenantID: p.TenantID, PluginID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return contracts.Output{}, contracts.Problem("not_found")
	}
	if err != nil {
		return contracts.Output{}, err
	}
	history, err := q.RuntimeHistory(ctx, dbgen.RuntimeHistoryParams{TenantID: p.TenantID, PluginID: id})
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"installation": ar.Installation{PluginID: r.PluginID, Version: r.Version, Status: r.Status, Revision: r.Revision, Digest: r.Digest, PreviousVersion: r.PreviousVersion}, "history": history}}, nil
}
func (m *Module) capabilityNames(id string) []string {
	out := []string{}
	for _, d := range m.Definitions {
		if d.Installation == id {
			out = append(out, d.Name)
		}
	}
	return out
}
func inputInt(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case float64:
		return int64(n)
	}
	return 0
}
func (m *Module) lifecycleWrite(ctx context.Context, tx pgx.Tx, action string, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	if !p.CanPermanent("applications:manage") {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	q := dbgen.New(tx)
	id := in["pluginId"].(string)
	version, _ := in["version"].(string)
	candidate, known := m.candidate(id, version)
	if !known {
		return contracts.Output{}, contracts.Problem("unavailable")
	}
	// Lock the tenant's installations in a stable order to serialize dependencies and activation.
	if err = q.RuntimeTenantLock(ctx, "runtime-installation:"+p.TenantID); err != nil {
		return contracts.Output{}, err
	}
	old, err := q.RuntimeInstallationLock(ctx, dbgen.RuntimeInstallationLockParams{TenantID: p.TenantID, PluginID: id})
	missing := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !missing {
		return contracts.Output{}, err
	}
	if action != "install" && action != "upgrade" && action != "rollback" && !missing {
		candidate, known = m.candidate(id, old.Version)
		if !known {
			return contracts.Output{}, contracts.Problem("unavailable")
		}
	}
	revision := int64(1)
	status := "installed"
	previous := ""
	if action == "install" {
		if !missing {
			revision = old.Revision + 1
		}
		if !missing && old.Status != "uninstalled" {
			return contracts.Output{}, contracts.Problem("conflict")
		}
	} else {
		if missing || old.Status == "uninstalled" || old.Revision != inputInt(in["revision"]) {
			return contracts.Output{}, contracts.Problem("conflict")
		}
		revision = old.Revision + 1
		status = old.Status
		previous = old.PreviousVersion
	}
	if action == "install" || action == "enable" || action == "upgrade" || action == "rollback" || action == "application.enable" {
		for _, dep := range candidate.DependencyVersions {
			r, e := q.RuntimeInstallation(ctx, dbgen.RuntimeInstallationParams{TenantID: p.TenantID, PluginID: dep.ID})
			if e != nil || r.Status != "enabled" || r.Version != dep.Version {
				return contracts.Output{}, contracts.Problem("conflict")
			}
		}
		for _, dep := range candidate.Dependencies {
			r, e := q.RuntimeInstallation(ctx, dbgen.RuntimeInstallationParams{TenantID: p.TenantID, PluginID: dep})
			if e != nil || r.Status != "enabled" {
				return contracts.Output{}, contracts.Problem("conflict")
			}
		}
	}
	if action == "application.disable" {
		for _, dependent := range m.contributions() {
			for _, dep := range dependent.Dependencies {
				if dep != in["applicationId"] {
					continue
				}
				installed, e := q.RuntimeInstallation(ctx, dbgen.RuntimeInstallationParams{TenantID: p.TenantID, PluginID: dependent.Source})
				state, se := q.RuntimeApplication(ctx, dbgen.RuntimeApplicationParams{TenantID: p.TenantID, ApplicationID: dependent.ID})
				if e == nil && se == nil && installed.Status == "enabled" && installed.Version == dependent.PluginVersion && state.Enabled {
					return contracts.Output{}, contracts.Problem("conflict")
				}
			}
		}
	}
	if action == "upgrade" || action == "rollback" {
		for _, dependent := range m.candidates() {
			for _, dep := range dependent.DependencyVersions {
				if dep.ID != id || dep.Version == candidate.Version {
					continue
				}
				installed, e := q.RuntimeInstallation(ctx, dbgen.RuntimeInstallationParams{TenantID: p.TenantID, PluginID: dependent.PluginID})
				if e == nil && installed.Status == "enabled" && installed.Version == dependent.Version {
					return contracts.Output{}, contracts.Problem("conflict")
				}
			}
		}
	}
	switch action {
	case "enable":
		status = "enabled"
	case "disable", "uninstall":
		for _, c := range m.candidates() {
			dependencies := append([]string{}, c.Dependencies...)
			for _, dep := range c.DependencyVersions {
				dependencies = append(dependencies, dep.ID)
			}
			for _, dep := range dependencies {
				if dep == id {
					r, e := q.RuntimeInstallation(ctx, dbgen.RuntimeInstallationParams{TenantID: p.TenantID, PluginID: c.PluginID})
					if e == nil && r.Status == "enabled" && r.Version == c.Version {
						return contracts.Output{}, contracts.Problem("conflict")
					}
				}
			}
		}
		status = "disabled"
		if action == "uninstall" {
			status = "uninstalled"
		}
	case "upgrade", "rollback":
		if version == old.Version || action == "rollback" && version != old.PreviousVersion {
			return contracts.Output{}, contracts.Problem("conflict")
		}
		previous = old.Version
	}
	if action == "upgrade" || action == "rollback" || action == "uninstall" {
		active, e := q.RuntimeActiveReferences(ctx, dbgen.RuntimeActiveReferencesParams{TenantID: p.TenantID, Capabilities: m.capabilityNames(id), PluginID: id})
		if e != nil {
			return contracts.Output{}, e
		}
		if active > 0 {
			return contracts.Output{}, contracts.Problem("conflict")
		}
	}
	if missing {
		err = q.RuntimeInstall(ctx, dbgen.RuntimeInstallParams{TenantID: p.TenantID, PluginID: id, Version: candidate.Version, Digest: candidate.Digest})
	} else {
		var changed int64
		changed, err = q.RuntimeChangeInstallation(ctx, dbgen.RuntimeChangeInstallationParams{TenantID: p.TenantID, PluginID: id, Revision: old.Revision, Version: candidate.Version, Status: status, Digest: candidate.Digest, PreviousVersion: previous})
		if err == nil && changed != 1 {
			return contracts.Output{}, contracts.Problem("conflict")
		}
	}
	if err != nil {
		return contracts.Output{}, err
	}
	for _, app := range candidate.Applications {
		if strings.HasPrefix(action, "application.") && in["applicationId"] != app {
			continue
		}
		if strings.HasPrefix(action, "application.") || action == "install" {
			if err = q.RuntimeSetApplication(ctx, dbgen.RuntimeSetApplicationParams{TenantID: p.TenantID, ApplicationID: app, PluginID: id, Enabled: action != "application.disable"}); err != nil {
				return contracts.Output{}, err
			}
		}
	}
	if strings.HasPrefix(action, "application.") {
		found := false
		for _, a := range candidate.Applications {
			if in["applicationId"] == a {
				found = true
			}
		}
		if !found {
			return contracts.Output{}, contracts.Problem("invalid")
		}
	}
	if err = q.RuntimeRecordHistory(ctx, dbgen.RuntimeRecordHistoryParams{TenantID: p.TenantID, PluginID: id, AccountID: p.Account.ID, Action: action, Version: candidate.Version, Revision: revision}); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: ar.Installation{PluginID: id, Version: candidate.Version, Status: status, Revision: revision, Digest: candidate.Digest, PreviousVersion: previous}, Events: []contracts.Event{{Type: "application.runtime.installation.changed", AggregateType: "runtime_installation", AggregateID: id, Payload: map[string]any{"pluginId": id, "revision": strconv.FormatInt(revision, 10), "action": action}}}}, nil
}
