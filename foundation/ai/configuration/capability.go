package configuration

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strconv"

	"github.com/jackc/pgx/v5"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
)

const ReadPermission = "aiConfig:read"
const UpdatePermission = "aiConfig:update"

type change struct {
	Key       string `json:"key"`
	Operation string `json:"operation"`
	Value     string `json:"value"`
}

type saveInput struct {
	BaseRevision int64    `json:"baseRevision"`
	Changes      []change `json:"changes"`
}

func Entries(core *governance.Core) []registry.Entry {
	revision := contracts.Int(0, 1<<40)
	entry := contracts.Object(map[string]*contracts.Schema{
		"key": {Type: "string"}, "value": {Type: "string"}, "configured": {Type: "boolean"}, "overridden": {Type: "boolean"},
	}, "key", "value", "configured", "overridden")
	option := contracts.Object(map[string]*contracts.Schema{
		"id": {Type: "string"}, "label": {Type: "string"},
	}, "id", "label")
	changeSchema := contracts.Object(map[string]*contracts.Schema{
		"key": contracts.Str(1, 100), "operation": contracts.Enum("set", "reset", "clear"),
		"value": contracts.Str(0, 4096),
	}, "key", "operation")
	return []registry.Entry{
		{
			Definition: contracts.Definition{
				Name: "ai.config.read", Version: "1.0.0", Title: "Read AI configuration",
				Description: "Read active AI model and service settings with credentials redacted.",
				Domain:      "aiConfig", Entity: "ai_configuration", Permission: ReadPermission,
				Layer: contracts.LayerAtomic, Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto, ReadOnly: true,
				Executor: "go:ai.config.read", Tags: []string{"manual-only"}, TransientResult: true,
				AllowedOrigins: []contracts.Origin{contracts.OriginHumanUI, contracts.OriginAPI},
				Input:          contracts.Object(nil),
				Output: contracts.Object(map[string]*contracts.Schema{
					"revision": revision, "entries": {Type: "array", Items: entry},
					"embeddingModels": {Type: "array", Items: option},
				}, "revision", "entries"),
			},
			Handler: contracts.Handler{Execute: func(ctx context.Context, _ contracts.Input) (contracts.Output, error) {
				p, scope, err := core.Identity.Authorize(ctx, ReadPermission, true)
				if err != nil || !scope.All {
					return contracts.Output{}, contracts.Problem("forbidden")
				}
				snapshot, err := governance.LoadAIConfiguration(ctx, core.Q, p.TenantID)
				if err != nil {
					return contracts.Output{}, contracts.Problem(governance.Code(err))
				}
				return readable(snapshot), nil
			}},
		},
		{
			Definition: contracts.Definition{
				Name: "ai.config.save", Version: "1.0.0", Title: "Save AI configuration",
				Description: "Apply validated AI model and service settings immediately at the given revision.",
				Domain:      "aiConfig", Entity: "ai_configuration", Permission: UpdatePermission,
				Layer: contracts.LayerAtomic, Risk: contracts.RiskMedium, Approval: contracts.ApprovalAuto,
				Executor: "go:ai.config.save", Tags: []string{"manual-only"},
				AllowedOrigins: []contracts.Origin{contracts.OriginHumanUI, contracts.OriginAPI},
				Idempotency:    contracts.Idempotency{Required: true, Scope: "tenant+account+key"},
				SuccessEvents:  []string{"ai.config.changed"}, Loggable: []string{"baseRevision"},
				Input: contracts.Object(map[string]*contracts.Schema{
					"baseRevision": revision, "changes": contracts.Array(changeSchema, 1, 100),
				}, "baseRevision", "changes"),
				Output: contracts.Object(map[string]*contracts.Schema{"revision": revision}, "revision"),
			},
			Handler: contracts.Handler{ExecuteTx: func(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
				return save(ctx, core, tx, in)
			}},
		},
	}
}

func readable(snapshot governance.AIConfiguration) contracts.Output {
	values := snapshot.Values()
	entries := make([]any, 0, len(values))
	for _, key := range slices.Sorted(maps.Keys(values)) {
		value := values[key]
		raw := value.Raw
		if governance.FindDefinition(key).Secret {
			raw = ""
		}
		entries = append(entries, map[string]any{
			"key": key, "value": raw, "configured": value.Configured, "overridden": value.Source != "default",
		})
	}
	return contracts.Output{Data: map[string]any{"revision": snapshot.Revision, "entries": entries, "embeddingModels": governance.EmbeddingModelOptions(values)}}
}

func save(ctx context.Context, core *governance.Core, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	scope, err := core.Identity.ScopeForInTx(ctx, tx, p, UpdatePermission, true)
	if err != nil || !scope.All {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	var input saveInput
	if err := contracts.Bind(in, &input); err != nil {
		return contracts.Output{}, err
	}
	q := dbgen.New(tx)
	snapshot, err := governance.LoadAIConfiguration(ctx, q, p.TenantID)
	if err != nil {
		return contracts.Output{}, contracts.Problem(governance.Code(err))
	}
	if snapshot.Revision != input.BaseRevision {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	before := snapshot.Values()
	if err := applyChanges(core, p.TenantID, snapshot.Overrides, input.Changes); err != nil {
		return contracts.Output{}, contracts.Problem(governance.Code(err))
	}
	if err := governance.ValidateAIModelChanges(before, snapshot.Values()); err != nil {
		return contracts.Output{}, contracts.Problem(governance.Code(err))
	}
	raw, err := json.Marshal(snapshot.Overrides)
	if err != nil {
		return contracts.Output{}, contracts.Problem("internal")
	}
	revision, err := q.AIConfigurationSave(ctx, dbgen.AIConfigurationSaveParams{
		TenantID: p.TenantID, BaseRevision: input.BaseRevision, ValuesJson: raw,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	if err != nil {
		return contracts.Output{}, contracts.Problem(governance.Code(err))
	}
	data := map[string]any{"revision": revision}
	return contracts.Output{Data: data, Events: []contracts.Event{{
		Type: "ai.config.changed", AggregateType: "ai_configuration", AggregateID: p.TenantID, Payload: data,
	}}}, nil
}

func applyChanges(core *governance.Core, tenant string, overrides map[string]json.RawMessage, changes []change) error {
	seen := map[string]bool{}
	for _, change := range changes {
		definition := governance.FindDefinition(change.Key)
		if seen[change.Key] || !governance.IsAISetting(change.Key) || definition == nil {
			return governance.Problem("invalid")
		}
		seen[change.Key] = true
		if change.Operation == "reset" {
			delete(overrides, change.Key)
			continue
		}
		raw := change.Value
		if change.Operation == "clear" {
			raw = `""`
		}
		if err := definition.Validate(raw); err != nil {
			return err
		}
		if definition.Secret && raw != `""` {
			sealed, err := core.Seal(tenant, "workspace:0", change.Key, raw)
			if err != nil {
				return err
			}
			raw = strconv.Quote(sealed)
		}
		overrides[change.Key] = json.RawMessage(raw)
	}
	return nil
}
