package applicationruntime

import (
	"baseline/services/api/internal/repository/dbgen"
	ar "baseline/services/api/kernel/applicationruntime"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/gateway"
	"baseline/services/api/system/identity"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
)

func (m *Module) lifecycleDefinitions(ctx context.Context, q *dbgen.Queries, p *identity.Principal, action string, candidate ar.Candidate, in contracts.Input) error {
	for _, id := range candidate.Applications {
		if action == "application.enable" || action == "application.disable" {
			if in["applicationId"] != id {
				continue
			}
		}
		var source ar.ApplicationV2
		for _, app := range m.contributions() {
			if app.ID == id && app.Source == candidate.PluginID && app.PluginVersion == candidate.Version {
				source = app
				break
			}
		}
		head, err := q.RuntimeDefinitionHeadLock(ctx, dbgen.RuntimeDefinitionHeadLockParams{TenantID: p.TenantID, ApplicationID: id})
		if err == pgx.ErrNoRows {
			if source.ID == "" {
				return contracts.Problem("unavailable")
			}
			raw, e := json.Marshal(source)
			if e != nil {
				return e
			}
			digest := contracts.Digest(contracts.Input{"definition": source})
			if _, e = q.RuntimeRegisterSnapshot(ctx, dbgen.RuntimeRegisterSnapshotParams{Digest: digest, PluginID: source.Source, PluginVersion: source.PluginVersion, ApplicationID: id, BaseVersion: source.Version, Definition: raw, ContractsDigest: contracts.Digest(contracts.Input{"definitions": m.Definitions})}); e != nil {
				return e
			}
			if _, e = q.RuntimeInitializeHead(ctx, dbgen.RuntimeInitializeHeadParams{TenantID: p.TenantID, ApplicationID: id, PluginID: source.Source, BaseDigest: digest, RouteRoot: source.Route}); e != nil {
				return e
			}
			source.ApplicationVersion = source.Version + ".0"
			raw, e = json.Marshal(source)
			if e != nil {
				return e
			}
			if e = q.RuntimeDefinitionInsert(ctx, dbgen.RuntimeDefinitionInsertParams{TenantID: p.TenantID, ApplicationID: id, Version: source.ApplicationVersion, BaseDigest: digest, Definition: raw, Digest: contracts.Digest(contracts.Input{"definition": source}), Source: "deployment", CreatedBy: p.Account.ID}); e != nil {
				return e
			}
			if _, e = q.RuntimeDefinitionInitializePublished(ctx, dbgen.RuntimeDefinitionInitializePublishedParams{TenantID: p.TenantID, ApplicationID: id, PublishedVersion: source.ApplicationVersion}); e != nil {
				return e
			}
			head, e = q.RuntimeDefinitionHeadLock(ctx, dbgen.RuntimeDefinitionHeadLockParams{TenantID: p.TenantID, ApplicationID: id})
			if e != nil {
				return e
			}
		} else if err != nil {
			return err
		}
		version := head.PublishedVersion
		if (action == "upgrade" || action == "rollback") && version != "" {
			current, e := q.RuntimeDefinitionGet(ctx, dbgen.RuntimeDefinitionGetParams{TenantID: p.TenantID, ApplicationID: id, Version: version})
			if e != nil {
				return e
			}
			app, e := decodeDefinition(current.Definition)
			if e != nil {
				return e
			}
			if app.PluginVersion != candidate.Version {
				selected := ""
				selectedDigest := ""
				var revision int64
				if reviews, ok := in["definitions"].([]any); ok {
					for _, raw := range reviews {
						review, ok := raw.(map[string]any)
						if !ok {
							return contracts.Problem("invalid")
						}
						if review["applicationId"] == id {
							selected, _ = review["version"].(string)
							selectedDigest, _ = review["digest"].(string)
							revision = inputInt(review["revision"])
						}
					}
				}
				if selected != "" {
					if selected != head.ProposalVersion || revision != head.Revision {
						return contracts.Problem("conflict")
					}
					reviewed, e := q.RuntimeDefinitionGet(ctx, dbgen.RuntimeDefinitionGetParams{TenantID: p.TenantID, ApplicationID: id, Version: selected})
					if e != nil {
						return e
					}
					replacement, e := decodeDefinition(reviewed.Definition)
					if e != nil {
						return e
					}
					if reviewed.Digest != selectedDigest || replacement.PluginVersion != candidate.Version {
						return contracts.Problem("conflict")
					}
					if _, e = m.validateCandidate(ctx, q, p, replacement); e != nil {
						return e
					}
					version = selected
				} else {
					if head.ProposalVersion != "" {
						return contracts.Problem("conflict")
					}
					parsed, e := ar.ParseConfigurationVersion(current.Version)
					if e != nil {
						return e
					}
					if parsed[3] != 0 || source.ID == "" {
						return contracts.Problem("conflict")
					}
					version = source.Version + ".0"
					if version == current.Version {
						return contracts.Problem("conflict")
					}
					raw, e := json.Marshal(source)
					if e != nil {
						return e
					}
					baseDigest := contracts.Digest(contracts.Input{"definition": source})
					if _, e = q.RuntimeRegisterSnapshot(ctx, dbgen.RuntimeRegisterSnapshotParams{Digest: baseDigest, PluginID: source.Source, PluginVersion: source.PluginVersion, ApplicationID: id, BaseVersion: source.Version, Definition: raw, ContractsDigest: contracts.Digest(contracts.Input{"definitions": m.Definitions})}); e != nil {
						return e
					}
					source.ApplicationVersion = version
					source, e = m.validateCandidate(ctx, q, p, source)
					if e != nil {
						return e
					}
					raw, e = json.Marshal(source)
					if e != nil {
						return e
					}
					if e = q.RuntimeDefinitionInsert(ctx, dbgen.RuntimeDefinitionInsertParams{TenantID: p.TenantID, ApplicationID: id, Version: version, ParentVersion: current.Version, BaseDigest: baseDigest, Definition: raw, Digest: contracts.Digest(contracts.Input{"definition": source}), Source: "deployment", CreatedBy: p.Account.ID}); e != nil {
						return e
					}
				}
				changed, e := q.RuntimeDefinitionPublish(ctx, dbgen.RuntimeDefinitionPublishParams{TenantID: p.TenantID, ApplicationID: id, Revision: head.Revision, PublishedVersion: version})
				if e != nil {
					return e
				}
				if changed != 1 {
					return contracts.Problem("conflict")
				}
			}
		}
		if action == "enable" || action == "application.enable" {
			if version == "" {
				return contracts.Problem("unavailable")
			}
			row, e := q.RuntimeDefinitionGet(ctx, dbgen.RuntimeDefinitionGetParams{TenantID: p.TenantID, ApplicationID: id, Version: version})
			if e != nil {
				return e
			}
			app, e := decodeDefinition(row.Definition)
			if e != nil {
				return e
			}
			if app.PluginVersion != candidate.Version {
				return contracts.Problem("conflict")
			}
		}
		meta, _ := gateway.CurrentInvocation(ctx)
		digest := ""
		if version != "" {
			written, e := q.RuntimeDefinitionGet(ctx, dbgen.RuntimeDefinitionGetParams{TenantID: p.TenantID, ApplicationID: id, Version: version})
			if e != nil {
				return e
			}
			digest = written.Digest
		}
		if err = q.RuntimeDefinitionChange(ctx, dbgen.RuntimeDefinitionChangeParams{TenantID: p.TenantID, ApplicationID: id, Action: "lifecycle." + action, Version: version, Digest: digest, AccountID: p.Account.ID, CorrelationID: meta.CorrelationID}); err != nil {
			return fmt.Errorf("runtime lifecycle history: %w", err)
		}
	}
	return nil
}
