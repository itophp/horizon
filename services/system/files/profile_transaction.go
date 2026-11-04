package files

import (
	"context"
	"encoding/json"

	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/system/governance"
	"baseline/services/api/system/identity"

	"github.com/jackc/pgx/v5"
)

// withTx binds storage-profile and settings reads to the caller's transaction.
// The registered driver registry remains shared with the original service.
func (s *Service) withTx(tx pgx.Tx) *Service {
	core := *s.Core
	core.Q = dbgen.New(tx)
	return &Service{Core: &core, Storage: s.Storage}
}

// PrepareProfileInTx fixes the current storage configuration in an immutable
// profile within the same transaction that creates an upload session.
func (s *Service) PrepareProfileInTx(ctx context.Context, tx pgx.Tx, p *identity.Principal) (Profile, int64, error) {
	if tx == nil {
		return Profile{}, 0, governance.Problem("unavailable")
	}
	return s.withTx(tx).prepareProfile(ctx, p)
}

// ProfileInTx resolves the immutable profile chosen when the upload began.
func (s *Service) ProfileInTx(ctx context.Context, tx pgx.Tx, tenant string, id int64) (Profile, error) {
	if tx == nil {
		return Profile{}, governance.Problem("unavailable")
	}
	return s.withTx(tx).profile(ctx, tenant, id)
}

// Profile resolves a stored profile after the caller has authorized the
// associated business object. It does not itself grant file access.
func (s *Service) Profile(ctx context.Context, tenant string, id int64) (Profile, error) {
	return s.profile(ctx, tenant, id)
}

// SettingInTx reads a registered shared setting using the active transaction.
func (s *Service) SettingInTx(ctx context.Context, tx pgx.Tx, p *identity.Principal, key string) (string, error) {
	if tx == nil {
		return "", governance.Problem("unavailable")
	}
	return s.withTx(tx).Setting(ctx, p, key)
}

func (s *Service) prepareProfile(ctx context.Context, p *identity.Principal) (Profile, int64, error) {
	profile, err := s.currentProfile(ctx, p)
	if err != nil {
		return Profile{}, 0, err
	}
	driver, err := s.Storage.Get(profile.Driver)
	if err != nil {
		return Profile{}, 0, err
	}
	if err = driver.Validate(ctx, profile); err != nil {
		return Profile{}, 0, err
	}
	configRaw, err := json.Marshal(profile)
	if err != nil {
		return Profile{}, 0, err
	}
	stored := profile
	stored.Parameters = map[string]string{}
	for k, v := range profile.Parameters {
		stored.Parameters[k] = v
	}
	for _, field := range driver.Descriptor().Fields {
		if field.Secret {
			stored.Parameters[field.Key], err = s.Seal(p.TenantID, "storage-profile", field.Key, stored.Parameters[field.Key])
			if err != nil {
				return Profile{}, 0, err
			}
		}
	}
	encoded, err := json.Marshal(stored)
	if err != nil {
		return Profile{}, 0, err
	}
	version, err := s.Q.GovEnsureStorageProfile(ctx, dbgen.GovEnsureStorageProfileParams{
		TenantID: p.TenantID, Driver: profile.Driver, ConfigHash: governance.Hash(string(configRaw)), Config: encoded})
	if err != nil {
		return Profile{}, 0, err
	}
	return profile, version, nil
}
