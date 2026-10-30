package app

import (
	"baseline/services/api/gen/demo"
	"baseline/services/api/internal/auth"
	"baseline/services/api/internal/dataaccess"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"goa.design/goa/v3/security"
	"log/slog"
)

type RecordStore interface {
	List(context.Context, dataaccess.Scope) ([]*demo.TechnicalRecord, error)
	Get(context.Context, dataaccess.Scope, int64) (*demo.TechnicalRecord, error)
	Update(context.Context, dataaccess.Scope, int64, string) (*demo.TechnicalRecord, error)
	Delete(context.Context, dataaccess.Scope, int64) error
}
type Demo struct {
	Store  RecordStore
	Scopes dataaccess.Resolver
}

func (d Demo) scope(ctx context.Context) (dataaccess.Scope, error) {
	p, ok := auth.FromContext(ctx)
	if !ok {
		return dataaccess.Scope{}, demo.MakeInternal(errors.New("missing trusted context"))
	}
	if d.Store == nil {
		return dataaccess.Scope{}, demo.MakeUnavailable(errors.New("unavailable"))
	}
	s, err := d.Scopes.Resolve(p)
	if err != nil {
		return s, demo.MakeInternal(errors.New("scope resolution failed"))
	}
	return s, nil
}
func safeError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return demo.MakeNotFound(errors.New("not_found"))
	}
	slog.Error("repository failure", "error", err)
	return demo.MakeInternal(errors.New("internal"))
}
func (d Demo) List(ctx context.Context, _ *demo.ListPayload) ([]*demo.TechnicalRecord, error) {
	s, err := d.scope(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := d.Store.List(ctx, s)
	return rows, safeError(err)
}
func (d Demo) Get(ctx context.Context, p *demo.GetPayload) (*demo.TechnicalRecord, error) {
	s, err := d.scope(ctx)
	if err != nil {
		return nil, err
	}
	row, err := d.Store.Get(ctx, s, p.ID)
	return row, safeError(err)
}
func (d Demo) Update(ctx context.Context, p *demo.UpdatePayload) (*demo.TechnicalRecord, error) {
	s, err := d.scope(ctx)
	if err != nil {
		return nil, err
	}
	row, err := d.Store.Update(ctx, s, p.ID, p.Label)
	return row, safeError(err)
}
func (d Demo) Delete(ctx context.Context, p *demo.DeletePayload) error {
	s, err := d.scope(ctx)
	if err != nil {
		return err
	}
	return safeError(d.Store.Delete(ctx, s, p.ID))
}

// APIKeyAuth bridges the generated security contract to the identity verified by the transport.
func (d Demo) APIKeyAuth(ctx context.Context, _ string, _ *security.APIKeyScheme) (context.Context, error) {
	if _, ok := auth.FromContext(ctx); !ok {
		return ctx, &demo.AccessError{Code: "unauthenticated"}
	}
	return ctx, nil
}
