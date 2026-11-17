package applicationruntime

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/internal/repository/dbgen"
)

// Earlier builds shipped these two definitions under the same version as
// their current counterparts. Match the exact pair of digests so a future
// definition change still requires a new version.
func previousDeploymentDigest(digest string) string {
	switch digest {
	case "0091061563d4940ca1c78d71e5fb58b19c2ce6f1beb1c6818f6d972febe3e8fd": // system-settings 1.0.1
		return "d0b31b4d86f4c8437bfd0d62dd5691cfc75ed85eff083c74e9100cfc88575464"
	case "1612864ce2c65fdb2e94553b2bdaa1a102ddf639bcb9c9aa6e366ddf435792f8": // projects 2.0.3
		return "185b240edeae5378fce2fc0a42258354d4dd18772fb65989ad2c42adcdd7dc99"
	default:
		return ""
	}
}

func deploymentDigest(source ar.ApplicationV2) string {
	return contracts.Digest(contracts.Input{"definition": source})
}

func (m *Module) trustsDeploymentSnapshot(digest string) bool {
	for _, source := range m.contributions() {
		current := deploymentDigest(source)
		if digest == current || (digest != "" && digest == previousDeploymentDigest(current)) {
			return true
		}
	}
	return false
}

func lookupDeploymentSnapshot(ctx context.Context, q *dbgen.Queries, digest string) (dbgen.RuntimeDefinitionSnapshotRow, error) {
	snapshot, err := q.RuntimeDefinitionSnapshot(ctx, digest)
	if previous := previousDeploymentDigest(digest); errors.Is(err, pgx.ErrNoRows) && previous != "" {
		return q.RuntimeDefinitionSnapshot(ctx, previous)
	}
	return snapshot, err
}

func (m *Module) registerDeploymentSnapshot(ctx context.Context, q *dbgen.Queries, source ar.ApplicationV2) (ar.ApplicationV2, string, error) {
	digest := deploymentDigest(source)
	if previous := previousDeploymentDigest(digest); previous != "" {
		snapshot, err := q.RuntimeDefinitionSnapshot(ctx, previous)
		if err == nil {
			saved, err := decodeDefinition(snapshot.Definition)
			if err == nil && (saved.ID != source.ID || saved.Source != source.Source ||
				saved.PluginVersion != source.PluginVersion || saved.Version != source.Version || deploymentDigest(saved) != previous) {
				err = contracts.Problem("conflict")
			}
			return saved, previous, err
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return source, "", err
		}
	}
	// An installed deployment definition is immutable: reuse the stored snapshot
	// when this application version was already registered, even if a code change
	// altered its definition without a version bump. Compatibility with the
	// current catalog is reported on the application head separately.
	existing, existingErr := q.RuntimeSnapshotByIdentity(ctx, dbgen.RuntimeSnapshotByIdentityParams{
		PluginID: source.Source, PluginVersion: source.PluginVersion, ApplicationID: source.ID,
	})
	if existingErr == nil {
		saved, decodeErr := decodeDefinition(existing.Definition)
		if decodeErr != nil || saved.ID != source.ID || saved.Source != source.Source ||
			saved.PluginVersion != source.PluginVersion || saved.Version != source.Version {
			return source, "", contracts.Problem("conflict")
		}
		return saved, existing.Digest, nil
	}
	if !errors.Is(existingErr, pgx.ErrNoRows) {
		return source, "", existingErr
	}
	raw, err := json.Marshal(source)
	if err != nil {
		return source, "", err
	}
	_, err = q.RuntimeRegisterSnapshot(ctx, dbgen.RuntimeRegisterSnapshotParams{
		Digest: digest, PluginID: source.Source, PluginVersion: source.PluginVersion, ApplicationID: source.ID,
		BaseVersion: source.Version, Definition: raw, ContractsDigest: contracts.Digest(contracts.Input{"definitions": m.Definitions}),
	})
	return source, digest, err

}
