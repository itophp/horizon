package dictionaries

import (
	"baseline/services/api/internal/governance"
	"baseline/services/api/internal/identity"
	"baseline/services/api/internal/repository/dbgen"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

var columns = []string{"code", "labelEn", "labelZh", "descriptionEn", "descriptionZh", "parentCode", "sort", "status", "isDefault", "startsAt", "endsAt", "color", "icon"}

func (s *Service) owned(ctx context.Context, permission string, id int64) (*identity.Principal, governance.Target, *DictionaryDocument, error) {
	p, all, keys, err := s.AllowedScopes(ctx, permission, true)
	if err != nil {
		return nil, governance.Target{}, nil, err
	}
	row, err := s.Q.GovOwnedDictionary(ctx, dbgen.GovOwnedDictionaryParams{TenantID: p.TenantID, ID: id, Column3: all, Column4: keys})
	if err != nil {
		return nil, governance.Target{}, nil, err
	}
	t, err := s.Resolve(ctx, p.TenantID, strings.SplitN(row.ScopeKey, ":", 2)[0], func() int64 { v, _ := strconv.ParseInt(strings.SplitN(row.ScopeKey, ":", 2)[1], 10, 64); return v }())
	if err != nil {
		return nil, t, nil, err
	}
	doc, err := s.document(ctx, s.Q, p.TenantID, dbgen.GovDictionaryRow(row), []string{row.ScopeKey})
	return p, t, doc, err
}
func formulaEscape(v string) string {
	if strings.HasPrefix(v, "'") {
		return "'" + v
	}
	trimmed := strings.TrimLeft(v, " \t\r\n")
	if trimmed != "" && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + v
	}
	if v != "" && strings.ContainsRune("\t\r\n", rune(v[0])) {
		return "'" + v
	}
	return v
}
func formulaUnescape(v string) string {
	if strings.HasPrefix(v, "'") {
		return v[1:]
	}
	return v
}
func parseImport(format, content string) ([]*DictionaryItem, []string) {
	errs := []string{}
	items := []*DictionaryItem{}
	if len(content) > 10<<20 {
		return items, []string{"0:content:too_large"}
	}
	if format == "json" {
		var doc DictionaryDocument
		decoder := json.NewDecoder(strings.NewReader(content))
		if err := decoder.Decode(&doc); err != nil {
			return items, []string{"0:content:json"}
		}
		if err := decoder.Decode(new(interface{})); err != io.EOF {
			return items, []string{"0:content:json"}
		}
		if len(doc.Items) > 10000 {
			return items, []string{"0:items:too_many"}
		}
		return doc.Items, errs
	}
	if format != "csv" {
		return items, []string{"0:format:invalid"}
	}
	reader := csv.NewReader(strings.NewReader(content))
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return items, []string{"1:header:invalid"}
	}
	marked := len(header) == 2 && header[0] == "#baseline-dictionary" && header[1] == "1"
	if marked {
		header, err = reader.Read()
		if err != nil {
			return items, []string{"2:header:invalid"}
		}
	}
	indices := map[string]int{}
	for i, h := range header {
		if _, exists := indices[h]; exists {
			return items, []string{"1:header:duplicate"}
		}
		indices[h] = i
	}
	for _, required := range []string{"code", "labelEn"} {
		if _, ok := indices[required]; !ok {
			return items, []string{"1:" + required + ":required"}
		}
	}
	for n := 0; ; n++ {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		line := n + 2
		if marked {
			line++
		}
		if n >= 10000 {
			return items, append(errs, "0:items:too_many")
		}
		if err != nil || len(row) != len(header) {
			errs = append(errs, fmt.Sprintf("%d:row:columns", line))
			continue
		}
		value := func(key string) string {
			i, ok := indices[key]
			if !ok {
				return ""
			}
			v := row[i]
			if marked {
				v = formulaUnescape(v)
			}
			return v
		}
		sort := 0
		if value("sort") != "" {
			sort, err = strconv.Atoi(value("sort"))
			if err != nil {
				errs = append(errs, fmt.Sprintf("%d:sort:integer", line))
				continue
			}
		}
		defaultValue := false
		if value("isDefault") != "" {
			defaultValue, err = strconv.ParseBool(value("isDefault"))
			if err != nil {
				errs = append(errs, fmt.Sprintf("%d:isDefault:boolean", line))
				continue
			}
		}
		status := value("status")
		if status == "" {
			status = "active"
		}
		items = append(items, &DictionaryItem{Code: value("code"), LabelEn: value("labelEn"), LabelZh: value("labelZh"), DescriptionEn: value("descriptionEn"), DescriptionZh: value("descriptionZh"), ParentCode: value("parentCode"), Sort: sort, Status: status, IsDefault: defaultValue, StartsAt: value("startsAt"), EndsAt: value("endsAt"), Color: value("color"), Icon: value("icon")})
	}
	return items, errs
}
func (s *Service) preview(ctx context.Context, payload *ImportPayload) (*ImportPreview, error) {
	p, t, doc, err := s.owned(ctx, "dictionaries:import", payload.ID)
	if err != nil {
		return nil, err
	}
	if doc.Definition.Kind == "dynamic" {
		return nil, governance.Problem("forbidden")
	}
	items, errs := parseImport(payload.Format, payload.Content)
	out := &ImportPreview{Document: doc, Errors: errs}
	byCode := map[string]*DictionaryItem{}
	for _, x := range doc.Items {
		byCode[x.Code] = x
	}
	seen := map[string]bool{}
	for i, item := range items {
		if item == nil {
			out.Errors = append(out.Errors, fmt.Sprintf("%d:item:invalid", i+1))
			continue
		}
		if seen[item.Code] {
			out.Errors = append(out.Errors, fmt.Sprintf("%d:code:duplicate", i+1))
			continue
		}
		seen[item.Code] = true
		if err := validItem(item); err != nil {
			out.Errors = append(out.Errors, fmt.Sprintf("%d:item:invalid", i+1))
			continue
		}
		item.Owner = doc.Definition.Target
		if old := byCode[item.Code]; old != nil {
			item.ID = old.ID
			for j, x := range doc.Items {
				if x.Code == item.Code {
					doc.Items[j] = item
					break
				}
			}
			out.Updated++
		} else {
			item.ID = 0
			doc.Items = append(doc.Items, item)
			out.Added++
		}
	}
	if len(out.Errors) == 0 {
		if err = s.validateDocument(ctx, p, t, doc, nil); err != nil {
			out.Errors = append(out.Errors, "0:document:invalid")
		}
	}
	return out, nil
}
func (s *Service) Import(ctx context.Context, payload *ImportPayload) (res *ImportPreview, err error) {
	defer apiError(&err)
	return s.preview(ctx, payload)
}
func (s *Service) ImportDraft(ctx context.Context, payload *ImportDraftPayload) (res *ChangeRecord, err error) {
	defer apiError(&err)
	preview, err := s.preview(ctx, &ImportPayload{ID: payload.ID, Format: payload.Format, Content: payload.Content})
	if err != nil {
		return nil, err
	}
	if len(preview.Errors) > 0 {
		return nil, governance.Problem("invalid")
	}
	return s.draft(ctx, &DraftPayload{Document: preview.Document, BaseRevision: payload.BaseRevision, Reason: payload.Reason}, "dictionaries:import")
}
func (s *Service) Export(ctx context.Context, payload *ExportPayload) (res *ExportResult, err error) {
	defer apiError(&err)
	_, _, doc, err := s.owned(ctx, "dictionaries:export", payload.ID)
	if err != nil {
		return nil, err
	}
	if doc.Definition.Kind == "dynamic" {
		return nil, governance.Problem("forbidden")
	}
	res = &ExportResult{Filename: doc.Definition.Code + "." + payload.Format}
	if payload.Format == "json" {
		raw, err := json.MarshalIndent(doc, "", "  ")
		res.Content = string(raw)
		return res, err
	}
	var buffer strings.Builder
	w := csv.NewWriter(&buffer)
	if err = w.Write([]string{"#baseline-dictionary", "1"}); err != nil {
		return nil, err
	}
	if err = w.Write(columns); err != nil {
		return nil, err
	}
	for _, x := range doc.Items {
		row := []string{x.Code, x.LabelEn, x.LabelZh, x.DescriptionEn, x.DescriptionZh, x.ParentCode, strconv.Itoa(x.Sort), x.Status, strconv.FormatBool(x.IsDefault), x.StartsAt, x.EndsAt, x.Color, x.Icon}
		for i, v := range row {
			row[i] = formulaEscape(v)
		}
		if err = w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err = w.Error(); err != nil {
		return nil, err
	}
	res.Content = buffer.String()
	return res, nil
}
