package repository

import (
	"baseline/services/api/gen/demo"
	"baseline/services/api/internal/dataaccess"
	"baseline/services/api/internal/repository/dbgen"
	"context"
	"errors"
)

type Records struct{ Queries *dbgen.Queries }

func (r Records) List(ctx context.Context, s dataaccess.Scope) ([]*demo.TechnicalRecord, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	rows, err := r.Queries.ListRecords(ctx, dbgen.ListRecordsParams{TenantID: s.TenantID, ScopeKind: string(s.Kind), UserID: s.UserID, OrganizationIds: s.OrganizationIDs, DepartmentIds: s.DepartmentIDs, ResourceIds: s.ResourceIDs})
	if err != nil {
		return nil, err
	}
	out := make([]*demo.TechnicalRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, &demo.TechnicalRecord{ID: row.ID, Label: row.Label})
	}
	return out, nil
}
func (r Records) Get(ctx context.Context, s dataaccess.Scope, id int64) (*demo.TechnicalRecord, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	row, err := r.Queries.GetRecord(ctx, dbgen.GetRecordParams{ID: id, TenantID: s.TenantID, ScopeKind: string(s.Kind), UserID: s.UserID, OrganizationIds: s.OrganizationIDs, DepartmentIds: s.DepartmentIDs, ResourceIds: s.ResourceIDs})
	if err != nil {
		return nil, err
	}
	return &demo.TechnicalRecord{ID: row.ID, Label: row.Label}, nil
}
func (r Records) Update(ctx context.Context, s dataaccess.Scope, id int64, label string) (*demo.TechnicalRecord, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	if label == "" || len([]rune(label)) > 80 {
		return nil, errors.New("invalid label")
	}
	row, err := r.Queries.UpdateRecord(ctx, dbgen.UpdateRecordParams{ID: id, Label: label, TenantID: s.TenantID, ScopeKind: string(s.Kind), UserID: s.UserID, OrganizationIds: s.OrganizationIDs, DepartmentIds: s.DepartmentIDs, ResourceIds: s.ResourceIDs})
	if err != nil {
		return nil, err
	}
	return &demo.TechnicalRecord{ID: row.ID, Label: row.Label}, nil
}
func (r Records) Delete(ctx context.Context, s dataaccess.Scope, id int64) error {
	if err := s.Validate(); err != nil {
		return err
	}
	_, err := r.Queries.DeleteRecord(ctx, dbgen.DeleteRecordParams{ID: id, TenantID: s.TenantID, ScopeKind: string(s.Kind), UserID: s.UserID, OrganizationIds: s.OrganizationIDs, DepartmentIds: s.DepartmentIDs, ResourceIds: s.ResourceIDs})
	return err
}
