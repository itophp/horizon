package dictionaries

import (
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/repository/dbgen"
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Service struct{ *governance.Core }

func New(c *governance.Core) *Service { return &Service{c} }
func apiError(err *error) {
	if *err != nil {
		*err = governance.Problem(governance.Code(*err))
	}
}

type draftBody struct {
	Document   *DictionaryDocument
	Overrides  []*DictionaryOverride
	Extensions []*DictionaryItem
}

func normalizeDocument(doc *DictionaryDocument) {
	if doc == nil || doc.Definition == nil {
		return
	}
	doc.Definition.Target = &ConfigTarget{Kind: "workspace", ID: 0}
	doc.Definition.AllowExtensions = false
	for _, item := range doc.Items {
		item.Owner = &ConfigTarget{Kind: "workspace", ID: 0}
	}
}

func owner(key string) (*ConfigTarget, error) {
	parts := strings.SplitN(key, ":", 2)
	if len(parts) != 2 {
		return nil, governance.Problem("internal")
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, err
	}
	return &ConfigTarget{Kind: parts[0], ID: id}, nil
}
func def(row dbgen.GovDictionaryRow) (*DictionaryDefinition, error) {
	var d DictionaryDefinition
	if err := json.Unmarshal(row.Spec, &d); err != nil {
		return nil, err
	}
	d.ID = row.ID
	d.Revision = row.Revision
	t, err := owner(row.ScopeKey)
	if err != nil {
		return nil, err
	}
	d.Target = t
	d.AllowExtensions = false
	return &d, nil
}
func (s *Service) readContext(ctx context.Context, permission, kind string, id int64) (*identity.Principal, governance.Target, []string, bool, error) {
	if kind == "account" {
		return nil, governance.Target{}, nil, false, governance.Problem("invalid")
	}
	p, t, err := s.Check(ctx, permission, kind, id, true)
	if err != nil {
		return nil, t, nil, false, err
	}
	chain, err := s.Chain(ctx, p.TenantID, t, p)
	if err != nil {
		return nil, t, nil, false, err
	}
	scope, err := s.Identity.ScopeFor(ctx, p, permission, true)
	return p, t, chain, scope.All, err
}
func (s *Service) List(ctx context.Context, payload *ListPayload) (res *DictionaryWorkspace, err error) {
	defer apiError(&err)
	p, t, chain, all, err := s.readContext(ctx, "dictionaries:read", payload.TargetKind, payload.TargetID)
	if err != nil {
		return nil, err
	}
	rows, err := s.Q.GovDictionaryList(ctx, dbgen.GovDictionaryListParams{TenantID: p.TenantID, Column2: chain, ViewScope: t.Key, AllData: all})
	if err != nil {
		return nil, err
	}
	res = &DictionaryWorkspace{Target: &ConfigTarget{Kind: t.Kind, ID: t.ID}, Definitions: []*DictionaryDefinition{}}
	for _, row := range rows {
		d, e := def(dbgen.GovDictionaryRow(row))
		if e != nil {
			return nil, e
		}
		res.Definitions = append(res.Definitions, d)
	}
	return res, nil
}
func (s *Service) document(ctx context.Context, q *dbgen.Queries, tenant string, row dbgen.GovDictionaryRow, chain []string) (*DictionaryDocument, error) {
	d, err := def(row)
	if err != nil {
		return nil, err
	}
	rows, err := q.GovDictionaryItems(ctx, dbgen.GovDictionaryItemsParams{TenantID: tenant, DictionaryID: row.ID, Column3: chain})
	if err != nil {
		return nil, err
	}
	doc := &DictionaryDocument{Definition: d, Items: []*DictionaryItem{}}
	for _, row := range rows {
		var item DictionaryItem
		if err = json.Unmarshal(row.Spec, &item); err != nil {
			return nil, err
		}
		item.ID = row.ID
		item.Code = row.Code
		item.ParentCode = row.ParentCode
		item.Owner, err = owner(row.ScopeKey)
		if err != nil {
			return nil, err
		}
		doc.Items = append(doc.Items, &item)
	}
	return doc, nil
}
func (s *Service) change(ctx context.Context, tenant string, id int64, domain, key string, rev, base, actor int64, status, reason, created string, payload []byte) (*ChangeRecord, error) {
	var body draftBody
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, err
	}
	normalizeDocument(body.Document)
	t, err := owner(key)
	if err != nil {
		return nil, err
	}
	return &ChangeRecord{ID: id, Domain: domain, Target: t, Revision: rev, BaseRevision: base, Status: status, Reason: reason, ActorID: actor, CreatedAt: created, Sensitive: status == "published", Changes: []*ChangeEntry{}, Document: body.Document, Overrides: body.Overrides}, nil
}
func (s *Service) recordDraft(ctx context.Context, row dbgen.GovDraftRow) (*ChangeRecord, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	return s.change(ctx, p.TenantID, row.ID, row.Domain, row.ScopeKey, 0, row.BaseRevision, row.ActorID, row.Status, row.Reason, governance.Time(row.CreatedAt), row.Payload)
}
func (s *Service) recordVersion(ctx context.Context, row dbgen.GovVersionRow) (*ChangeRecord, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	return s.change(ctx, p.TenantID, row.ID, row.Domain, row.ScopeKey, row.Revision, row.Revision-1, row.ActorID, "published", row.Reason, governance.Time(row.CreatedAt), row.Payload)
}
func dictionaryKey(id int64) string { return "dictionary:" + strconv.FormatInt(id, 10) }
func overlayKey(id int64, target string) string {
	return "override:" + strconv.FormatInt(id, 10) + ":" + target
}
func (s *Service) Get(ctx context.Context, payload *GetPayload) (res *DictionaryDetail, err error) {
	defer apiError(&err)
	p, t, chain, all, err := s.readContext(ctx, "dictionaries:read", payload.TargetKind, payload.TargetID)
	if err != nil {
		return nil, err
	}
	row, err := s.Q.GovDictionary(ctx, dbgen.GovDictionaryParams{TenantID: p.TenantID, ID: payload.ID, Column3: chain, ViewScope: t.Key, AllData: all})
	if err != nil {
		return nil, err
	}
	doc, err := s.document(ctx, s.Q, p.TenantID, row, chain)
	if err != nil {
		return nil, err
	}
	res = &DictionaryDetail{Document: doc, Drafts: []*ChangeRecord{}, History: []*ChangeRecord{}, Overrides: []*DictionaryOverride{}}
	overrides, err := s.Q.GovItemOverrides(ctx, dbgen.GovItemOverridesParams{TenantID: p.TenantID, DictionaryID: row.ID, Column3: []string{t.Key}})
	if err != nil {
		return nil, err
	}
	for _, o := range overrides {
		var item DictionaryOverride
		if err = json.Unmarshal(o.Spec, &item); err != nil {
			return nil, err
		}
		res.Overrides = append(res.Overrides, &item)
	}
	res.OverrideRevision, err = s.Revision(ctx, p.TenantID, overlayKey(row.ID, t.Key))
	if err != nil {
		return nil, err
	}
	for _, resource := range []string{dictionaryKey(row.ID), overlayKey(row.ID, t.Key)} {
		kind, id := t.Kind, t.ID
		if resource == dictionaryKey(row.ID) {
			kind, id = doc.Definition.Target.Kind, doc.Definition.Target.ID
		}
		if p.Can("dictionaries:update") {
			_, _, e := s.Check(ctx, "dictionaries:update", kind, id, true)
			if e == nil {
				drafts, e := s.Q.GovDrafts(ctx, dbgen.GovDraftsParams{TenantID: p.TenantID, ResourceKey: resource})
				if e != nil {
					return nil, e
				}
				for _, d := range drafts {
					record, e := s.recordDraft(ctx, dbgen.GovDraftRow(d))
					if e != nil {
						return nil, e
					}
					res.Drafts = append(res.Drafts, record)
				}
			} else if governance.Code(e) != "forbidden" {
				return nil, e
			}
		}
		if p.Can("dictionaries:history") {
			_, _, e := s.Check(ctx, "dictionaries:history", kind, id, true)
			if e == nil {
				versions, e := s.Q.GovVersions(ctx, dbgen.GovVersionsParams{TenantID: p.TenantID, ResourceKey: resource})
				if e != nil {
					return nil, e
				}
				for _, v := range versions {
					record, e := s.recordVersion(ctx, dbgen.GovVersionRow(v))
					if e != nil {
						return nil, e
					}
					res.History = append(res.History, record)
				}
			} else if governance.Code(e) != "forbidden" {
				return nil, e
			}
		}
	}
	if _, _, e := s.Check(ctx, "dictionaries:read", doc.Definition.Target.Kind, doc.Definition.Target.ID, true); e == nil {
		res.References, err = s.Q.GovDictionaryReferences(ctx, dbgen.GovDictionaryReferencesParams{TenantID: p.TenantID, DictionaryID: row.ID})
	} else if governance.Code(e) != "forbidden" {
		return nil, e
	}
	return res, err
}

var codePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,80}$`)
var colorPattern = regexp.MustCompile(`^(|#[0-9a-fA-F]{3}|#[0-9a-fA-F]{6}|neutral|green|red|amber|blue)$`)

func validItem(item *DictionaryItem) error {
	if item == nil || !codePattern.MatchString(item.Code) || strings.TrimSpace(item.LabelEn) == "" || len([]rune(item.LabelEn)) > 200 || len([]rune(item.LabelZh)) > 200 || !governance.Has([]string{"active", "inactive", "archived"}, item.Status) || !colorPattern.MatchString(item.Color) || len(item.Icon) > 40 {
		return governance.Problem("invalid")
	}
	var start, end time.Time
	var err error
	if item.StartsAt != "" {
		start, err = time.Parse(time.RFC3339, item.StartsAt)
		if err != nil {
			return governance.Problem("invalid")
		}
	}
	if item.EndsAt != "" {
		end, err = time.Parse(time.RFC3339, item.EndsAt)
		if err != nil || !start.IsZero() && !end.After(start) {
			return governance.Problem("invalid")
		}
	}
	return nil
}
func validateTree(items []*DictionaryItem, kind string) error {
	byCode := map[string]*DictionaryItem{}
	defaults := 0
	for _, item := range items {
		if err := validItem(item); err != nil {
			return err
		}
		if byCode[item.Code] != nil {
			return governance.Problem("invalid")
		}
		byCode[item.Code] = item
		if item.IsDefault && item.Status == "active" {
			defaults++
		}
		if kind == "flat" && item.ParentCode != "" {
			return governance.Problem("invalid")
		}
	}
	if defaults > 1 {
		return governance.Problem("invalid")
	}
	for _, item := range items {
		seen := map[string]bool{}
		node := item
		for depth := 0; node != nil && node.ParentCode != ""; depth++ {
			if depth >= 16 || seen[node.Code] {
				return governance.Problem("invalid")
			}
			seen[node.Code] = true
			parent, ok := byCode[node.ParentCode]
			if !ok {
				return governance.Problem("invalid")
			}
			node = parent
		}
	}
	return nil
}
func (s *Service) validateDocument(ctx context.Context, p *identity.Principal, t governance.Target, doc *DictionaryDocument, existing *DictionaryDocument) error {
	d := doc.Definition
	if d.AllowExtensions {
		return governance.Problem("invalid")
	}
	if d.Target.Kind != t.Kind || d.Target.ID != t.ID || t.Kind == "account" || strings.HasPrefix(d.Code, "system.") || strings.HasPrefix(d.Code, "iam.") {
		return governance.Problem("invalid")
	}
	if d.Kind == "dynamic" {
		if !governance.Has([]string{"organizations", "departments", "roles", "accounts"}, d.Provider) || len(doc.Items) > 0 || d.AllowExtensions {
			return governance.Problem("invalid")
		}
	} else if d.Provider != "" {
		return governance.Problem("invalid")
	}
	if d.ConsumerPermission != "" {
		known := false
		for _, x := range identity.Permissions {
			known = known || x == d.ConsumerPermission
		}
		if !known {
			return governance.Problem("invalid")
		}
	}
	if len(doc.Items) > 10000 {
		return governance.Problem("invalid")
	}
	if existing != nil {
		old := existing.Definition
		if d.Code != old.Code || d.Target.Kind != old.Target.Kind || d.Target.ID != old.Target.ID || old.Revision > 0 && (d.Kind != old.Kind || d.Provider != old.Provider) {
			return governance.Problem("conflict")
		}
		newCodes := map[string]*DictionaryItem{}
		for _, item := range doc.Items {
			newCodes[item.Code] = item
		}
		for _, item := range existing.Items {
			n := newCodes[item.Code]
			if n == nil || n.ID != item.ID {
				return governance.Problem("conflict")
			}
		}
	}
	return validateTree(doc.Items, d.Kind)
}
func (s *Service) Draft(ctx context.Context, payload *DraftPayload) (res *ChangeRecord, err error) {
	defer apiError(&err)
	return s.draft(ctx, payload, "")
}
func (s *Service) draft(ctx context.Context, payload *DraftPayload, requested string) (res *ChangeRecord, err error) {
	d := payload.Document.Definition
	permission := "dictionaries:update"
	if d.ID == 0 {
		permission = "dictionaries:create"
	}
	if requested != "" {
		permission = requested
	}
	p, t, err := s.Check(ctx, permission, d.Target.Kind, d.Target.ID, true)
	if err != nil {
		return nil, err
	}
	var existing *DictionaryDocument
	if d.ID != 0 {
		row, e := s.Q.GovDictionary(ctx, dbgen.GovDictionaryParams{TenantID: p.TenantID, ID: d.ID, Column3: []string{t.Key}, ViewScope: t.Key, AllData: true})
		if e != nil {
			return nil, e
		}
		existing, e = s.document(ctx, s.Q, p.TenantID, row, []string{t.Key})
		if e != nil {
			return nil, e
		}
	}
	if err = s.validateDocument(ctx, p, t, payload.Document, existing); err != nil {
		return nil, err
	}
	var draftID int64
	err = s.Transaction(ctx, func(q *dbgen.Queries) error {
		if e := s.EnsureScope(ctx, q, p.TenantID, t); e != nil {
			return e
		}
		if d.ID == 0 {
			raw, e := json.Marshal(d)
			if e != nil {
				return e
			}
			d.ID, e = q.GovCreateDictionary(ctx, dbgen.GovCreateDictionaryParams{TenantID: p.TenantID, ScopeKey: t.Key, Code: d.Code, Spec: raw})
			if e != nil {
				return e
			}
		}
		key := dictionaryKey(d.ID)
		if e := s.Lock(ctx, q, p, t, key, payload.BaseRevision); e != nil {
			return e
		}
		raw, e := json.Marshal(draftBody{Document: payload.Document, Overrides: []*DictionaryOverride{}, Extensions: []*DictionaryItem{}})
		if e != nil {
			return e
		}
		draftID, e = q.GovCreateDraft(ctx, dbgen.GovCreateDraftParams{TenantID: p.TenantID, Domain: "dictionary", ResourceKey: key, ScopeKey: t.Key, EntityID: d.ID, BaseRevision: payload.BaseRevision, Payload: raw, Reason: payload.Reason, ActorID: p.Account.ID})
		return e
	})
	if err != nil {
		return nil, err
	}
	row, err := s.Core.Draft(ctx, permission, draftID)
	if err != nil {
		return nil, err
	}
	return s.recordDraft(ctx, row)
}
func (s *Service) saveItems(ctx context.Context, q *dbgen.Queries, p *identity.Principal, t governance.Target, id int64, items []*DictionaryItem) error {
	for _, item := range items {
		raw, err := json.Marshal(item)
		if err != nil {
			return err
		}
		itemID, err := q.GovUpsertItem(ctx, dbgen.GovUpsertItemParams{TenantID: p.TenantID, DictionaryID: id, ScopeKey: t.Key, Code: item.Code, ParentCode: item.ParentCode, Spec: raw})
		if err != nil {
			return err
		}
		item.ID = itemID
		item.Owner = &ConfigTarget{Kind: t.Kind, ID: t.ID}
	}
	return nil
}
func (s *Service) validateOverlay(ctx context.Context, q *dbgen.Queries, p *identity.Principal, t governance.Target, id int64, body draftBody) (*DictionaryDocument, error) {
	if t.Kind == "account" {
		return nil, governance.Problem("invalid")
	}
	chain, err := s.ChainWith(ctx, q, p.TenantID, t, p)
	if err != nil {
		return nil, err
	}
	row, err := q.GovDictionary(ctx, dbgen.GovDictionaryParams{TenantID: p.TenantID, ID: id, Column3: chain, ViewScope: t.Key, AllData: true})
	if err != nil {
		return nil, err
	}
	if row.ScopeKey == t.Key {
		return nil, governance.Problem("invalid")
	}
	doc, err := s.document(ctx, q, p.TenantID, row, chain)
	if err != nil {
		return nil, err
	}
	if doc.Definition.Kind == "dynamic" {
		return nil, governance.Problem("invalid")
	}
	byID := map[int64]*DictionaryItem{}
	codes := map[string]bool{}
	for _, item := range doc.Items {
		byID[item.ID] = item
		codes[item.Code] = true
	}
	seen := map[int64]bool{}
	for _, o := range body.Overrides {
		if o == nil || seen[o.ItemID] || byID[o.ItemID] == nil {
			return nil, governance.Problem("invalid")
		}
		seen[o.ItemID] = true
		for _, f := range o.Fields {
			if !governance.Has([]string{"labelEn", "labelZh", "sort", "status", "isDefault"}, f) {
				return nil, governance.Problem("invalid")
			}
		}
	}
	combined := append([]*DictionaryItem{}, doc.Items...)
	for _, item := range body.Extensions {
		if !doc.Definition.AllowExtensions {
			return nil, governance.Problem("forbidden")
		}
		if codes[item.Code] {
			old := byID[item.ID]
			if old == nil || old.Owner.Kind != t.Kind || old.Owner.ID != t.ID || old.Code != item.Code {
				return nil, governance.Problem("conflict")
			}
			for i, x := range combined {
				if x.ID == item.ID {
					combined[i] = item
				}
			}
		} else {
			if item.ID != 0 {
				return nil, governance.Problem("invalid")
			}
			combined = append(combined, item)
		}
		codes[item.Code] = true
	}
	overrides, e := q.GovItemOverrides(ctx, dbgen.GovItemOverridesParams{TenantID: p.TenantID, DictionaryID: id, Column3: chain})
	if e != nil {
		return nil, e
	}
	pending := map[int64]*DictionaryOverride{}
	for _, o := range body.Overrides {
		pending[o.ItemID] = o
	}
	for _, key := range chain {
		for _, stored := range overrides {
			if stored.ScopeKey != key {
				continue
			}
			if key == t.Key && pending[stored.ItemID] != nil {
				continue
			}
			var o DictionaryOverride
			if e = json.Unmarshal(stored.Spec, &o); e != nil {
				return nil, e
			}
			if item := byID[o.ItemID]; item != nil {
				applyOverride(item, &o)
			}
		}
	}
	for _, o := range body.Overrides {
		if !o.Reset {
			applyOverride(byID[o.ItemID], o)
		}
	}
	if err = validateTree(combined, doc.Definition.Kind); err != nil {
		return nil, err
	}
	return doc, nil
}
func (s *Service) Override(ctx context.Context, _ *OverridePayload) (res *ChangeRecord, err error) {
	defer apiError(&err)
	if _, _, err = s.Identity.Authorize(ctx, "dictionaries:update", true); err != nil {
		return nil, err
	}
	return nil, governance.Problem("invalid")
}
func (s *Service) apply(ctx context.Context, q *dbgen.Queries, p *identity.Principal, t governance.Target, domain string, entity, base int64, body *draftBody, reason string) (int64, error) {
	normalizeDocument(body.Document)
	key := dictionaryKey(entity)
	if domain == "override" {
		key = overlayKey(entity, t.Key)
		if _, err := s.validateOverlay(ctx, q, p, t, entity, *body); err != nil {
			return 0, err
		}
		if err := s.saveItems(ctx, q, p, t, entity, body.Extensions); err != nil {
			return 0, err
		}
		for _, o := range body.Overrides {
			var err error
			if o.Reset {
				err = q.GovResetItemOverride(ctx, dbgen.GovResetItemOverrideParams{TenantID: p.TenantID, DictionaryID: entity, ItemID: o.ItemID, ScopeKey: t.Key})
			} else {
				raw, e := json.Marshal(o)
				if e != nil {
					return 0, e
				}
				err = q.GovSetItemOverride(ctx, dbgen.GovSetItemOverrideParams{TenantID: p.TenantID, DictionaryID: entity, ItemID: o.ItemID, ScopeKey: t.Key, Spec: raw})
			}
			if err != nil {
				return 0, err
			}
		}
	} else {
		row, err := q.GovDictionary(ctx, dbgen.GovDictionaryParams{TenantID: p.TenantID, ID: entity, Column3: []string{t.Key}, ViewScope: t.Key, AllData: true})
		if err != nil {
			return 0, err
		}
		existing, err := s.document(ctx, q, p.TenantID, row, []string{t.Key})
		if err != nil {
			return 0, err
		}
		if err = s.validateDocument(ctx, p, t, body.Document, existing); err != nil {
			return 0, err
		}
		if err = s.saveItems(ctx, q, p, t, entity, body.Document.Items); err != nil {
			return 0, err
		}
		body.Document.Definition.Revision = base + 1
		raw, err := json.Marshal(body.Document.Definition)
		if err != nil {
			return 0, err
		}
		if err = q.GovUpdateDictionary(ctx, dbgen.GovUpdateDictionaryParams{TenantID: p.TenantID, ID: entity, ScopeKey: t.Key, Spec: raw, Revision: base + 1}); err != nil {
			return 0, err
		}
	}
	if domain == "override" {
		stored, e := q.GovItemOverrides(ctx, dbgen.GovItemOverridesParams{TenantID: p.TenantID, DictionaryID: entity, Column3: []string{t.Key}})
		if e != nil {
			return 0, e
		}
		body.Overrides = []*DictionaryOverride{}
		for _, r := range stored {
			var o DictionaryOverride
			if e = json.Unmarshal(r.Spec, &o); e != nil {
				return 0, e
			}
			body.Overrides = append(body.Overrides, &o)
		}
		rows, e := q.GovDictionaryItems(ctx, dbgen.GovDictionaryItemsParams{TenantID: p.TenantID, DictionaryID: entity, Column3: []string{t.Key}})
		if e != nil {
			return 0, e
		}
		body.Extensions = []*DictionaryItem{}
		for _, r := range rows {
			var item DictionaryItem
			if e = json.Unmarshal(r.Spec, &item); e != nil {
				return 0, e
			}
			item.ID = r.ID
			item.Owner = &ConfigTarget{Kind: t.Kind, ID: t.ID}
			body.Extensions = append(body.Extensions, &item)
		}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return 0, err
	}
	detail, err := json.Marshal(map[string]int{"items": len(body.Document.Items), "overrides": len(body.Overrides)})
	if err != nil {
		return 0, err
	}
	return s.CommitVersion(ctx, q, p, t, domain, key, entity, base, raw, reason, detail)
}
func (s *Service) Publish(ctx context.Context, payload *PublishPayload) (res *ChangeRecord, err error) {
	defer apiError(&err)
	row, err := s.Core.Draft(ctx, "dictionaries:publish", payload.ID)
	if err != nil {
		return nil, err
	}
	if row.Status != "draft" || row.BaseRevision != payload.BaseRevision || row.Domain == "settings" {
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
	var body draftBody
	if err = json.Unmarshal(row.Payload, &body); err != nil {
		return nil, err
	}
	if row.Domain == "override" {
		live, e := s.Q.GovItemOverrides(ctx, dbgen.GovItemOverridesParams{TenantID: p.TenantID, DictionaryID: row.EntityID, Column3: []string{t.Key}})
		if e != nil {
			return nil, e
		}
		seen := map[int64]bool{}
		for _, o := range body.Overrides {
			seen[o.ItemID] = true
		}
		for _, o := range live {
			if !seen[o.ItemID] {
				body.Overrides = append(body.Overrides, &DictionaryOverride{ItemID: o.ItemID, Reset: true})
			}
		}
		items, e := s.Q.GovDictionaryItems(ctx, dbgen.GovDictionaryItemsParams{TenantID: p.TenantID, DictionaryID: row.EntityID, Column3: []string{t.Key}})
		if e != nil {
			return nil, e
		}
		codes := map[string]bool{}
		for _, x := range body.Extensions {
			codes[x.Code] = true
		}
		for _, r := range items {
			if !codes[r.Code] {
				var x DictionaryItem
				if e = json.Unmarshal(r.Spec, &x); e != nil {
					return nil, e
				}
				x.ID = r.ID
				x.Status = "archived"
				x.IsDefault = false
				body.Extensions = append(body.Extensions, &x)
			}
		}
	}
	var id int64
	err = s.Transaction(ctx, func(q *dbgen.Queries) error {
		if e := s.Lock(ctx, q, p, t, row.ResourceKey, payload.BaseRevision); e != nil {
			return e
		}
		if _, e := q.GovPublishDraft(ctx, dbgen.GovPublishDraftParams{TenantID: p.TenantID, ID: payload.ID}); e != nil {
			return e
		}
		var e error
		id, e = s.apply(ctx, q, p, t, row.Domain, row.EntityID, payload.BaseRevision, &body, row.Reason)
		return e
	})
	if err != nil {
		return nil, err
	}
	v, err := s.Version(ctx, "dictionaries:publish", id)
	if err != nil {
		return nil, err
	}
	return s.recordVersion(ctx, v)
}
func (s *Service) Rollback(ctx context.Context, payload *RollbackPayload) (res *ChangeRecord, err error) {
	defer apiError(&err)
	row, err := s.Version(ctx, "dictionaries:rollback", payload.ID)
	if err != nil {
		return nil, err
	}
	if row.Domain == "settings" {
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
	var body draftBody
	if err = json.Unmarshal(row.Payload, &body); err != nil {
		return nil, err
	}
	if row.Domain == "dictionary" {
		live, err := s.Q.GovDictionary(ctx, dbgen.GovDictionaryParams{TenantID: p.TenantID, ID: row.EntityID, Column3: []string{t.Key}, ViewScope: t.Key, AllData: true})
		if err != nil {
			return nil, err
		}
		current, err := s.document(ctx, s.Q, p.TenantID, live, []string{t.Key})
		if err != nil {
			return nil, err
		}
		present := map[string]bool{}
		for _, x := range body.Document.Items {
			present[x.Code] = true
		}
		for _, x := range current.Items {
			if !present[x.Code] {
				copy := *x
				copy.Status = "archived"
				copy.IsDefault = false
				body.Document.Items = append(body.Document.Items, &copy)
			}
		}
	}
	var id int64
	err = s.Transaction(ctx, func(q *dbgen.Queries) error {
		if e := s.Lock(ctx, q, p, t, row.ResourceKey, payload.BaseRevision); e != nil {
			return e
		}
		if e := s.Consume(ctx, q, p, payload.Proof, "dictionaries.rollback", payload.ID, payload.BaseRevision); e != nil {
			return e
		}
		var e error
		id, e = s.apply(ctx, q, p, t, row.Domain, row.EntityID, payload.BaseRevision, &body, payload.Reason)
		return e
	})
	if err != nil {
		return nil, err
	}
	v, err := s.Version(ctx, "dictionaries:rollback", id)
	if err != nil {
		return nil, err
	}
	return s.recordVersion(ctx, v)
}
func (s *Service) Remove(ctx context.Context, payload *RemovePayload) (err error) {
	defer apiError(&err)
	p, all, keys, err := s.AllowedScopes(ctx, "dictionaries:delete", true)
	if err != nil {
		return err
	}
	row, err := s.Q.GovOwnedDictionary(ctx, dbgen.GovOwnedDictionaryParams{TenantID: p.TenantID, ID: payload.ID, Column3: all, Column4: keys})
	if err != nil {
		return err
	}
	if row.Revision != 0 {
		return governance.Problem("conflict")
	}
	_, err = s.Q.GovDeleteDictionary(ctx, dbgen.GovDeleteDictionaryParams{TenantID: p.TenantID, ID: row.ID, ScopeKey: row.ScopeKey})
	return err
}
func applyOverride(item *DictionaryItem, o *DictionaryOverride) {
	for _, field := range o.Fields {
		switch field {
		case "labelEn":
			item.LabelEn = o.LabelEn
		case "labelZh":
			item.LabelZh = o.LabelZh
		case "sort":
			item.Sort = o.Sort
		case "status":
			item.Status = o.Status
		case "isDefault":
			item.IsDefault = o.IsDefault
		}
	}
}
func available(item *DictionaryItem, now time.Time) bool {
	if item.Status != "active" {
		return false
	}
	if item.StartsAt != "" {
		t, e := time.Parse(time.RFC3339, item.StartsAt)
		if e != nil || now.Before(t) {
			return false
		}
	}
	if item.EndsAt != "" {
		t, e := time.Parse(time.RFC3339, item.EndsAt)
		if e != nil || !now.Before(t) {
			return false
		}
	}
	return true
}
func (s *Service) Options(ctx context.Context, payload *OptionsPayload) (res []*DictionaryOption, err error) {
	defer apiError(&err)
	current, err := identity.Current(ctx)
	if err != nil {
		return nil, err
	}
	p, _, err := s.Check(ctx, "dictionaries:use", "department", current.Account.DepartmentID, false)
	if err != nil {
		return nil, err
	}
	chain, err := s.ContextChain(ctx, p)
	if err != nil {
		return nil, err
	}
	all := false
	keys := []string{}
	if p.Can("dictionaries:read") {
		_, all, keys, err = s.AllowedScopes(ctx, "dictionaries:read", true)
		if err != nil {
			return nil, err
		}
	}
	row, err := s.Q.GovRuntimeDictionary(ctx, dbgen.GovRuntimeDictionaryParams{TenantID: p.TenantID, Code: payload.Code, VisibleScopes: chain, AllData: all, ManagementScopes: keys})
	if err != nil {
		return nil, err
	}
	doc, err := s.document(ctx, s.Q, p.TenantID, dbgen.GovDictionaryRow(row), chain)
	if err != nil {
		return nil, err
	}
	res = []*DictionaryOption{}
	if doc.Definition.ConsumerPermission != "" && !p.Can(doc.Definition.ConsumerPermission) {
		return nil, governance.Problem("forbidden")
	}
	if doc.Definition.Status != "active" && len(payload.Ids) == 0 {
		return res, nil
	}
	if doc.Definition.Kind == "dynamic" {
		options, e := s.Identity.ReferenceOptions(ctx, doc.Definition.Provider, payload.Search, payload.Ids...)
		if e != nil {
			return nil, e
		}
		for _, o := range options {
			if len(payload.Ids) > 0 && !governance.HasInt(payload.Ids, o.ID) {
				continue
			}
			res = append(res, &DictionaryOption{ID: o.ID, Code: o.Code, Label: o.Label, Source: doc.Definition.Provider, ParentCode: o.ParentCode})
		}
		return res, nil
	}
	byID := map[int64]*DictionaryItem{}
	byCode := map[string]*DictionaryItem{}
	for _, i := range doc.Items {
		byID[i.ID] = i
		byCode[i.Code] = i
	}
	now := time.Now()
	defaults := 0
	for _, item := range doc.Items {
		disabled := !available(item, now) || doc.Definition.Status != "active"
		if item.Owner != nil && !doc.Definition.AllowExtensions && (item.Owner.Kind != doc.Definition.Target.Kind || item.Owner.ID != doc.Definition.Target.ID) {
			disabled = true
		}
		seen := map[string]bool{}
		node := item
		for node.ParentCode != "" {
			parent := byCode[node.ParentCode]
			if parent == nil || seen[node.Code] || !available(parent, now) {
				disabled = true
				break
			}
			seen[node.Code] = true
			node = parent
		}
		historical := len(payload.Ids) > 0
		if historical && !governance.HasInt(payload.Ids, item.ID) || !historical && disabled {
			continue
		}
		label := item.LabelEn
		if payload.Language == "zh-CN" && item.LabelZh != "" {
			label = item.LabelZh
		}
		if payload.Search != "" && !strings.Contains(strings.ToLower(label+" "+item.Code), strings.ToLower(payload.Search)) {
			continue
		}
		isDefault := item.IsDefault && !disabled
		if isDefault {
			defaults++
		}
		res = append(res, &DictionaryOption{ID: item.ID, Code: item.Code, Label: label, Source: "dictionary", ParentCode: item.ParentCode, Disabled: disabled, IsDefault: isDefault, Color: item.Color, Icon: item.Icon})
	}
	if defaults > 1 {
		for _, o := range res {
			o.IsDefault = false
		}
	}
	sort.SliceStable(res, func(i, j int) bool { return byID[res[i].ID].Sort < byID[res[j].ID].Sort })
	return res, nil
}

// ReplaceReferences is called inside the consumer's already-authorized transaction.
// resourceType/resourceID and previousIDs must come from its scoped repository row,
// never directly from a request. This adapter enforces selectable new references and
// permits unchanged historical references after a code has been disabled.
func (s *Service) ReplaceReferences(ctx context.Context, q *dbgen.Queries, code, resourceType string, resourceID int64, itemIDs, previousIDs []int64) error {
	bound := &Service{Core: s.Core.WithReadQueries(q)}
	p, err := identity.Current(ctx)
	if err != nil {
		return err
	}
	if resourceType == "" || len(resourceType) > 80 || resourceID <= 0 || len(itemIDs) > 200 {
		return governance.Problem("invalid")
	}
	options, err := bound.Options(ctx, &OptionsPayload{Code: code, Language: "en", Ids: itemIDs})
	if err != nil {
		return err
	}
	allowed := map[int64]bool{}
	for _, o := range options {
		if o.Source != "dictionary" {
			return governance.Problem("invalid")
		}
		allowed[o.ID] = !o.Disabled || governance.HasInt(previousIDs, o.ID)
	}
	for _, id := range itemIDs {
		if !allowed[id] {
			return governance.Problem("invalid")
		}
	}
	chain, err := bound.ContextChain(ctx, p)
	if err != nil {
		return err
	}
	all := false
	keys := []string{}
	if p.Can("dictionaries:read") {
		_, all, keys, err = bound.AllowedScopes(ctx, "dictionaries:read", true)
		if err != nil {
			return err
		}
	}
	row, err := q.GovRuntimeDictionary(ctx, dbgen.GovRuntimeDictionaryParams{TenantID: p.TenantID, Code: code, VisibleScopes: chain, AllData: all, ManagementScopes: keys})
	if err != nil {
		return err
	}
	if err = q.GovRemoveDictionaryReferences(ctx, dbgen.GovRemoveDictionaryReferencesParams{TenantID: p.TenantID, ResourceType: resourceType, ResourceID: strconv.FormatInt(resourceID, 10), DictionaryID: row.ID}); err != nil {
		return err
	}
	for _, id := range itemIDs {
		if err = q.GovRegisterDictionaryReference(ctx, dbgen.GovRegisterDictionaryReferenceParams{TenantID: p.TenantID, DictionaryID: row.ID, ItemID: id, ResourceType: resourceType, ResourceID: strconv.FormatInt(resourceID, 10)}); err != nil {
			return err
		}
	}
	return nil
}
