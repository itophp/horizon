package settings

import (
	gensettings "baseline/services/api/gen/settings"
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/repository/dbgen"
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type Service struct{ *governance.Core }

func New(c *governance.Core) *Service { return &Service{c} }
func apiError(err *error) {
	if *err != nil {
		*err = &gensettings.IAMProblem{Code: governance.Code(*err)}
	}
}
func target(t governance.Target) *gensettings.ConfigTarget {
	return &gensettings.ConfigTarget{Kind: t.Kind, ID: t.ID}
}
func (s *Service) definitions(kind string) []*gensettings.ConfigDefinition {
	out := []*gensettings.ConfigDefinition{}
	for _, d := range s.Definitions {
		if !governance.Has(d.Scopes, kind) {
			continue
		}
		choices := append([]string{}, d.Choices...)
		out = append(out, &gensettings.ConfigDefinition{Key: d.Key, Group: d.Group, Kind: d.Kind, DefaultValue: d.Default, Scopes: d.Scopes, Choices: choices, Sensitive: d.Sensitive, Secret: d.Secret})
	}
	return out
}
func (s *Service) redact(changes []*gensettings.ChangeEntry) []*gensettings.ChangeEntry {
	out := []*gensettings.ChangeEntry{}
	for _, ch := range changes {
		copy := *ch
		if d := s.Definition(ch.Key); d != nil && d.Secret {
			copy.Value = ""
		}
		out = append(out, &copy)
	}
	return out
}
func decodeChanges(raw []byte) ([]*gensettings.ChangeEntry, error) {
	out := []*gensettings.ChangeEntry{}
	err := json.Unmarshal(raw, &out)
	return out, err
}
func (s *Service) draftRecord(ctx context.Context, row dbgen.GovDraftRow) (*gensettings.ChangeRecord, error) {
	t, err := s.ScopeTarget(ctx, tenantFrom(ctx), row.ScopeKey)
	if err != nil {
		return nil, err
	}
	ch, err := decodeChanges(row.Payload)
	if err != nil {
		return nil, err
	}
	return &gensettings.ChangeRecord{ID: row.ID, Target: target(t), Domain: row.Domain, BaseRevision: row.BaseRevision, Status: row.Status, Reason: row.Reason, ActorID: row.ActorID, CreatedAt: governance.Time(row.CreatedAt), Sensitive: row.Sensitive, Changes: s.redact(ch)}, nil
}
func tenantFrom(ctx context.Context) string {
	p, err := identity.Current(ctx)
	if err != nil {
		return ""
	}
	return p.TenantID
}
func (s *Service) versionRecord(ctx context.Context, row dbgen.GovVersionRow) (*gensettings.ChangeRecord, error) {
	t, err := s.ScopeTarget(ctx, tenantFrom(ctx), row.ScopeKey)
	if err != nil {
		return nil, err
	}
	ch, err := decodeChanges(row.Payload)
	if err != nil {
		return nil, err
	}
	return &gensettings.ChangeRecord{ID: row.ID, Target: target(t), Domain: row.Domain, Revision: row.Revision, BaseRevision: row.Revision - 1, Status: "published", Reason: row.Reason, ActorID: row.ActorID, CreatedAt: governance.Time(row.CreatedAt), Sensitive: true, Changes: s.redact(ch)}, nil
}
func (s *Service) entries(values map[string]governance.Value, kind, own string, ui bool) []*gensettings.ConfigEntry {
	out := []*gensettings.ConfigEntry{}
	for _, d := range s.Definitions {
		if ui && !d.UI || !ui && !governance.Has(d.Scopes, kind) {
			continue
		}
		v := values[d.Key]
		raw := v.Raw
		if d.Secret {
			raw = ""
		}
		out = append(out, &gensettings.ConfigEntry{Key: d.Key, Value: raw, Source: v.Source, Overridden: v.Source == own, Configured: v.Configured})
	}
	return out
}
func (s *Service) Workspace(ctx context.Context, payload *gensettings.WorkspacePayload) (res *gensettings.SettingsWorkspace, err error) {
	defer apiError(&err)
	p, t, err := s.Check(ctx, "settings:read", payload.TargetKind, payload.TargetID, true)
	if err != nil {
		return nil, err
	}
	chain, err := s.Chain(ctx, p.TenantID, t, p)
	if err != nil {
		return nil, err
	}
	values, err := s.Values(ctx, p.TenantID, chain)
	if err != nil {
		return nil, err
	}
	rev, err := s.Revision(ctx, p.TenantID, "settings:"+t.Key)
	if err != nil {
		return nil, err
	}
	res = &gensettings.SettingsWorkspace{Target: target(t), Revision: rev, Definitions: s.definitions(t.Kind), Entries: s.entries(values, t.Kind, t.Key, false), Drafts: []*gensettings.ChangeRecord{}, History: []*gensettings.ChangeRecord{}}
	if p.Can("settings:update") {
		_, _, authErr := s.Check(ctx, "settings:update", t.Kind, t.ID, true)
		if authErr == nil {
			rows, e := s.Q.GovDrafts(ctx, dbgen.GovDraftsParams{TenantID: p.TenantID, ResourceKey: "settings:" + t.Key})
			if e != nil {
				return nil, e
			}
			for _, row := range rows {
				r, e := s.draftRecord(ctx, dbgen.GovDraftRow(row))
				if e != nil {
					return nil, e
				}
				res.Drafts = append(res.Drafts, r)
			}
		} else if governance.Code(authErr) != "forbidden" {
			return nil, authErr
		}
	}
	if p.Can("settings:history") {
		_, _, authErr := s.Check(ctx, "settings:history", t.Kind, t.ID, true)
		if authErr == nil {
			rows, e := s.Q.GovVersions(ctx, dbgen.GovVersionsParams{TenantID: p.TenantID, ResourceKey: "settings:" + t.Key})
			if e != nil {
				return nil, e
			}
			for _, row := range rows {
				r, e := s.versionRecord(ctx, dbgen.GovVersionRow(row))
				if e != nil {
					return nil, e
				}
				res.History = append(res.History, r)
			}
		} else if governance.Code(authErr) != "forbidden" {
			return nil, authErr
		}
	}
	return res, nil
}
func (s *Service) effective(ctx context.Context, p *identity.Principal) (*gensettings.EffectiveConfiguration, error) {
	chain, err := s.ContextChain(ctx, p)
	if err != nil {
		return nil, err
	}
	v, err := s.Values(ctx, p.TenantID, chain)
	if err != nil {
		return nil, err
	}
	entries := s.entries(v, "account", governance.TargetKey("account", p.Account.ID), true)
	raw, err := json.Marshal(entries)
	if err != nil {
		return nil, err
	}
	return &gensettings.EffectiveConfiguration{Entries: entries, Revision: governance.Hash(string(raw))}, nil
}
func (s *Service) Effective(ctx context.Context, _ *gensettings.EffectivePayload) (res *gensettings.EffectiveConfiguration, err error) {
	defer apiError(&err)
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	return s.effective(ctx, p)
}
func (s *Service) validate(ctx context.Context, p *identity.Principal, t governance.Target, changes []*gensettings.ChangeEntry, stored bool) ([]*gensettings.ChangeEntry, bool, error) {
	out := []*gensettings.ChangeEntry{}
	sensitive := false
	seen := map[string]bool{}
	for _, ch := range changes {
		d := s.Definition(ch.Key)
		if d == nil || seen[ch.Key] || !governance.Has(d.Scopes, t.Kind) {
			return nil, false, governance.Problem("invalid")
		}
		seen[ch.Key] = true
		sensitive = sensitive || d.Sensitive
		copy := *ch
		if ch.Operation == "reset" {
			copy.Value = ""
			out = append(out, &copy)
			continue
		}
		raw := ch.Value
		if ch.Operation == "clear" {
			raw = `""`
			copy.Operation = "set"
		}
		if d.Secret && stored {
			encoded, e := governance.String(raw)
			if e != nil {
				return nil, false, e
			}
			if encoded != "" {
				raw, e = s.Unseal(p.TenantID, t.Key, d.Key, encoded)
				if e != nil {
					return nil, false, e
				}
			}
		}
		if e := d.Validate(raw); e != nil {
			return nil, false, e
		}
		if d.Key == "branding.logo" {
			id, e := governance.Number(raw)
			if e != nil {
				return nil, false, e
			}
			if id > 0 {
				if s.Logos == nil {
					return nil, false, governance.Problem("unavailable")
				}
				if e = s.Logos.ValidateLogo(ctx, p, id, t); e != nil {
					return nil, false, e
				}
			}
		}
		copy.Value = raw
		if d.Secret && raw != `""` {
			encoded, e := s.Seal(p.TenantID, t.Key, d.Key, raw)
			if e != nil {
				return nil, false, e
			}
			j, e := json.Marshal(encoded)
			if e != nil {
				return nil, false, e
			}
			copy.Value = string(j)
		}
		out = append(out, &copy)
	}
	return out, sensitive, nil
}
func (s *Service) combined(ctx context.Context, q *dbgen.Queries, p *identity.Principal, t governance.Target, changes []*gensettings.ChangeEntry) (map[string]governance.Value, error) {
	chain, err := s.ChainWith(ctx, q, p.TenantID, t, p)
	if err != nil {
		return nil, err
	}
	baseChain := chain[:len(chain)-1]
	if t.Kind == "workspace" {
		baseChain = []string{}
	}
	base, err := s.ValuesWith(ctx, q, p.TenantID, baseChain)
	if err != nil {
		return nil, err
	}
	rows, err := q.GovValues(ctx, dbgen.GovValuesParams{TenantID: p.TenantID, Column2: []string{t.Key}})
	if err != nil {
		return nil, err
	}
	current := map[string]string{}
	for _, r := range rows {
		current[r.ConfigKey] = string(r.ValueJson)
	}
	for _, ch := range changes {
		if ch.Operation == "reset" {
			delete(current, ch.Key)
		} else {
			current[ch.Key] = ch.Value
		}
	}
	for key, raw := range current {
		if key == "files.maxBytes" || key == "files.logoMaxBytes" {
			parent, e := governance.Number(base[key].Raw)
			if e != nil {
				return nil, e
			}
			next, e := governance.Number(raw)
			if e != nil || next > parent {
				return nil, governance.Problem("invalid")
			}
		}
		if key == "files.extensions" {
			var next, allowed []string
			if e := json.Unmarshal([]byte(raw), &next); e != nil {
				return nil, e
			}
			if e := json.Unmarshal([]byte(base[key].Raw), &allowed); e != nil {
				return nil, e
			}
			for _, v := range next {
				if !governance.Has(allowed, v) {
					return nil, governance.Problem("invalid")
				}
			}
		}
		base[key] = governance.Value{Raw: raw, Source: t.Key, Configured: raw != `""`}
	}
	return base, s.ValidateCombined(base)
}
func (s *Service) Draft(ctx context.Context, payload *gensettings.DraftPayload) (res *gensettings.ChangeRecord, err error) {
	defer apiError(&err)
	p, t, err := s.Check(ctx, "settings:update", payload.Target.Kind, payload.Target.ID, true)
	if err != nil {
		return nil, err
	}
	changes, sensitive, err := s.validate(ctx, p, t, payload.Changes, false)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(changes)
	if err != nil {
		return nil, err
	}
	var id int64
	err = s.Transaction(ctx, func(q *dbgen.Queries) error {
		if e := s.Lock(ctx, q, p, t, "settings:"+t.Key, payload.BaseRevision); e != nil {
			return e
		}
		if _, e := s.combined(ctx, q, p, t, changes); e != nil {
			return e
		}
		var e error
		id, e = q.GovCreateDraft(ctx, dbgen.GovCreateDraftParams{TenantID: p.TenantID, Domain: "settings", ResourceKey: "settings:" + t.Key, ScopeKey: t.Key, BaseRevision: payload.BaseRevision, Payload: raw, Reason: strings.TrimSpace(payload.Reason), Sensitive: sensitive, ActorID: p.Account.ID})
		return e
	})
	if err != nil {
		return nil, err
	}
	row, err := s.Core.Draft(ctx, "settings:update", id)
	if err != nil {
		return nil, err
	}
	return s.draftRecord(ctx, row)
}
func (s *Service) write(ctx context.Context, q *dbgen.Queries, p *identity.Principal, t governance.Target, changes []*gensettings.ChangeEntry, base int64, reason string) (int64, error) {
	for _, ch := range changes {
		var err error
		if ch.Operation == "reset" {
			err = q.GovResetValue(ctx, dbgen.GovResetValueParams{TenantID: p.TenantID, ScopeKey: t.Key, ConfigKey: ch.Key})
		} else {
			err = q.GovSetValue(ctx, dbgen.GovSetValueParams{TenantID: p.TenantID, ScopeKey: t.Key, ConfigKey: ch.Key, ValueJson: []byte(ch.Value)})
		}
		if err != nil {
			return 0, err
		}
	}
	rows, err := q.GovValues(ctx, dbgen.GovValuesParams{TenantID: p.TenantID, Column2: []string{t.Key}})
	if err != nil {
		return 0, err
	}
	snapshot := []*gensettings.ChangeEntry{}
	for _, row := range rows {
		snapshot = append(snapshot, &gensettings.ChangeEntry{Key: row.ConfigKey, Operation: "set", Value: string(row.ValueJson)})
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return 0, err
	}
	detail, err := json.Marshal(s.redact(changes))
	if err != nil {
		return 0, err
	}
	if t.Kind == "workspace" {
		values, e := s.ValuesWith(ctx, q, p.TenantID, []string{t.Key})
		if e != nil {
			return 0, e
		}
		maximum, e := governance.Number(values["security.sessionMinutes"].Raw)
		if e != nil {
			return 0, e
		}
		idle, e := governance.Number(values["security.idleMinutes"].Raw)
		if e != nil {
			return 0, e
		}
		if e = q.GovClampSessions(ctx, dbgen.GovClampSessionsParams{TenantID: p.TenantID, MaximumMinutes: int32(maximum)}); e != nil {
			return 0, e
		}
		if e = q.GovExpireIdleSessions(ctx, dbgen.GovExpireIdleSessionsParams{TenantID: p.TenantID, IdleMinutes: int32(idle)}); e != nil {
			return 0, e
		}
	}
	id, err := s.CommitVersion(ctx, q, p, t, "settings", "settings:"+t.Key, 0, base, raw, reason, detail)
	if err != nil {
		return 0, err
	}
	for _, ch := range changes {
		if ch.Key == "branding.logo" && ch.Operation != "reset" {
			file, e := governance.Number(ch.Value)
			if e != nil {
				return 0, e
			}
			if file > 0 {
				if e = q.GovBindFile(ctx, dbgen.GovBindFileParams{TenantID: p.TenantID, FileID: file, ScopeKey: t.Key, Revision: base + 1}); e != nil {
					return 0, e
				}
			}
		}
	}
	return id, nil
}
func (s *Service) Publish(ctx context.Context, payload *gensettings.PublishPayload) (res *gensettings.ChangeRecord, err error) {
	defer apiError(&err)
	row, err := s.Core.Draft(ctx, "settings:publish", payload.ID)
	if err != nil {
		return nil, err
	}
	if row.Domain != "settings" || row.Status != "draft" || row.BaseRevision != payload.BaseRevision {
		return nil, governance.Problem("conflict")
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	t, err := s.ScopeTarget(ctx, p.TenantID, row.ScopeKey)
	if err != nil {
		return nil, err
	}
	rawChanges, err := decodeChanges(row.Payload)
	if err != nil {
		return nil, err
	}
	changes, sensitive, err := s.validate(ctx, p, t, rawChanges, true)
	if err != nil {
		return nil, err
	}
	row.Sensitive = row.Sensitive || sensitive
	var id int64
	err = s.Transaction(ctx, func(q *dbgen.Queries) error {
		var e error
		id, e = s.publishWithin(ctx, q, p, t, row, changes, payload.BaseRevision, payload.Proof, false)
		return e
	})
	if err != nil {
		return nil, err
	}
	v, err := s.Version(ctx, "settings:publish", id)
	if err != nil {
		return nil, err
	}
	return s.versionRecord(ctx, v)
}
func (s *Service) Rollback(ctx context.Context, payload *gensettings.RollbackPayload) (res *gensettings.ChangeRecord, err error) {
	defer apiError(&err)
	row, err := s.Version(ctx, "settings:rollback", payload.ID)
	if err != nil {
		return nil, err
	}
	if row.Domain != "settings" {
		return nil, governance.Problem("invalid")
	}
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	t, err := s.ScopeTarget(ctx, p.TenantID, row.ScopeKey)
	if err != nil {
		return nil, err
	}
	snapshot, err := decodeChanges(row.Payload)
	if err != nil {
		return nil, err
	}
	changes, _, err := s.validate(ctx, p, t, snapshot, true)
	if err != nil {
		return nil, err
	}
	present := map[string]bool{}
	for _, ch := range changes {
		present[ch.Key] = true
	}
	rows, err := s.Q.GovValues(ctx, dbgen.GovValuesParams{TenantID: p.TenantID, Column2: []string{t.Key}})
	if err != nil {
		return nil, err
	}
	for _, v := range rows {
		if !present[v.ConfigKey] {
			changes = append(changes, &gensettings.ChangeEntry{Key: v.ConfigKey, Operation: "reset", Value: ""})
		}
	}
	var id int64
	err = s.Transaction(ctx, func(q *dbgen.Queries) error {
		if e := s.Lock(ctx, q, p, t, row.ResourceKey, payload.BaseRevision); e != nil {
			return e
		}
		if values, e := s.combined(ctx, q, p, t, changes); e != nil {
			return e
		} else if e = s.validateStorage(ctx, p, values, changes); e != nil {
			return e
		}
		if e := s.Consume(ctx, q, p, payload.Proof, "settings.rollback", payload.ID, payload.BaseRevision); e != nil {
			return e
		}
		var e error
		id, e = s.write(ctx, q, p, t, changes, payload.BaseRevision, payload.Reason)
		return e
	})
	if err != nil {
		return nil, err
	}
	v, err := s.Version(ctx, "settings:rollback", id)
	if err != nil {
		return nil, err
	}
	return s.versionRecord(ctx, v)
}
func (s *Service) Preferences(ctx context.Context, payload *gensettings.PreferencesPayload) (res *gensettings.EffectiveConfiguration, err error) {
	defer apiError(&err)
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	t, err := s.Resolve(ctx, p.TenantID, "account", p.Account.ID)
	if err != nil {
		return nil, err
	}
	changes, _, err := s.validate(ctx, p, t, payload.Changes, false)
	if err != nil {
		return nil, err
	}
	base, err := s.Revision(ctx, p.TenantID, "settings:"+t.Key)
	if err != nil {
		return nil, err
	}
	err = s.Transaction(ctx, func(q *dbgen.Queries) error {
		if e := s.Lock(ctx, q, p, t, "settings:"+t.Key, base); e != nil {
			return e
		}
		_, e := s.write(ctx, q, p, t, changes, base, "preferences.update")
		return e
	})
	if err != nil {
		return nil, err
	}
	return s.effective(ctx, p)
}
func (s *Service) Export(ctx context.Context, payload *gensettings.ExportPayload) (res []*gensettings.ChangeEntry, err error) {
	defer apiError(&err)
	p, t, err := s.Check(ctx, "settings:export", payload.TargetKind, payload.TargetID, true)
	if err != nil {
		return nil, err
	}
	chain, err := s.Chain(ctx, p.TenantID, t, p)
	if err != nil {
		return nil, err
	}
	values, err := s.Values(ctx, p.TenantID, chain)
	if err != nil {
		return nil, err
	}
	res = []*gensettings.ChangeEntry{}
	for _, d := range s.Definitions {
		if !d.Secret && governance.Has(d.Scopes, t.Kind) {
			res = append(res, &gensettings.ChangeEntry{Key: d.Key, Operation: "set", Value: values[d.Key].Raw})
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Key < res[j].Key })
	return res, nil
}
func (s *Service) Reauthenticate(ctx context.Context, payload *gensettings.ReauthenticatePayload) (res *gensettings.ReauthProof, err error) {
	defer apiError(&err)
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	var resource string
	switch payload.Operation {
	case "settings.publish", "settings.schedule":
		r, e := s.Core.Draft(ctx, "settings:publish", payload.ResourceID)
		if e != nil {
			return nil, e
		}
		if r.Domain != "settings" || r.Status != "draft" {
			return nil, governance.Problem("invalid")
		}
		resource = r.ResourceKey
	case "settings.rollback", "dictionaries.rollback":
		permission := "settings:rollback"
		if payload.Operation == "dictionaries.rollback" {
			permission = "dictionaries:rollback"
		}
		r, e := s.Version(ctx, permission, payload.ResourceID)
		if e != nil {
			return nil, e
		}
		if payload.Operation == "settings.rollback" && r.Domain != "settings" || payload.Operation == "dictionaries.rollback" && r.Domain == "settings" {
			return nil, governance.Problem("invalid")
		}
		resource = r.ResourceKey
	default:
		return nil, governance.Problem("invalid")
	}
	revision, err := s.Revision(ctx, p.TenantID, resource)
	if err != nil {
		return nil, err
	}
	if revision != payload.Revision {
		return nil, governance.Problem("conflict")
	}
	if err = s.Identity.VerifyCurrentPassword(ctx, p, payload.Password); err != nil {
		return nil, err
	}
	token, err := governance.Token()
	if err != nil {
		return nil, err
	}
	err = s.Q.GovCreateProof(ctx, dbgen.GovCreateProofParams{TokenHash: governance.Hash(token), TenantID: p.TenantID, SessionHash: p.TokenHash, DepartmentID: p.Account.DepartmentID, Operation: payload.Operation, ResourceID: payload.ResourceID, Revision: payload.Revision})
	if err != nil {
		return nil, err
	}
	return &gensettings.ReauthProof{Proof: token, ExpiresAt: time.Now().UTC().Add(5 * time.Minute).Format(time.RFC3339)}, nil
}

func (s *Service) validateStorage(ctx context.Context, p *identity.Principal, values map[string]governance.Value, changes []*gensettings.ChangeEntry) error {
	for _, change := range changes {
		if strings.HasPrefix(change.Key, "storage.") {
			if s.Storage == nil {
				return governance.Problem("unavailable")
			}
			return s.Storage.ValidateStorage(ctx, p.TenantID, values)
		}
	}
	return nil
}

func (s *Service) Logo(ctx context.Context, _ *gensettings.LogoPayload) (res *gensettings.FileContent, err error) {
	defer apiError(&err)
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	if s.Logos == nil {
		return nil, governance.Problem("unavailable")
	}
	name, mime, data, err := s.Logos.ContextLogo(ctx, p)
	if err != nil {
		return nil, err
	}
	return &gensettings.FileContent{Name: name, Mime: mime, Content: data}, nil
}
