package governance

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"horizon/foundation/system/identity"
	"horizon/internal/repository/dbgen"
)

type AIConfiguration struct {
	Revision  int64
	Overrides map[string]json.RawMessage
}

func LoadAIConfiguration(ctx context.Context, q *dbgen.Queries, tenant string) (AIConfiguration, error) {
	snapshot := AIConfiguration{Overrides: map[string]json.RawMessage{}}
	if q == nil {
		return snapshot, Problem("unavailable")
	}
	row, err := q.AIConfiguration(ctx, tenant)
	if errors.Is(err, pgx.ErrNoRows) {
		return snapshot, nil
	}
	if err != nil {
		return snapshot, err
	}
	snapshot.Revision = row.Revision
	err = json.Unmarshal(row.ValuesJson, &snapshot.Overrides)
	return snapshot, err
}

func (s AIConfiguration) Values() map[string]Value {
	values := map[string]Value{}
	for _, definition := range Registry {
		if IsAISetting(definition.Key) {
			values[definition.Key] = Value{Raw: definition.Default, Source: "default"}
		}
	}
	for key, raw := range s.Overrides {
		if IsAISetting(key) && FindDefinition(key) != nil {
			values[key] = Value{Raw: string(raw), Source: "workspace:0", Configured: string(raw) != `""`}
		}
	}
	return values
}

// AISetting is a server-side consumer of the committed AI snapshot.
func (c *Core) AISetting(ctx context.Context, p *identity.Principal, key string) (string, error) {
	if !IsAISetting(key) || FindDefinition(key) == nil {
		return "", Problem("invalid")
	}
	state, err := LoadAIConfiguration(ctx, c.Q, p.TenantID)
	if err != nil {
		return "", err
	}
	return c.settingValue(p.TenantID, key, state.Values()[key])
}
