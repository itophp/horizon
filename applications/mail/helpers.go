package mail

import (
	"context"
	"fmt"

	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"
)

func principal(ctx context.Context) (*identity.Principal, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return nil, contracts.Problem("unauthenticated")
	}
	return p, nil
}

func requireSystem(ctx context.Context) (*identity.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	meta, ok := gateway.CurrentInvocation(ctx)
	if !ok || meta.Origin != contracts.OriginEvent || p.Account.ID != 0 || !p.Can("mail:sync") {
		return nil, contracts.Problem("forbidden")
	}
	return p, nil
}

func mailEvent(eventType string, entity string, id int64, payload map[string]any) contracts.Event {
	if payload == nil {
		payload = map[string]any{}
	}
	return contracts.Event{Type: eventType, AggregateType: entity, AggregateID: fmt.Sprintf("%d", id), Payload: payload}
}

func timestamp(value interface{}) string {
	if typed, ok := value.(interface{ Time() string }); ok {
		return typed.Time()
	}
	return ""
}

func problem(err error) error {
	return governance.Problem(governance.Code(err))
}
