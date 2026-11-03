package app

import (
	"baseline/services/api/gen/system"
	"context"
	"time"
)

type System struct{}

func (System) Health(context.Context) (*system.HealthResult, error) {
	return &system.HealthResult{Status: "ok"}, nil
}
func (System) Echo(_ context.Context, p *system.EchoPayload) (*system.EchoResult, error) {
	return &system.EchoResult{Name: p.Name, Timestamp: time.Now().UTC().Format(time.RFC3339)}, nil
}
